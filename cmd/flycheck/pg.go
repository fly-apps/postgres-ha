package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

// CheckPostgreSQL health, replication, etc
func CheckPostgreSQL(hostname string, passed []string, failed []error) ([]string, []error) {
	var msg string

	leaderConn, err := openLeaderConnection(hostname)
	if err != nil {
		err = fmt.Errorf("leader check: %v", err.Error())
		return passed, append(failed, err)
	}
	defer leaderConn.Close(context.Background())

	localConn, err := openLocalConnection()
	if err != nil {
		err = fmt.Errorf("pg: %v", err.Error())
		return passed, append(failed, err)
	}
	defer localConn.Close(context.Background())

	isLeader := leaderConn.PgConn().Conn().RemoteAddr().String() == localConn.PgConn().Conn().RemoteAddr().String()
	if isLeader {
		passed = append(passed, "replication: currently leader")
	}

	if !isLeader {
		msg, err = leaderAvailable(leaderConn)

		if err != nil {
			failed = append(failed, err)
		} else {
			passed = append(passed, msg)
		}
	}

	if !isLeader {
		msg, err = replicationLag(leaderConn, localConn)
		if err != nil {
			if err != pgx.ErrNoRows {
				failed = append(failed, err)
			}
		} else {
			passed = append(passed, msg)
		}
	}

	msg, err = connectionCount(localConn)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}

	return passed, failed
}

// PostgreSQLRole outputs current role
func PostgreSQLRole(hostname string) {
	leaderConn, err := openLeaderConnection(hostname)
	if err != nil {
		fmt.Println("unknown")
		os.Exit(1)
		return
	}
	defer leaderConn.Close(context.Background())

	localConn, err := openLocalConnection()
	if err != nil {
		fmt.Println("offline")
		os.Exit(1)
		return
	}
	defer localConn.Close(context.Background())

	isLeader := leaderConn.PgConn().Conn().RemoteAddr().String() == localConn.PgConn().Conn().RemoteAddr().String()

	if isLeader {
		fmt.Println("leader")
	} else {
		fmt.Println("replica")
	}
}

func leaderAvailable(conn *pgx.Conn) (string, error) {
	var readonly string
	err := conn.QueryRow(context.Background(), "SHOW transaction_read_only").Scan(&readonly)
	if err != nil {
		return "", fmt.Errorf("leader check: %v", err)
	}

	if readonly == "on" {
		return "", fmt.Errorf("leader check: %v", err)
	}
	return fmt.Sprintf("leader check: %s connected", conn.PgConn().Conn().RemoteAddr()), nil
}

func replicationLag(leader *pgx.Conn, local *pgx.Conn) (string, error) {
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

	err = leader.QueryRow(context.Background(), sql).Scan(&delay)

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

func connectionCount(local *pgx.Conn) (string, error) {
	sql := `select used, res_for_super as reserved, max_conn as max from
			(select count(*) used from pg_stat_activity) q1,
			(select setting::int res_for_super from pg_settings where name=$$superuser_reserved_connections$$) q2,
			(select setting::int max_conn from pg_settings where name=$$max_connections$$) q3`

	var used, reserved, max int

	err := local.QueryRow(context.Background(), sql).Scan(&used, &reserved, &max)

	if err != nil {
		return "", fmt.Errorf("connections: %v", err)
	}

	return fmt.Sprintf("connections: %d used, %d reserved, %d max", used, reserved, max), nil
}

func openLeaderConnection(hostname string) (*pgx.Conn, error) {
	addrs, err := get6PN(hostname)
	if err != nil {
		return nil, err
	}
	if len(addrs) < 1 {
		return nil, fmt.Errorf("No 6PN found for hostname: %s", hostname)
	}
	hosts := make([]string, len(addrs))
	for i, v := range addrs {
		hosts[i] = fmt.Sprintf("[%v]:%s", v.String(), pgPort())
	}
	conn, err := openConnection(hosts, "read-write")

	if err != nil {
		return nil, fmt.Errorf("%s, ips: %s", err, strings.Join(hosts, ", "))
	}
	return conn, err
}

func openLocalConnection() (*pgx.Conn, error) {
	host := os.Getenv("FLY_LOCAL_6PN")
	if host == "" {
		host = "fly-local-6pn"
	}

	host = net.JoinHostPort(host, pgPort())

	return openConnection([]string{host}, "any")
}

func pgPort() string {
	port := os.Getenv("PG_PORT")
	if port == "" {
		return "5433" // our default port for pg direct
	}
	return port
}

func openConnection(hosts []string, mode string) (*pgx.Conn, error) {
	if mode == "" {
		mode = "any"
	}
	url := fmt.Sprintf("postgres://%s/postgres?target_session_attrs=%s", strings.Join(hosts, ","), mode)
	conf, err := pgx.ParseConfig(url)

	if err != nil {
		return nil, err
	}
	conf.User = "flypgadmin"
	conf.Password = os.Getenv("SU_PASSWORD")

	return pgx.ConnectConfig(context.Background(), conf)
}

func get6PN(hostname string) ([]net.IPAddr, error) {
	nameserver := os.Getenv("FLY_NAMESERVER")
	if nameserver == "" {
		nameserver = "fdaa::3"
	}
	nameserver = net.JoinHostPort(nameserver, "53")
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(1000),
			}
			return d.DialContext(ctx, "udp6", nameserver)
		},
	}
	ips, err := r.LookupIPAddr(context.Background(), hostname)

	if err != nil {
		return ips, err
	}

	// make sure we're including the local ip, just in case it's not in service discovery yet
	local, err := r.LookupIPAddr(context.Background(), "fly-local-6pn")

	if err != nil || len(local) < 1 {
		return ips, err
	}

	localExists := false
	for _, v := range ips {
		if v.IP.String() == local[0].IP.String() {
			localExists = true
		}
	}

	if !localExists {
		ips = append(ips, local[0])
	}
	return ips, err
}
