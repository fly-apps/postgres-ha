package flyunlock

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/fly-examples/postgres-ha/pkg/supervisor"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
)

const pathToFile = "/data/postgres/pg_hba.conf"
const pathToBackup = "/data/postgres/pg_hba.conf.bak"
const restoreLockFile = "/data/restore.lock"

func Run() error {
	if err := backupHBAFile(); err != nil {
		if os.IsNotExist(err) {
			// if there's no pg_hba.conf file assume we are a new standby coming online
			return nil
		}
		return errors.Wrap(err, "failed backing up pg_hba.conf")
	}

	if err := overwriteHBAFile(); err != nil {
		return errors.Wrap(err, "failed to overwrite pg_hba.conf")
	}

	stolonUser, err := user.Lookup("stolon")
	if err != nil {
		return err
	}
	stolonUID, err := strconv.Atoi(stolonUser.Uid)
	if err != nil {
		return err
	}
	stolonGID, err := strconv.Atoi(stolonUser.Gid)
	if err != nil {
		return err
	}
	if err := os.Chown("/data/postgres", stolonUID, stolonGID); err != nil {
		return err
	}

	if _, err := os.Stat("/data/postgres/standby.signal"); err == nil {
		fmt.Println("restoring from a hot standby. clearing standby signal so we can boot.")
		// We are restoring from a hot standby, so we need to clear the signal so we can boot.
		if err = os.Remove("/data/postgres/standby.signal"); err != nil {
			return errors.Wrap(err, "failed to remove standby signal")
		}
	}

	ip, err := privnet.PrivateIPv6()
	if err != nil {
		return err
	}
	svisor := supervisor.New("flypg", 5*time.Minute)
	svisor.AddProcess("pg", fmt.Sprintf("postgres -D /data/postgres -p 5432 -h %s", ip.String()))

	go svisor.Run()

	conn, err := openConn()
	if err != nil {
		return errors.Wrap(err, "failed opening connection to postgres")
	}

	if err = setInternalCredential(conn, "flypgadmin", os.Getenv("SU_PASSWORD"), false); err != nil {
		return err
	}

	if err = setInternalCredential(conn, "repluser", os.Getenv("REPL_PASSWORD"), false); err != nil {
		return err
	}

	if err = setInternalCredential(conn, "postgres", os.Getenv("OPERATOR_PASSWORD"), true); err != nil {
		return err
	}

	if err := restoreHBAFile(); err != nil {
		return errors.Wrap(err, "failed to restore original pg_hba.conf")
	}

	svisor.Stop()

	if err := setRestoreLock(); err != nil {
		return errors.Wrap(err, "failed to set restore lock")
	}

	return nil
}

func LockFilePath() string {
	return restoreLockFile
}

func backupHBAFile() error {
	if _, err := os.Stat(pathToFile); os.IsNotExist(err) {
		return err
	}

	input, err := ioutil.ReadFile(pathToFile)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(pathToBackup, input, 0644); err != nil {
		return err
	}

	return nil
}

func overwriteHBAFile() error {
	file, err := os.OpenFile(pathToFile, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	perm := []byte("host all flypgadmin ::0/0 trust")
	_, err = file.Write(perm)
	if err != nil {
		return err
	}

	return nil
}

func openConn() (*pgx.Conn, error) {
	mode := "any"
	ip, err := privnet.PrivateIPv6()
	if err != nil {
		return nil, err
	}

	hosts := []string{ip.String()}
	url := fmt.Sprintf("postgres://[%s]/postgres?target_session_attrs=%s", strings.Join(hosts, ","), mode)
	conf, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	conf.User = "flypgadmin"

	// Allow up to 30 seconds for PG to boot and accept connections.
	timeout := time.After(2 * time.Minute)
	tick := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timed out waiting for successful connection")
		case <-tick:
			conn, err := pgx.ConnectConfig(context.Background(), conf)
			if err == nil {
				return conn, err
			}
		}
	}
}

func setInternalCredential(conn *pgx.Conn, user, password string, optional bool) error {
	sql := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", user, password)
	_, err := conn.Exec(context.Background(), sql)
	if err != nil {
		if optional {
			fmt.Printf("failed to reset credentials for user: %q. error: %v", user, err)
			return nil
		}
		return err
	}
	return nil
}

func restoreHBAFile() error {
	input, err := ioutil.ReadFile(pathToBackup)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(pathToFile, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(input)
	if err != nil {
		return err
	}

	if err := os.Remove(pathToBackup); err != nil {
		return err
	}

	return nil
}

func setRestoreLock() error {
	file, err := os.OpenFile(restoreLockFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(os.Getenv("FLY_APP_NAME"))
	if err != nil {
		return err
	}
	return nil
}
