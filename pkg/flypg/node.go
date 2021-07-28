package flypg

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
)

const (
	BackendStoreConsul      = "consul"
	BackendStoreEtcd        = "etcdv3"
	BackendStoreUnspecified = ""
)

type Credentials struct {
	Username string
	Password string
}

type Node struct {
	AppName       string
	PrivateIP     net.IP
	Region        string
	PrimaryRegion string
	DataDir       string

	SUCredentials       Credentials
	ReplCredentials     Credentials
	OperatorCredentials Credentials

	BackendStore    string
	BackendStoreURL *url.URL

	KeeperUID string
	StoreNode string

	PGPort      int
	PGProxyPort int
}

func NewNode() (*Node, error) {
	node := &Node{
		AppName:       "local",
		PGPort:        5433,
		PGProxyPort:   5432,
		Region:        "local",
		PrimaryRegion: "local",
		DataDir:       "/data",
	}

	if region := os.Getenv("FLY_REGION"); region != "" {
		node.Region = region
	}

	if region := os.Getenv("PRIMARY_REGION"); region != "" {
		node.PrimaryRegion = region
	}

	if appName := os.Getenv("FLY_APP_NAME"); appName != "" {
		node.AppName = appName
	}

	// If BACKEND_STORE is unspecified, it will default to consul.
	if node.BackendStore = os.Getenv("BACKEND_STORE"); node.BackendStore == BackendStoreUnspecified {
		node.BackendStore = BackendStoreConsul
	}

	var err error

	node.PrivateIP, err = privnet.PrivateIPv6()
	if err != nil {
		return nil, errors.Wrap(err, "error getting private ip")
	}

	var rawBackendStoreURL string

	switch node.BackendStore {
	case BackendStoreConsul:
		rawBackendStoreURL = os.Getenv("FLY_CONSUL_URL")
		if rawBackendStoreURL == "" {
			rawBackendStoreURL = os.Getenv("CONSUL_URL")
		}
		if rawBackendStoreURL == "" {
			return nil, errors.New("FLY_CONSUL_URL or CONSUL_URL are required")
		}
	case BackendStoreEtcd:
		rawBackendStoreURL = os.Getenv("FLY_ETCD_URL")
		if rawBackendStoreURL == "" {
			rawBackendStoreURL = os.Getenv("ETCD_URL")
		}
		if rawBackendStoreURL == "" {
			return nil, errors.New("FLY_ETCD_URL or ETCD_URL are required")
		}
	default:
		return nil, errors.New(fmt.Sprintf("Backend store %q is not supported", node.BackendStore))
	}

	node.BackendStoreURL, err = url.Parse(rawBackendStoreURL)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing backend store url")
	}

	node.KeeperUID = keeperUID(node.PrivateIP)
	node.StoreNode = strings.TrimPrefix(path.Join(node.BackendStoreURL.Path, node.KeeperUID), "/")

	node.SUCredentials = Credentials{
		Username: envOrDefault("SU_USERNAME", "flypgadmin"),
		Password: envOrDefault("SU_PASSWORD", "supassword"),
	}

	node.ReplCredentials = Credentials{
		Username: envOrDefault("REPL_USERNAME", "repluser"),
		Password: envOrDefault("REPL_PASSWORD", "replpassword"),
	}

	node.OperatorCredentials = Credentials{
		Username: envOrDefault("OPERATOR_USERNAME", "postgres"),
		Password: envOrDefault("OPERATOR_PASSWORD", "operatorpassword"),
	}

	if port, err := strconv.Atoi(os.Getenv("PG_PORT")); err == nil {
		node.PGPort = port
	}

	if port, err := strconv.Atoi(os.Getenv("PG_PROXY_PORT")); err == nil {
		node.PGProxyPort = port
	}

	return node, nil
}

func (n *Node) IsPrimaryRegion() bool {
	return n.Region == n.PrimaryRegion
}

func (n *Node) NewLeaderConnection(ctx context.Context) (*pgx.Conn, error) {
	addrs, err := privnet.AllPeers(ctx, n.AppName)
	if err != nil {
		return nil, err
	}
	if len(addrs) < 1 {
		return nil, fmt.Errorf("no peers found for app: %s", n.AppName)
	}
	hosts := make([]string, len(addrs))
	for i, v := range addrs {
		hosts[i] = net.JoinHostPort(v.IP.String(), strconv.Itoa(n.PGPort))
	}
	conn, err := openConnection(ctx, hosts, "read-write", n.SUCredentials)

	if err != nil {
		return nil, fmt.Errorf("%s, ips: %s", err, strings.Join(hosts, ", "))
	}
	return conn, err
}

func (n *Node) NewLocalConnection(ctx context.Context) (*pgx.Conn, error) {
	host := net.JoinHostPort(n.PrivateIP.String(), strconv.Itoa(n.PGPort))
	return openConnection(ctx, []string{host}, "any", n.SUCredentials)
}

func (n *Node) NewProxyConnection(ctx context.Context) (*pgx.Conn, error) {
	host := net.JoinHostPort(n.PrivateIP.String(), strconv.Itoa(n.PGProxyPort))
	return openConnection(ctx, []string{host}, "any", n.SUCredentials)
}

func envOrDefault(name, defaultVal string) string {
	val, ok := os.LookupEnv(name)
	if ok {
		return val
	}
	return defaultVal
}
