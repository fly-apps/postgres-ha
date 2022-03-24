// Package commands implements an http server that runs postgres commands like create database, create user, etc.
package commands

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
)

type Commands struct {
	pg *pgx.Conn
}

func New(pg *pgx.Conn) *Commands {
	return &Commands{pg: pg}
}

func ListDatabases(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

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

func ListUsers(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

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

	rows, err := pg.Query(ctx, sql)
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

func CreateUser(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`CREATE USER %q WITH LOGIN PASSWORD '%s'`, input["username"], input["password"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	if val, ok := input["superuser"]; ok && val == true {
		return GrantSuperuser(ctx, input)
	}

	return true, nil
}

func DeleteUser(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`DROP USER IF EXISTS %q`, input["username"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func CreateDatabase(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("CREATE DATABASE %q;", input["name"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func DeleteDatabase(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("DROP DATABASE %q;", input["name"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func GrantAccess(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %q TO %q", input["database"], input["username"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func RevokeAccess(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("REVOKE ALL PRIVILEGES ON DATABASE %q FROM %q", input["database"], input["username"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func GrantSuperuser(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER USER %q WITH SUPERUSER;", input["username"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}

func RevokeSuperuser(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pg, err := getConnection(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER USER %q WITH NOSUPERUSER;", input["username"])

	_, err = pg.Exec(ctx, sql)
	if err != nil {
		return false, err
	}

	return true, nil
}
