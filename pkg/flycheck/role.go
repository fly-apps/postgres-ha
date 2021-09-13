package flycheck

import (
	"context"

	chk "github.com/fly-examples/postgres-ha/pkg/check"
	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
)

// PostgreSQLRole outputs current role
func PostgreSQLRole(ctx context.Context, checks *chk.CheckSuite) (*chk.CheckSuite, error) {
	node, err := flypg.NewNode()
	if err != nil {
		return checks, errors.Wrap(err, "failed to initialize node")
	}

	conn, err := node.NewLocalConnection(ctx)
	if err != nil {
		return checks, errors.Wrap(err, "failed to connect to local node")
	}

	// Cleanup connections
	checks.OnCompletion = func() {
		conn.Close(ctx)
	}

	checks.AddCheck("role", func() (string, error) {
		return resolveRole(ctx, conn)
	})
	return checks, nil
}

func resolveRole(ctx context.Context, conn *pgx.Conn) (string, error) {
	var readonly string
	err := conn.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly)
	if err != nil {
		return "offline", err
	}

	if readonly == "on" {
		return "replica", nil
	} else {
		return "leader", nil
	}
}
