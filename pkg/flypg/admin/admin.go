package admin

import (
	"context"
	"crypto/md5"
	"fmt"
	"strings"

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
			a.rolpassword as passwordhash,
      (select array_agg(d.datname::text order by d.datname)
				from pg_database d
				WHERE datistemplate = false
				AND has_database_privilege(u.usename, d.datname, 'CONNECT')
			) as allowed_databases
			from pg_user u
			join pg_authid a on u.usesysid = a.oid
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
		if err := rows.Scan(&ui.Username, &ui.SuperUser, &ui.PasswordHash, &ui.Databases); err != nil {
			return nil, err
		}
		values = append(values, ui)
	}

	return values, nil
}

func FindUser(ctx context.Context, pg *pgx.Conn, username string) (*UserInfo, error) {
	users, err := ListUsers(ctx, pg)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if u.Username == username {
			return &u, nil
		}
	}

	return nil, nil
}
