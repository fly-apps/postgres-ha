package flypg

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/jackc/pgx/v4"
)

func NewLeaderConnection(ctx context.Context, hostname string, creds Credentials) (*pgx.Conn, error) {
	addrs, err := privnet.Get6PN(ctx, hostname)
	if err != nil {
		return nil, err
	}
	if len(addrs) < 1 {
		return nil, fmt.Errorf("no 6PN found for hostname: %s", hostname)
	}
	hosts := make([]string, len(addrs))
	for i, v := range addrs {
		hosts[i] = fmt.Sprintf("[%v]:%s", v.String(), PGPort())
	}
	conn, err := openConnection(ctx, hosts, "read-write", creds)

	if err != nil {
		return nil, fmt.Errorf("%s, ips: %s", err, strings.Join(hosts, ", "))
	}
	return conn, err
}

func NewLocalConnection(ctx context.Context, creds Credentials) (*pgx.Conn, error) {
	host := os.Getenv("FLY_LOCAL_6PN")
	if host == "" {
		host = "fly-local-6pn"
	}

	host = net.JoinHostPort(host, PGPort())

	return openConnection(ctx, []string{host}, "any", creds)
}

func NewProxyConnection(ctx context.Context, creds Credentials) (*pgx.Conn, error) {
	host := os.Getenv("FLY_LOCAL_6PN")
	if host == "" {
		host = "fly-local-6pn"
	}

	host = net.JoinHostPort(host, "5432")

	return openConnection(ctx, []string{host}, "any", creds)
}

func PGPort() string {
	port := os.Getenv("PG_PORT")
	if port == "" {
		return "5433" // our default port for pg direct
	}
	return port
}

func openConnection(ctx context.Context, hosts []string, mode string, creds Credentials) (*pgx.Conn, error) {
	if mode == "" {
		mode = "any"
	}
	url := fmt.Sprintf("postgres://%s/postgres?target_session_attrs=%s", strings.Join(hosts, ","), mode)
	conf, err := pgx.ParseConfig(url)

	if err != nil {
		return nil, err
	}
	conf.User = creds.Username
	conf.Password = creds.Password

	return pgx.ConnectConfig(ctx, conf)
}
