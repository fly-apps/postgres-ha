package flycheck

import (
	"context"

	chk "github.com/fly-examples/postgres-ha/pkg/check"
	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/flypg/admin"
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
		return admin.ResolveRole(ctx, conn)
	})
	return checks, nil
}
