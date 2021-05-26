package flypg

import (
	"context"
	"fmt"
	"os"
	"time"

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

	result := make(chan *pgx.Conn, len(hosts))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, host := range hosts {
		url := fmt.Sprintf("postgres://%s/postgres?target_session_attrs=%s", host, mode)
		conf, err := pgx.ParseConfig(url)
		if err != nil {
			return nil, err
		}
		conf.User = creds.Username
		conf.Password = creds.Password
		conf.ConnectTimeout = 5 * time.Second

		go func() {
			if cnn, err := pgx.ConnectConfig(ctx, conf); err == nil {
				result <- cnn
			}
		}()
	}

	select {
	case cnn := <-result:
		return cnn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
