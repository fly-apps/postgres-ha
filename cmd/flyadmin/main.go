package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type cmd func(pg *pgx.Conn, input map[string]interface{}) (result interface{}, err error)

func main() {
	app := os.Getenv("FLY_APP_NAME")
	hostname := fmt.Sprintf("%s.internal", app)
	cnn, err := openLeaderConnection(hostname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to postgres: %s\n", err)
		os.Exit(1)
	}
	defer cnn.Close(context.Background())

	if len(os.Args) == 1 {
		fmt.Fprintln(os.Stderr, "subcommand required")
		os.Exit(1)
	}

	command := os.Args[1]
	input := map[string]interface{}{}

	if len(os.Args) > 2 && os.Args[2] != "" {
		if err := json.Unmarshal([]byte(os.Args[2]), &input); err != nil {
			fmt.Fprintf(os.Stderr, "error decoding json input: %s\n", err)
			os.Exit(1)
		}
	}

	commands := map[string]cmd{
		"database-list":    listDatabases,
		"database-create":  createDatabase,
		"database-delete":  deleteDatabase,
		"user-list":        listUsers,
		"user-create":      createUser,
		"user-delete":      deleteUser,
		"grant-access":     grantAccess,
		"revoke-access":    revokeAccess,
		"grant-superuser":  grantSuperuser,
		"revoke-superuser": revokeSuperuser,
	}

	cmd := commands[command]
	if cmd == nil {
		fmt.Fprintf(os.Stderr, "unknown command '%s'\n", command)
		os.Exit(1)
	}

	output, err := cmd(cnn, input)
	resp := response{
		Result: output,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling response '%s'\n", err)
		os.Exit(1)
	}
}

type response struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

func listDatabases(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := `
		SELECT d.datname,
					(SELECT array_agg(u.usename::text order by u.usename) 
						from pg_user u 
						where has_database_privilege(u.usename, d.datname, 'CONNECT')) as allowed_users
		from pg_database d where d.datistemplate = false
		order by d.datname;
		`

	rows, err := pg.Query(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []dbInfo{}

	for rows.Next() {
		di := dbInfo{}
		if err := rows.Scan(&di.Name, &di.Users); err != nil {
			return nil, err
		}
		values = append(values, di)
	}

	return values, nil
}

type userInfo struct {
	Username  string   `json:"username"`
	SuperUser bool     `json:"superuser"`
	Databases []string `json:"databases"`
}

type dbInfo struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
}

func listUsers(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := `
		select u.usename,
			usesuper as superuser,
      (select array_agg(d.datname::text order by d.datname)
				from pg_database d
				WHERE datistemplate = false
				AND has_database_privilege(u.usename, d.datname, 'CONNECT')
			) as allowed_databases
			from pg_user u
			order by u.usename
			`

	rows, err := pg.Query(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []userInfo{}

	for rows.Next() {
		ui := userInfo{}
		if err := rows.Scan(&ui.Username, &ui.SuperUser, &ui.Databases); err != nil {
			return nil, err
		}
		values = append(values, ui)
	}

	return values, nil
}

func createUser(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf(`CREATE USER %s WITH LOGIN PASSWORD '%s'`, input["username"], input["password"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	if val, ok := input["superuser"]; ok && val == true {
		return grantSuperuser(pg, input)
	}

	return true, nil
}

func deleteUser(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf(`DROP USER IF EXISTS %s`, input["username"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func createDatabase(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf("CREATE DATABASE %s;", input["name"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func deleteDatabase(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf("DROP DATABASE %s;", input["name"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func grantAccess(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", input["database"], input["username"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func revokeAccess(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf("REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s", input["database"], input["username"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func grantSuperuser(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf("ALTER USER %s WITH SUPERUSER;", input["username"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func revokeSuperuser(pg *pgx.Conn, input map[string]interface{}) (interface{}, error) {
	sql := fmt.Sprintf("ALTER USER %s WITH NOSUPERUSER;", input["username"])

	_, err := pg.Exec(context.Background(), sql)
	if err != nil {
		return false, err
	}

	return true, nil
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
