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

	"github.com/fly-examples/postgres-ha/pkg/supervisor"
	"github.com/jackc/pgx/v4"
)

const pathToFile = "/data/postgres/pg_hba.conf"
const pathToBackup = "/data/postgres/pg_hba.conf.bak"
const restoreLockFile = "/data/restore.lock"

func Run() error {
	// Backup pg_hba.conf file.
	if err := backupHBAFile(); err != nil {
		return err
	}

	// Write a new temperary pg_hba.conf file.
	if err := overwriteHBAFile(); err != nil {
		return err
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

	// Start PG process
	svisor := supervisor.New("flypg", 5*time.Minute)
	svisor.AddProcess("pg", "postgres -D /data/postgres -p 5432")

	go svisor.Run()

	time.Sleep(time.Second * 2)

	conn, err := openConn()
	if err != nil {
		return err
	}

	// Change internal user credentials.
	if err = setInternalCredential(conn, "flypgadmin", os.Getenv("SU_PASSWORD")); err != nil {
		return err
	}

	if err = setInternalCredential(conn, "repluser", os.Getenv("REPL_PASSWORD")); err != nil {
		return err
	}

	if err = setInternalCredential(conn, "postgres", os.Getenv("OPERATOR_PASSWORD")); err != nil {
		return err
	}

	// Restore original pg_hba.conf file.
	if err := restoreHBAFile(); err != nil {
		return err
	}

	// Stop PG
	svisor.Stop()

	// Set restore lock
	if err := setRestoreLock(); err != nil {
		return err
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

	perm := []byte("host all flypgadmin 0.0.0.0/0 trust")
	_, err = file.Write(perm)
	if err != nil {
		return err
	}

	return nil
}

func openConn() (*pgx.Conn, error) {
	mode := "any"
	hosts := []string{"localhost"}

	url := fmt.Sprintf("postgres://%s/postgres?target_session_attrs=%s", strings.Join(hosts, ","), mode)
	conf, err := pgx.ParseConfig(url)

	if err != nil {
		return nil, err
	}
	conf.User = "flypgadmin"

	return pgx.ConnectConfig(context.Background(), conf)
}

func setInternalCredential(conn *pgx.Conn, user, password string) error {
	sql := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", user, password)
	_, err := conn.Exec(context.Background(), sql)
	if err != nil {
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
	file, err := os.OpenFile(restoreLockFile, os.O_RDWR|os.O_TRUNC, 0644)
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
