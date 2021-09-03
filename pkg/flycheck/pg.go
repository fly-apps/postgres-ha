package flycheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/jackc/pgx/v4"
)

// CheckPostgreSQL health, replication, etc
func CheckPostgreSQL(ctx context.Context, node *flypg.Node, passed []string, failed []error) ([]string, []error) {
	var msg string

	localtime := time.Now()
	localConn, err := node.NewLocalConnection(ctx)
	if err != nil {
		err = fmt.Errorf("pg: %v", err.Error())
		return passed, append(failed, err)
	}
	defer localConn.Close(ctx)
	fmt.Printf("Took %v to open local connection.\n", time.Since(localtime))

	proxyt := time.Now()
	proxyConn, err := node.NewProxyConnection(ctx)
	if err != nil {
		err = fmt.Errorf("proxy: %v", err.Error())
		return passed, append(failed, err)
	}
	defer proxyConn.Close(ctx)
	fmt.Printf("Took %v to open proxy connection.\n", time.Since(proxyt))

	primTime := time.Now()
	primaryAddr, err := resolvePrimary(ctx, localConn)
	if err != nil {
		err = fmt.Errorf("Unable to resolve primary")
		return passed, append(failed, err)
	}
	fmt.Printf("Took %v to resolvePrimary.\n", time.Since(primTime))

	isLeader := primaryAddr == node.PrivateIP.String()
	if isLeader {
		passed = append(passed, "replication: currently leader")
	}

	// Standby specific checks
	if !isLeader {

		lTime := time.Now()

		leaderConn, err := node.NewConnection(ctx, primaryAddr)
		if err != nil {
			err = fmt.Errorf("leader check: %v", err.Error())
			return passed, append(failed, err)
		}
		defer leaderConn.Close(ctx)
		fmt.Printf("Took %v to open leader connection.\n", time.Since(lTime))

		laTime := time.Now()
		msg, err = leaderAvailable(ctx, leaderConn, "leader")
		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}
		fmt.Printf("Took %v to check leader availability.\n", time.Since(laTime))

		replTime := time.Now()
		msg, err = replicationLag(ctx, leaderConn, localConn)
		if err != nil {
			if err != pgx.ErrNoRows {
				failed = append(failed, err)
			}
		} else {
			passed = append(passed, msg)
		}
		fmt.Printf("Took %v to check replication lab.\n", time.Since(replTime))

	}

	pTime := time.Now()
	msg, err = leaderAvailable(ctx, proxyConn, "proxy")
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}
	fmt.Printf("Took %v to check leader available from proxy.\n", time.Since(pTime))

	cTime := time.Now()
	msg, err = connectionCount(ctx, localConn)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}
	fmt.Printf("Took %v to check connection count.\n", time.Since(cTime))

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

// resolvePrimary works to resolve the primary address by parsing the primary_conninfo
// configuration setting.
func resolvePrimary(ctx context.Context, local *pgx.Conn) (string, error) {
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
