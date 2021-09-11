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
)

// CheckPostgreSQL health, replication, etc
func CheckPostgreSQL(ctx context.Context, node *flypg.Node, passed []string, failed []error) ([]string, []error) {
	var msg string

	pTime := time.Now()
	leaderConn, err := node.NewProxyConnection(ctx)
	if err != nil {
		err = fmt.Errorf("proxy: %v", err.Error())
		fmt.Printf("Proxy error: %s\n", err)
		// return passed, append(failed, err)
	}
	defer leaderConn.Close(ctx)
	fmt.Printf("It took %s to open up proxy connection.", time.Since(pTime))

	// Resolve the leader address from the proxy connection.
	leaderAddr, err := resolveServerIp(ctx, leaderConn)
	if err != nil {
		err = fmt.Errorf("failed to resolve leader address from proxy conn: %q", err.Error())
		return passed, append(failed, err)
	}

	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		err = fmt.Errorf("local: %v", err.Error())
		return passed, append(failed, err)
	}
	defer localConn.Close(ctx)

	isLeader := (leaderAddr == node.PrivateIP.String())

	// Primary specific checks
	if isLeader {
		// If we are the leader, no need to open up another connection.
		// passed = append(passed, "replication: currently leader")

		// Verify leader is not readonly
		fmt.Println("Checking readonly")
		rOnlyt := time.Now()
		msg, err = connReadOnly(ctx, leaderConn, "leader", false)
		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}
		fmt.Printf("Time took to measure readonly %s\n", time.Since(rOnlyt))

		// Replication lag checks
		replTime := time.Now()
		msg, err = replicationLag(ctx, leaderConn)
		if err != nil {
			if err != pgx.ErrNoRows {
				failed = append(failed, err)
			}
		} else {
			passed = append(passed, msg)
		}
		fmt.Printf("Time took to measure replication lag: %v\n", time.Since(replTime))

	}

	// Standby specific checks
	if !isLeader {
		// TODO - Verify standby is in ReadOnly
		msg, err = connReadOnly(ctx, localConn, "standby", true)
		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}

		// TODO - Verify primary conn data matches our leader.
		ldrAddr, err := resolvePrimaryFromStandby(ctx, localConn)
		if err != nil {
			failed = append(failed, fmt.Errorf("failed to resolve primary from local conn"))
		}
		if ldrAddr == leaderAddr {
			passed = append(passed, fmt.Sprintf("connected to leader"))
		} else {
			fmt.Printf("Incorrect leader assignment: Leader: %s, Connected to: %s\n", leaderAddr, ldrAddr)
			failed = append(failed, fmt.Errorf("incorrect leader assignment"))
		}
	}

	cTime := time.Now()
	msg, err = connectionCount(ctx, localConn)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}
	fmt.Printf("Took %s to check connection count", time.Since(cTime))

	return passed, failed
}

// PostgreSQLRole outputs current role
func PostgreSQLRole(ctx context.Context, node *flypg.Node) (string, error) {
	t := time.Now()
	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		return "offline", err
	}
	defer localConn.Close(ctx)

	fmt.Printf("Took %v to open local connection.\n", time.Since(t))

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

func connReadOnly(ctx context.Context, conn *pgx.Conn, name string, expected bool) (string, error) {
	var readonly string
	err := conn.QueryRow(ctx, "SHOW transaction_read_only;").Scan(&readonly)
	if err != nil {
		return "", fmt.Errorf("%s check: %v", name, err)
	}

	if readonly == "on" {
		if expected {
			return fmt.Sprintf("%s check: readonly", name), nil
		} else {
			return "", fmt.Errorf("%s check: readonly", name)
		}
	}
	if readonly == "off" {
		if expected {
			return "", fmt.Errorf("%s check: accepting writes", name)
		} else {
			return fmt.Sprintf("%s check: accepting writes", name), nil
		}
	}
	return "", fmt.Errorf("unable to resolve readonly state")
}

func replicationLag(ctx context.Context, leader *pgx.Conn) (string, error) {
	var cases []string
	sql := `select client_addr, replay_lag from pg_stat_replication;`
	rows, err := leader.Query(ctx, sql)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var clientAddr net.IPNet
		var replayLag pgtype.Interval
		err = rows.Scan(&clientAddr, &replayLag)
		if err != nil {
			return "", err
		}

		var dur time.Duration
		dur = time.Duration(replayLag.Microseconds)

		if dur >= 3*time.Second {
			note := fmt.Sprintf("%s is lagging behind %s", clientAddr.IP.String(), dur)
			cases = append(cases, note)
		}
	}

	output := strings.Join(cases, "\n")
	if len(cases) > 0 {
		return "", fmt.Errorf(output)
	}

	return output, nil
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

// resolveServerIp takes a connection and will return the destination server address.
func resolveServerIp(ctx context.Context, conn *pgx.Conn) (string, error) {
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
