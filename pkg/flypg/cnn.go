package flypg

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v4"
)

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
