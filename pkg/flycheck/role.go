package flycheck

import (
	"context"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
)

// PostgreSQLRole outputs current role
func PostgreSQLRole(ctx context.Context, node *flypg.Node) (string, error) {
	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		return "offline", err
	}
	defer localConn.Close(ctx)

	var readonly string
	err = localConn.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly)
	if err != nil {
		return "offline", err
	}

	if readonly == "on" {
		return "replica", nil
	} else {
		return "leader", nil
	}
}
