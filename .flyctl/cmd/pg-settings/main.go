package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/util"
)

type Settings struct {
	Settings []pgSetting `json:"settings"`
}

type pgSetting struct {
	Name           *string `json:"name,omitempty"`
	Setting        *string `json:"setting,omitempty"`
	Context        *string `json:"context,omitempty"`
	Unit           *string `json:"unit,omitempty"`
	ShortDesc      *string `json:"short_desc,omitempty"`
	PendingChange  *string `json:"pending_change,omitempty"`
	PendingRestart bool    `json:"pending_restart,omitempty"`
}

func main() {
	encodedArg := os.Args[1]

	settingsBytes, err := base64.StdEncoding.DecodeString(encodedArg)
	if err != nil {
		util.WriteError(err)
	}

	node, err := flypg.NewNode()
	if err != nil {
		util.WriteError(err)
	}

	conn, err := node.NewLocalConnection(context.TODO())
	if err != nil {
		util.WriteError(err)
	}

	settingList := strings.Split(string(settingsBytes), ",")
	if len(settingList) == 0 {
		util.WriteError(fmt.Errorf("no settings were specified"))
	}

	settingsValues := "'" + strings.Join(settingList, "', '") + "'"

	sql := fmt.Sprintf(`
		select name, setting, context, unit, short_desc, pending_restart FROM pg_settings
		WHERE name IN (%s);  	
`, settingsValues)

	rows, err := conn.Query(context.Background(), sql)
	defer rows.Close()
	if err != nil {
		util.WriteError(err)
	}

	settings := Settings{}
	for rows.Next() {
		s := pgSetting{}
		if err := rows.Scan(&s.Name, &s.Setting, &s.Context, &s.Unit, &s.ShortDesc, &s.PendingRestart); err != nil {
			util.WriteError(err)
		}
		if s.PendingRestart {
			p, err := findPendingChange(node, *s.Name)
			if err != nil {
				util.WriteError(err)
			}
			s.PendingChange = &p
		}

		settings.Settings = append(settings.Settings, s)
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		util.WriteError(err)
	}

	util.WriteOutput("Success", string(settingsJSON))
}

func findPendingChange(node *flypg.Node, setting string) (string, error) {
	file, err := os.Open("/data/postgres/postgresql.conf")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		elemSlice := strings.Split(scanner.Text(), " = ")
		if elemSlice[0] == setting {
			val := strings.Trim(elemSlice[1], "'")
			return val, nil
		}
	}

	return "", nil
}
