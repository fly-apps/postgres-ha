package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/supervisor"
)

func main() {
	node, err := flypg.NewNode()
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll("/data", 0700); err != nil {
		panic(err)
	}

	writeStolonctlEnvFile(node, "/data/.env")

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
	if err := os.Chown("/data", stolonUID, stolonGID); err != nil {
		panic(err)
	}

	go func() {
		for range time.Tick(5 * time.Second) {
			fmt.Println("checking stolon status")

			status, err := node.GetStolonStatus()
			if err != nil {
				panic(err)
			}
			fmt.Printf("stolon status: %#v\n", status)
		}
	}()

	svisor := supervisor.New("flypg", 5*time.Minute)

	keeperEnv := map[string]string{
		"STKEEPER_UID":               node.KeeperUID,
		"STKEEPER_DATA_DIR":          "/data/",
		"STKEEPER_PG_SU_USERNAME":    node.SUCredentials.Username,
		"STKEEPER_PG_SU_PASSWORD":    node.SUCredentials.Password,
		"STKEEPER_PG_REPL_USERNAME":  node.ReplCredentials.Username,
		"STKEEPER_PG_REPL_PASSWORD":  node.ReplCredentials.Password,
		"STKEEPER_PG_LISTEN_ADDRESS": node.PrivateIP.String(),
		"STKEEPER_PG_PORT":           strconv.Itoa(node.PGPort),
		"STKEEPER_LOG_LEVEL":         "info",
		"STKEEPER_CLUSTER_NAME":      node.AppName,
		"STKEEPER_STORE_BACKEND":     "consul",
		"STKEEPER_STORE_URL":         node.ConsulURL.String(),
		"STKEEPER_STORE_NODE":        node.StoreNode,
	}

	if primaryRegion := os.Getenv("PRIMARY_REGION"); primaryRegion != "" {
		if primaryRegion != os.Getenv("FLY_REGION") {
			keeperEnv["STKEEPER_CAN_BE_MASTER"] = "false"
			keeperEnv["STKEEPER_CAN_BE_SYNCHRONOUS_REPLICA"] = "false"
		}
	}

	svisor.AddProcess("keeper", "stolon-keeper", supervisor.WithEnv(keeperEnv))

	sentinelEnv := map[string]string{
		"STSENTINEL_DATA_DIR":             "/data/",
		"STSENTINEL_INITIAL_CLUSTER_SPEC": "/fly/cluster-spec.json",
		"STSENTINEL_LOG_LEVEL":            "info",
		"STSENTINEL_CLUSTER_NAME":         node.AppName,
		"STSENTINEL_STORE_BACKEND":        "consul",
		"STSENTINEL_STORE_URL":            node.ConsulURL.String(),
		"STSENTINEL_STORE_NODE":           node.StoreNode,
	}

	svisor.AddProcess("sentinel", "stolon-sentinel", supervisor.WithEnv(sentinelEnv))

	proxyEnv := map[string]string{
		"STPROXY_LISTEN_ADDRESS": node.PrivateIP.String(),
		"STPROXY_PORT":           strconv.Itoa(node.PGProxyPort),
		"STPROXY_LOG_LEVEL":      "info",
		"STPROXY_CLUSTER_NAME":   node.AppName,
		"STPROXY_STORE_BACKEND":  "consul",
		"STPROXY_STORE_URL":      node.ConsulURL.String(),
		"STPROXY_STORE_NODE":     node.StoreNode,
	}

	svisor.AddProcess("proxy", "stolon-proxy", supervisor.WithEnv(proxyEnv))

	exporterEnv := map[string]string{
		"DATA_SOURCE_URI":                      fmt.Sprintf("[%s]:%d/postgres?sslmode=disable", node.PrivateIP, node.PGPort),
		"DATA_SOURCE_USER":                     node.SUCredentials.Username,
		"DATA_SOURCE_PASS":                     node.SUCredentials.Password,
		"PG_EXPORTER_EXCLUDE_DATABASE":         "template0,template1",
		"PG_EXPORTER_DISABLE_SETTINGS_METRICS": "true",
		"PG_EXPORTER_AUTO_DISCOVER_DATABASES":  "true",
		"PG_EXPORTER_EXTEND_QUERY_PATH":        "/fly/queries.yaml",
	}

	svisor.AddProcess("exporter", "postgres_exporter", supervisor.WithEnv(exporterEnv))

	if err := flypg.InitConfig("/fly/cluster-spec.json"); err != nil {
		panic(err)
	}

	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigch
		fmt.Println("Got interrupt, stopping")
		svisor.Stop()
	}()

	svisor.Run()
}

func writeStolonctlEnvFile(n *flypg.Node, filename string) {
	var b bytes.Buffer
	b.WriteString("STOLONCTL_CLUSTER_NAME=" + n.AppName + "\n")
	b.WriteString("STOLONCTL_STORE_BACKEND=consul\n")
	b.WriteString("STOLONCTL_STORE_URL=" + n.ConsulURL.String() + "\n")
	b.WriteString("STOLONCTL_STORE_NODE=" + n.StoreNode + "\n")

	os.WriteFile(filename, b.Bytes(), 0644)
}
