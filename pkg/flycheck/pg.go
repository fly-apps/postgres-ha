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

	proxyTime := time.Now()
	leaderConn, err := node.NewProxyConnection(ctx)
	if err != nil {
		err = fmt.Errorf("proxy: %v", err.Error())
		return passed, append(failed, err)
	}
	defer leaderConn.Close(ctx)
	fmt.Printf("Time took to open proxy connection %s.\n", time.Since(proxyTime))

	// Resolve the leader address from the proxy connection.
	lResolveTime := time.Now()
	leaderAddr, err := resolveServerIp(ctx, leaderConn)
	if err != nil {
		err = fmt.Errorf("failed to resolve leader address from proxy conn: %q", err.Error())
		return passed, append(failed, err)
	}
	fmt.Printf("Time took to resolve leader connection %s.\n", time.Since(lResolveTime))

	fmt.Printf("Leader IP: %s\n", leaderAddr)
	fmt.Printf("%s == %s", leaderAddr, node.PrivateIP.String())

	localTime := time.Now()
	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		err = fmt.Errorf("local: %v", err.Error())
		return passed, append(failed, err)
	}
	defer localConn.Close(ctx)
	fmt.Printf("Time took to open local connection %s.\n", time.Since(localTime))

	isLeader := (leaderAddr == node.PrivateIP.String())
	// Primary specific checks
	if isLeader {
		// Verify leader is not readonly
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
		sReadtime := time.Now()
		msg, err = connReadOnly(ctx, localConn, "standby", true)
		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}
		fmt.Printf("Time took to check readonly: %v\n", time.Since(sReadtime))

		laTime := time.Now()
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
		fmt.Printf("Time took to verify connected leader matches: %v\n", time.Since(laTime))
	}

	cTime := time.Now()
	msg, err = connectionCount(ctx, localConn)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}
	fmt.Printf("Took took to check connection count %s.\n", time.Since(cTime))

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
	passed := true
	for rows.Next() {
		var clientAddr net.IPNet
		var replayLag pgtype.Interval
		err = rows.Scan(&clientAddr, &replayLag)
		if err != nil {
			return "", err
		}

		var dur time.Duration
		dur = time.Duration(replayLag.Microseconds)

		note := fmt.Sprintf("%s is lagging %s", clientAddr.IP.String(), dur)
		cases = append(cases, note)

		if dur >= 3*time.Second {
			passed = false
		}
	}

	if passed {
		return strings.Join(cases, "\n"), nil
	}

	return "", fmt.Errorf(strings.Join(cases, "\n"))
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
