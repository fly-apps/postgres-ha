package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/jackc/pgx/v4"
)

// CheckPostgreSQL health, replication, etc
func CheckPostgreSQL(ctx context.Context, node *flypg.Node, passed []string, failed []error) ([]string, []error) {
	var msg string

	leaderConn, err := node.NewLeaderConnection(ctx)
	if err != nil {
		err = fmt.Errorf("leader check: %v", err.Error())
		return passed, append(failed, err)
	}
	defer leaderConn.Close(ctx)

	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		err = fmt.Errorf("pg: %v", err.Error())
		return passed, append(failed, err)
	}
	defer localConn.Close(ctx)

	proxyConn, err := node.NewProxyConnection(ctx)
	if err != nil {
		err = fmt.Errorf("proxy: %v", err.Error())
		return passed, append(failed, err)
	}
	defer proxyConn.Close(ctx)

	isLeader := leaderConn.PgConn().Conn().RemoteAddr().String() == localConn.PgConn().Conn().RemoteAddr().String()
	if isLeader {
		passed = append(passed, "replication: currently leader")
	}

	if !isLeader {
		msg, err = leaderAvailable(ctx, leaderConn, "leader")

		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}
	}

	if !isLeader {
		msg, err = replicationLag(ctx, leaderConn, localConn)
		if err != nil {
			if err != pgx.ErrNoRows {
				failed = append(failed, err)
			}
		} else {
			passed = append(passed, msg)
		}
	}

	msg, err = leaderAvailable(ctx, proxyConn, "proxy")

	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}

	msg, err = connectionCount(ctx, localConn)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}

	return passed, failed
}

// PostgreSQLRole outputs current role
func PostgreSQLRole(ctx context.Context, node *flypg.Node) {
	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		fmt.Println("offline")
		os.Exit(1)
		return
	}
	defer localConn.Close(ctx)

	var readonly string
	err = localConn.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly)
	if err != nil {
		fmt.Println("offline")
		os.Exit(1)
		return
	}

	if readonly == "on" {
		fmt.Println("replica")
	} else {
		fmt.Println("leader")
	}
}

func leaderAvailable(ctx context.Context, conn *pgx.Conn, name string) (string, error) {
	var readonly string
	err := conn.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly)
	if err != nil {
		return "", fmt.Errorf("%s check: %v", name, err)
	}

	if readonly == "on" {
		return "", fmt.Errorf("%s check: %v", name, err)
	}
	return fmt.Sprintf("%s check: %s connected", name, conn.PgConn().Conn().RemoteAddr()), nil
}

func replicationLag(ctx context.Context, leader *pgx.Conn, local *pgx.Conn) (string, error) {
	if leader.PgConn().Conn().RemoteAddr().String() == local.PgConn().Conn().RemoteAddr().String() {
		return "replication lag: currently leader", nil
	}

	self := local.PgConn().Conn().RemoteAddr().String()
	localhost, _, err := net.SplitHostPort(self)

	if err != nil {
		return "", fmt.Errorf("replication lag: couldn't get localhost, %v", err)
	}
	sql := fmt.Sprintf("select COALESCE(write_lag, '0 seconds'::interval) as delay from pg_stat_replication where client_addr='%s'", localhost)

	var delay time.Duration

	err = leader.QueryRow(ctx, sql).Scan(&delay)

	if err != nil {
		if err == pgx.ErrNoRows {
			return "replication lag: no lag", nil
		}
		return "", fmt.Errorf("replication lag: %v", err)
	}

	msg := fmt.Sprintf("replication lag: %v", delay)
	if delay > (time.Second * 10) {
		return "", fmt.Errorf(msg)
	}

	return msg, nil
}

func connectionCount(ctx context.Context, local *pgx.Conn) (string, error) {
	sql := `select used, res_for_super as reserved, max_conn as max from
			(select count(*) used from pg_stat_activity) q1,
			(select setting::int res_for_super from pg_settings where name=$$superuser_reserved_connections$$) q2,
			(select setting::int max_conn from pg_settings where name=$$max_connections$$) q3`

	var used, reserved, max int

	err := local.QueryRow(ctx, sql).Scan(&used, &reserved, &max)

	if err != nil {
		return "", fmt.Errorf("connections: %v", err)
	}

	return fmt.Sprintf("connections: %d used, %d reserved, %d max", used, reserved, max), nil
}
