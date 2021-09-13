package flycheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
)

// CheckPostgreSQL health, replication, etc
func CheckPostgreSQL(ctx context.Context, node *flypg.Node, passed []string, failed []error) ([]string, []error) {
	var msg string

	// Proxy connection should always point to the leader.
	leaderConn, err := node.NewProxyConnection(ctx)
	if err != nil {
		err = fmt.Errorf("proxy: %v", err.Error())
		return passed, append(failed, errors.Wrap(err, "failed to establish proxy conn"))
	}
	defer leaderConn.Close(ctx)

	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		return passed, append(failed, errors.Wrap(err, "failed to establish local conn"))
	}
	defer localConn.Close(ctx)

	// Resolve the leader address from the proxy connection.
	leaderAddr, err := resolveServerAddr(ctx, leaderConn)
	if err != nil {
		return passed, append(failed, errors.Wrap(err, "failed to resolve leader addr"))
	}

	isLeader := (leaderAddr == node.PrivateIP.String())

	// Leader specific checks
	if isLeader {
		// Verify that leader is writable.
		msg, err = transactionReadOnly(ctx, leaderConn, "off")
		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}

		entries, err := replicationEntries(ctx, leaderConn)
		if err != nil {
			failed = append(failed, errors.Wrap(err, "failed to query replication lag"))
		} else {
			for _, entry := range entries {
				msg := fmt.Sprintf("%s is lagging %s", entry.Client, entry.ReplayLag)

				if entry.ReplayLag >= 3*time.Second {
					failed = append(failed, fmt.Errorf(msg))
				} else {
					passed = append(passed, msg)
				}
			}
		}
	}

	// Standby specific checks
	if !isLeader {
		// Verify standby is readonly
		msg, err = transactionReadOnly(ctx, localConn, "on")
		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}

		// Verify standby is subscribed to the expected leader
		ldrAddr, err := resolvePrimaryFromStandby(ctx, localConn)
		if err != nil {
			failed = append(failed, fmt.Errorf("failed to resolve primary from local conn"))
		}
		if ldrAddr == leaderAddr {
			passed = append(passed, fmt.Sprintf("connected to leader"))
		} else {
			msg = fmt.Sprintf("leader mismatch detected: current: %s, expected %s\n", leaderAddr, ldrAddr)
			fmt.Println(msg)
			failed = append(failed, fmt.Errorf(msg))
		}
	}

	// Generic checks
	msg, err = connectionCount(ctx, localConn)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}

	return passed, failed
}

func transactionReadOnly(ctx context.Context, conn *pgx.Conn, expected string) (string, error) {
	var readonly string
	err := conn.QueryRow(ctx, "SHOW transaction_read_only;").Scan(&readonly)
	if err != nil {
		return "", fmt.Errorf("readOnlyMode: %v", err)
	}

	if readonly == expected {
		return fmt.Sprintf("readOnlyMode: %s", readonly), nil
	}
	return "", fmt.Errorf("readOnlyMode: %s", readonly)

}

type ReplicationClient struct {
	Client    string
	ReplayLag time.Duration
}

func replicationEntries(ctx context.Context, leader *pgx.Conn) ([]ReplicationClient, error) {
	sql := `select client_addr, replay_lag from pg_stat_replication;`
	rows, err := leader.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	var entries []ReplicationClient
	for rows.Next() {
		var clientAddr net.IPNet
		var replayLag pgtype.Interval
		err = rows.Scan(&clientAddr, &replayLag)
		if err != nil {
			return nil, err
		}
		var dur time.Duration
		dur = time.Duration(replayLag.Microseconds)

		entry := ReplicationClient{
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
		return "", fmt.Errorf("connections: %v", err)
	}

	return fmt.Sprintf("connections: %d used, %d reserved, %d max", used, reserved, max), nil
}

// resolvePrimary works to resolve the primary address by parsing the primary_conninfo
// configuration setting.
func resolvePrimaryFromStandby(ctx context.Context, local *pgx.Conn) (string, error) {
	rows, err := local.Query(context.TODO(), "show primary_conninfo;")
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
	rows, err := conn.Query(context.TODO(), "SELECT inet_server_addr();")
	if err != nil {
		return "", err
	}
	var addr *net.IPNet
	for rows.Next() {
		err = rows.Scan(&addr)
		if err != nil {
			return "", err
		}
	}
	return addr.IP.String(), nil
}
