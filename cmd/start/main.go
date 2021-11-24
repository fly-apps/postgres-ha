package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/flypg/admin"
	"github.com/fly-examples/postgres-ha/pkg/flyunlock"
	"github.com/fly-examples/postgres-ha/pkg/supervisor"
	"github.com/jackc/pgx/v4"
)

func main() {

	if os.Getenv("FLY_RESTORED_FROM") != "" {
		if err := manageRestore(); err != nil {
			panic(err)
		}
	}

	node, err := flypg.NewNode()
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(node.DataDir, 0700); err != nil {
		panic(err)
	}

	writeStolonctlEnvFile(node, filepath.Join(node.DataDir, ".env"))

	stolonUser, err := user.Lookup("stolon")
	if err != nil {
		panic(err)
	}
	stolonUID, err := strconv.Atoi(stolonUser.Uid)
	if err != nil {
		panic(err)
	}
	stolonGID, err := strconv.Atoi(stolonUser.Gid)
	if err != nil {
		panic(err)
	}

	cmdStr := fmt.Sprintf("chown -R %d:%d %s", stolonUID, stolonGID, node.DataDir)
	cmd := exec.Command("sh", "-c", cmdStr)
	_, err = cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
	}

	stolonCmd := func(cmd string) string {
		return "gosu stolon " + cmd
	}

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for range t.C {
			fmt.Println("checking stolon status")

			cd, err := node.GetStolonClusterData()
			if err != nil && !errors.Is(err, flypg.ErrClusterNotInitialized) {
				panic(err)
			}

			currentKeeper := cd.Keepers[node.KeeperUID]
			if currentKeeper == nil {
				continue
			}
			currentDB := cd.FindDB(node.KeeperUID)
			if currentDB == nil {
				continue
			}

			if currentKeeper.Status.Healthy && currentDB.Status.Healthy {
				fmt.Println("keeper is healthy, db is healthy, role:", currentDB.Spec.Role)
				if currentDB.Spec.Role == "master" {
					pg, err := node.NewLocalConnection(context.TODO())
					if err != nil {
						fmt.Println("error connecting to local postgres", err)
						continue
					}

					if err = initOperator(context.TODO(), pg, node.OperatorCredentials); err != nil {
						fmt.Println("error configuring operator user:", err)
						continue
					}

					if err = initReplicationUser(context.TODO(), pg, node.ReplCredentials); err != nil {
						fmt.Println("error configuring replication user:", err)
						continue
					}
				}

				return
			}
		}
	}()

	svisor := supervisor.New("flypg", 5*time.Minute)

	keeperEnv := map[string]string{
		"STKEEPER_UID":               node.KeeperUID,
		"STKEEPER_DATA_DIR":          node.DataDir,
		"STKEEPER_PG_SU_USERNAME":    node.SUCredentials.Username,
		"STKEEPER_PG_SU_PASSWORD":    node.SUCredentials.Password,
		"STKEEPER_PG_REPL_USERNAME":  node.ReplCredentials.Username,
		"STKEEPER_PG_REPL_PASSWORD":  node.ReplCredentials.Password,
		"STKEEPER_PG_LISTEN_ADDRESS": node.PrivateIP.String(),
		"STKEEPER_PG_PORT":           strconv.Itoa(node.PGPort),
		"STKEEPER_LOG_LEVEL":         "info",
		"STKEEPER_CLUSTER_NAME":      node.AppName,
		"STKEEPER_STORE_BACKEND":     node.BackendStore,
		"STKEEPER_STORE_URL":         node.BackendStoreURL.String(),
		"STKEEPER_STORE_NODE":        node.StoreNode,
	}

	if !node.IsPrimaryRegion() {
		keeperEnv["STKEEPER_CAN_BE_MASTER"] = "false"
		keeperEnv["STKEEPER_CAN_BE_SYNCHRONOUS_REPLICA"] = "false"
	}

	svisor.AddProcess("keeper", stolonCmd("stolon-keeper"), supervisor.WithEnv(keeperEnv), supervisor.WithRestart(10, 10*time.Second))

	sentinelEnv := map[string]string{
		"STSENTINEL_DATA_DIR":             node.DataDir,
		"STSENTINEL_INITIAL_CLUSTER_SPEC": "/fly/cluster-spec.json",
		"STSENTINEL_LOG_LEVEL":            "info",
		"STSENTINEL_CLUSTER_NAME":         node.AppName,
		"STSENTINEL_STORE_BACKEND":        node.BackendStore,
		"STSENTINEL_STORE_URL":            node.BackendStoreURL.String(),
		"STSENTINEL_STORE_NODE":           node.StoreNode,
	}

	svisor.AddProcess("sentinel", stolonCmd("stolon-sentinel"), supervisor.WithEnv(sentinelEnv), supervisor.WithRestart(0, 3*time.Second))

	//svisor.AddProcess("proxy", "stolon-proxy", supervisor.WithEnv(proxyEnv))
	// proxyEnv := map[string]string{
	// 	"STPROXY_LISTEN_ADDRESS": net.ParseIP("0.0.0.0").String(),
	// 	"STPROXY_PORT":           strconv.Itoa(node.PGProxyPort),
	// 	"STPROXY_LOG_LEVEL":      "info",
	// 	"STPROXY_CLUSTER_NAME":   node.AppName,
	// 	"STPROXY_STORE_BACKEND":  node.BackendStore,
	// 	"STPROXY_STORE_URL":      node.BackendStoreURL.String(),
	// 	"STPROXY_STORE_NODE":     node.StoreNode,
	// }

	proxyEnv := map[string]string{
		"FLY_APP_NAME":   os.Getenv("FLY_APP_NAME"),
		"PRIMARY_REGION": os.Getenv("PRIMARY_REGION"),
	}
	svisor.AddProcess("proxy", "/usr/sbin/haproxy -W -db -f /fly/haproxy.cfg", supervisor.WithEnv(proxyEnv), supervisor.WithRestart(0, 1*time.Second))

	exporterEnv := map[string]string{
		"DATA_SOURCE_URI":                      fmt.Sprintf("[%s]:%d/postgres?sslmode=disable", node.PrivateIP, node.PGPort),
		"DATA_SOURCE_USER":                     node.SUCredentials.Username,
		"DATA_SOURCE_PASS":                     node.SUCredentials.Password,
		"PG_EXPORTER_EXCLUDE_DATABASE":         "template0,template1",
		"PG_EXPORTER_DISABLE_SETTINGS_METRICS": "true",
		"PG_EXPORTER_AUTO_DISCOVER_DATABASES":  "true",
		"PG_EXPORTER_EXTEND_QUERY_PATH":        "/fly/queries.yaml",
	}

	svisor.AddProcess("exporter", "postgres_exporter", supervisor.WithEnv(exporterEnv), supervisor.WithRestart(0, 1*time.Second))

	if err := flypg.InitConfig("/fly/cluster-spec.json"); err != nil {
		panic(err)
	}

	svisor.StopOnSignal(syscall.SIGINT, syscall.SIGTERM)

	svisor.StartHttpListener()
	err = svisor.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func writeStolonctlEnvFile(n *flypg.Node, filename string) {
	var b bytes.Buffer
	b.WriteString("STOLONCTL_CLUSTER_NAME=" + n.AppName + "\n")
	b.WriteString("STOLONCTL_STORE_BACKEND=" + n.BackendStore + "\n")
	b.WriteString("STOLONCTL_STORE_URL=" + n.BackendStoreURL.String() + "\n")
	b.WriteString("STOLONCTL_STORE_NODE=" + n.StoreNode + "\n")

	os.WriteFile(filename, b.Bytes(), 0644)
}

func initOperator(ctx context.Context, pg *pgx.Conn, creds flypg.Credentials) error {
	fmt.Println("configuring operator")

	if creds.Password == "" {
		fmt.Println("OPERATOR_PASSWORD not set, cannot configure operator")
		return nil
	}

	users, err := admin.ListUsers(ctx, pg)
	if err != nil {
		return err
	}

	var operatorUser *admin.UserInfo

	for _, u := range users {
		if u.Username == creds.Username {
			operatorUser = &u
			break
		}
	}

	if operatorUser == nil {
		fmt.Println("operator user does not exist, creating")
		err = admin.CreateUser(ctx, pg, creds.Username, creds.Password)
		if err != nil {
			return err
		}
		operatorUser, err = admin.FindUser(ctx, pg, creds.Username)
		if err != nil {
			return err
		}
	}

	if operatorUser == nil {
		return errors.New("error creating operator: user not found")
	}

	if !operatorUser.SuperUser {
		fmt.Println("operator is not a superuser, fixing")
		if err := admin.GrantSuperuser(ctx, pg, creds.Username); err != nil {
			return err
		}
	}

	if !operatorUser.IsPassword(creds.Password) {
		fmt.Println("operator password does not match config, changing")
		if err := admin.ChangePassword(ctx, pg, creds.Username, creds.Password); err != nil {
			return err
		}
	}

	fmt.Println("operator ready!")

	return nil
}

func initReplicationUser(ctx context.Context, pg *pgx.Conn, creds flypg.Credentials) error {
	fmt.Println("configuring repluser")

	if creds.Password == "" {
		fmt.Println("REPL_PASSWORD not set, cannot configure operator")
		return nil
	}

	users, err := admin.ListUsers(ctx, pg)
	if err != nil {
		return err
	}

	var replUser *admin.UserInfo

	for _, u := range users {
		if u.Username == creds.Username {
			replUser = &u
			break
		}
	}

	if replUser == nil {
		fmt.Println("repl user does not exist, creating")
		err = admin.CreateUser(ctx, pg, creds.Username, creds.Password)
		if err != nil {
			return err
		}
		replUser, err = admin.FindUser(ctx, pg, creds.Username)
		if err != nil {
			return err
		}
	}

	if replUser == nil {
		return errors.New("error creating operator: user not found")
	}

	if !replUser.ReplUser {
		fmt.Println("repluser does not have REPLICATION role, fixing")
		if err := admin.GrantSuperuser(ctx, pg, creds.Username); err != nil {
			return err
		}
	}

	if !replUser.IsPassword(creds.Password) {
		fmt.Println("repluser password does not match config, changing")
		if err := admin.ChangePassword(ctx, pg, creds.Username, creds.Password); err != nil {
			return err
		}
	}

	fmt.Println("replication ready!")

	return nil
}

func manageRestore() error {
	isRestore, err := isActiveRestore()
	if err != nil {
		return err
	}
	if isRestore {
		if err := flyunlock.Run(); err != nil {
			return err
		}
	}

	return nil
}

func isActiveRestore() (bool, error) {
	restoreLockFile := flyunlock.LockFilePath()
	if _, err := os.Stat(restoreLockFile); err == nil {
		input, err := ioutil.ReadFile(restoreLockFile)
		if err != nil {
			return false, err
		}
		if string(input) == os.Getenv("FLY_APP_NAME") {
			return false, nil
		}
	}
	return true, nil
}
