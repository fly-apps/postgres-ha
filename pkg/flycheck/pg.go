package flycheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/pkg/errors"
	chk "github.com/superfly/fly-checks/check"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
)

// CheckPostgreSQL health, replication, etc
func CheckPostgreSQL(ctx context.Context, checks *chk.CheckSuite) (*chk.CheckSuite, error) {

	node, err := flypg.NewNode()
	if err != nil {
		return checks, errors.Wrap(err, "failed to initialize node")
	}

	leaderConn, err := node.NewProxyConnection(ctx)
	if err != nil {
		return checks, errors.Wrap(err, "failed to connect to proxy")
	}

	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		return checks, errors.Wrap(err, "failed to connect with local node")
	}

	// Cleanup connections
	checks.OnCompletion = func() {
		leaderConn.Close(ctx)
		localConn.Close(ctx)
	}

	leaderAddr, err := resolveServerAddr(ctx, leaderConn)
	if err != nil {
		return checks, err
	}

	isLeader := (leaderAddr == node.PrivateIP.String())

	if isLeader {
		checks.AddCheck("transactions", func() (string, error) {
			return transactionMode(ctx, localConn, "read/write")
		})

		entries, err := replicationEntries(ctx, leaderConn)
		if err != nil {
			return checks, errors.Wrap(err, "failed to query replication info")
		}

		for _, entry := range entries {
			msg := fmt.Sprintf("%s is lagging %s", entry.Client, entry.ReplayLag)
			lag := entry.ReplayLag
			checks.AddCheck("replicationLag", func() (string, error) {
				if lag >= 3*time.Second {
					return "", fmt.Errorf(msg)
				}
				return msg, nil
			})
		}
	}

	if !isLeader {
		checks.AddCheck("transactions", func() (string, error) {
			return transactionMode(ctx, localConn, "readonly")
		})
		// Ensures the the Proxy address and the primary adddress
		// we are receiving updates from are the same.
		checks.AddCheck("replication", func() (string, error) {
			return connectedToLeader(ctx, localConn, leaderAddr)
		})
	}

	checks.AddCheck("connections", func() (string, error) {
		return connectionCount(ctx, localConn)
	})

	return checks, nil
}

func connectedToLeader(ctx context.Context, conn *pgx.Conn, leaderAddr string) (string, error) {
	ldrAddr, err := resolvePrimaryFromStandby(ctx, conn)
	if err != nil {
		return "", err
	}
	if ldrAddr == leaderAddr {
		return fmt.Sprintf("syncing from %s", leaderAddr), nil
	}
	return "", fmt.Errorf("primary mismatch detected: current: %q, expected %q", leaderAddr, ldrAddr)
}

func transactionMode(ctx context.Context, conn *pgx.Conn, expected string) (string, error) {
	var readonly string
	conn.QueryRow(ctx, "SHOW transaction_read_only;").Scan(&readonly)

	var state string
	if readonly == "on" {
		state = "readonly"
	}
	if readonly == "off" {
		state = "read/write"
	}

	if state != expected {
		return "", fmt.Errorf("%s but expected %s", state, expected)
	}
	return state, nil
}

type ReplicationEntry struct {
	Client    string
	ReplayLag time.Duration
}

func replicationEntries(ctx context.Context, leader *pgx.Conn) ([]ReplicationEntry, error) {
	sql := `select client_addr, replay_lag from pg_stat_replication;`
	rows, err := leader.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ReplicationEntry
	for rows.Next() {
		var clientAddr net.IPNet
		var replayLag pgtype.Interval
		err = rows.Scan(&clientAddr, &replayLag)
		if err != nil {
			return nil, err
		}

		dur := time.Duration(replayLag.Microseconds)

		entry := ReplicationEntry{
			Client:    clientAddr.IP.String(),
			ReplayLag: dur,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func connectionCount(ctx context.Context, local *pgx.Conn) (string, error) {
	sql := `select used, res_for_super as reserved, max_conn as max from
			(select count(*) used from pg_stat_activity) q1,
			(select setting::int res_for_super from pg_settings where name=$$superuser_reserved_connections$$) q2,
			(select setting::int max_conn from pg_settings where name=$$max_connections$$) q3`

	var used, reserved, max int

	err := local.QueryRow(ctx, sql).Scan(&used, &reserved, &max)

	if err != nil {
		return "", fmt.Errorf("%v", err)
	}

	return fmt.Sprintf("%d used, %d reserved, %d max", used, reserved, max), nil
}

// resolvePrimary works to resolve the primary address by parsing the primary_conninfo
// configuration setting.
func resolvePrimaryFromStandby(ctx context.Context, local *pgx.Conn) (string, error) {
	rows, err := local.Query(ctx, "show primary_conninfo;")
	if err != nil {
		return "", err
	}

	var primaryConn string
	for rows.Next() {
		err = rows.Scan(&primaryConn)
		if err != nil {
			return "", err
		}
	}
	rows.Close()

	// If we don't have any assigned primary, assume we are the primary.
	if primaryConn == "" {
		ip, err := privnet.PrivateIPv6()
		if err != nil {
			return "", err
		}
		return ip.String(), nil
	}

	connMap := map[string]string{}
	for _, entry := range strings.Split(primaryConn, " ") {
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, "=")
		connMap[parts[0]] = parts[1]
	}

	return connMap["host"], nil
}

// resolveServerAddr takes a connection and will return the destination server address.
func resolveServerAddr(ctx context.Context, conn *pgx.Conn) (string, error) {
	rows, err := conn.Query(ctx, "SELECT inet_server_addr();")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var addr *net.IPNet
	for rows.Next() {
		err = rows.Scan(&addr)
		if err != nil {
			return "", err
		}
	}
	return addr.IP.String(), nil
}
