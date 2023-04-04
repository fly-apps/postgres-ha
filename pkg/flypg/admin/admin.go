package admin

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"strings"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/jackc/pgx/v4"
)

func CreateUser(ctx context.Context, pg *pgx.Conn, username string, password string) error {
	sql := fmt.Sprintf(`CREATE USER %s WITH LOGIN PASSWORD '%s'`, username, password)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func GrantSuperuser(ctx context.Context, pg *pgx.Conn, username string) error {
	sql := fmt.Sprintf("ALTER USER %s WITH SUPERUSER;", username)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func GrantReplication(ctx context.Context, pg *pgx.Conn, username string) error {
	sql := fmt.Sprintf("ALTER USER %s WITH REPLICATION;", username)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func ChangePassword(ctx context.Context, pg *pgx.Conn, username, password string) error {
	sql := fmt.Sprintf("ALTER USER %s WITH LOGIN PASSWORD '%s';", username, password)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func ListDatabases(ctx context.Context, pg *pgx.Conn) ([]DbInfo, error) {
	sql := `
		SELECT d.datname,
					(SELECT array_agg(u.usename::text order by u.usename) 
						from pg_user u 
						where has_database_privilege(u.usename, d.datname, 'CONNECT')) as allowed_users
		from pg_database d where d.datistemplate = false
		order by d.datname;
		`

	rows, err := pg.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []DbInfo{}

	for rows.Next() {
		di := DbInfo{}
		if err := rows.Scan(&di.Name, &di.Users); err != nil {
			return nil, err
		}
		values = append(values, di)
	}

	return values, nil
}

type UserInfo struct {
	Username     string   `json:"username"`
	SuperUser    bool     `json:"superuser"`
	ReplUser     bool     `json:"repluser"`
	Databases    []string `json:"databases"`
	PasswordHash string   `json:"-"`
}

func (ui UserInfo) IsPassword(password string) bool {
	if !strings.HasPrefix(ui.PasswordHash, "md5") {
		return false
	}

	encoded := fmt.Sprintf("md5%x", md5.Sum([]byte(password+ui.Username)))
	return encoded == ui.PasswordHash
}

type DbInfo struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
}

func ListUsers(ctx context.Context, pg *pgx.Conn) ([]UserInfo, error) {
	sql := `
	select u.usename,
		usesuper as superuser,
		userepl as repluser,
		a.rolpassword as passwordhash,
    (
		select array_agg(d.datname::text order by d.datname)
			from pg_database d
				WHERE datistemplate = false
				AND has_database_privilege(u.usename, d.datname, 'CONNECT')
	) as 
		allowed_databases
	from 
		pg_user u join pg_authid a on u.usesysid = a.oid 
	order by u.usename
	`

	rows, err := pg.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []UserInfo{}

	for rows.Next() {
		ui := UserInfo{}
		if err := rows.Scan(&ui.Username, &ui.SuperUser, &ui.ReplUser, &ui.PasswordHash, &ui.Databases); err != nil {
			return nil, err
		}
		values = append(values, ui)
	}

	return values, nil
}

func FindUser(ctx context.Context, pg *pgx.Conn, username string) (*UserInfo, error) {
	sql := `
	select 
		u.usename,
        usesuper as superuser,
        userepl as repluser,
    	a.rolpassword as passwordhash,
    (
        SELECT 
			array_agg(d.datname::text order by d.datname)
        	from pg_database d
        WHERE datistemplate = false AND has_database_privilege(u.usename, d.datname, 'CONNECT')
    ) AS 
		allowed_databases
    FROM 
		pg_user u join pg_authid a on u.usesysid = a.oid 
	WHERE u.usename='%s';`

	sql = fmt.Sprintf(sql, username)

	row := pg.QueryRow(ctx, sql)

	var user = UserInfo{}

	if err := row.Scan(&user.Username, &user.SuperUser, &user.ReplUser, &user.PasswordHash, &user.Databases); err != nil {
		return nil, err
	}
	return &user, nil

}

func DeleteUser(ctx context.Context, pg *pgx.Conn, username string) error {
	sql := fmt.Sprintf("DROP USER %s", username)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func CreateDatabase(ctx context.Context, pg *pgx.Conn, name string) error {
	sql := fmt.Sprintf("CREATE DATABASE %s;", name)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func DeleteDatabase(ctx context.Context, pg *pgx.Conn, name string) error {
	sql := fmt.Sprintf("DROP DATABASE %s;", name)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func FindDatabase(ctx context.Context, pg *pgx.Conn, name string) (*DbInfo, error) {
	sql := `
	SELECT 
		datname, 
		(SELECT array_agg(u.usename::text order by u.usename) FROM pg_user u WHERE has_database_privilege(u.usename, d.datname, 'CONNECT')) as allowed_users 
	FROM pg_database d WHERE d.datname='%s';
	`

	sql = fmt.Sprintf(sql, name)

	row := pg.QueryRow(ctx, sql)

	db := new(DbInfo)
	if err := row.Scan(&db.Name, &db.Users); err != nil {
		return nil, err
	}

	return db, nil
}

func GrantAccess(ctx context.Context, pg *pgx.Conn, database, username string) error {
	sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %q TO %q", database, username)

	_, err := pg.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

func ResolveRole(ctx context.Context, pg *pgx.Conn) (string, error) {
	var readonly string
	err := pg.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly)
	if err != nil {
		return "offline", err
	}

	if readonly == "on" {
		return "replica", nil
	}
	return "leader", nil
}

type ReplicationStat struct {
	Name string
	Diff int
}

func ResolveReplicationLag(ctx context.Context, pg *pgx.Conn) ([]*ReplicationStat, error) {
	sql := "select application_name, pg_current_wal_lsn() - flush_lsn as diff from pg_stat_replication;"

	rows, err := pg.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []*ReplicationStat
	for rows.Next() {
		var s ReplicationStat
		if err := rows.Scan(&s.Name, &s.Diff); err != nil {
			return nil, err
		}
		stats = append(stats, &s)
	}
	return stats, nil
}

func ResolveSettings(ctx context.Context, pg *pgx.Conn, list []string) (*flypg.Settings, error) {
	node, err := flypg.NewNode()
	if err != nil {
		return nil, err
	}
	sValues := "'" + strings.Join(list, "', '") + "'"

	sql := fmt.Sprintf(`
	SELECT
		name, 
		setting, 
		vartype, 
		min_val, 
		max_val, 
		enumvals, 
		context, 
		unit, 
		short_desc, 
		pending_restart 
	FROM pg_settings WHERE name IN (%s);`, sValues)

	rows, err := pg.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var confMap map[string]string

	var values []flypg.Setting

	for rows.Next() {
		var s flypg.Setting

		if err := rows.Scan(&s.Name, &s.Setting, &s.VarType, &s.MinVal, &s.MaxVal, &s.EnumVals, &s.Context, &s.Unit, &s.ShortDesc, &s.PendingRestart); err != nil {
			return nil, err
		}
		if s.PendingRestart {
			if len(confMap) == 0 {
				confMap, err = populatePgSettings(node.DataDir)
				if err != nil {
					return nil, err
				}
			}
			val := confMap[*s.Name]
			s.PendingChange = &val
		}
		values = append(values, s)
	}

	var settings = &flypg.Settings{
		Settings: values,
	}

	return settings, nil
}

func populatePgSettings(dataDir string) (map[string]string, error) {
	pathToFile := fmt.Sprintf("%s/postgres/postgresql.conf", dataDir)
	file, err := os.Open(pathToFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sMap := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sS := strings.Split(scanner.Text(), " = ")
		val := strings.Trim(sS[1], "'")
		sMap[sS[0]] = val
	}

	return sMap, err
}
