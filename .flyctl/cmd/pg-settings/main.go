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
	if err != nil {
		util.WriteError(err)
	}
	defer rows.Close()

	var settings Settings
	var pgConfMap map[string]string
	for rows.Next() {
		s := pgSetting{}
		if err := rows.Scan(&s.Name, &s.Setting, &s.Context, &s.Unit, &s.ShortDesc, &s.PendingRestart); err != nil {
			util.WriteError(err)
		}
		if s.PendingRestart {
			if len(pgConfMap) == 0 {
				pgConfMap, err = populatePgSettings(node.DataDir)
				if err != nil {
					util.WriteError(err)
				}
			}
			val := pgConfMap[*s.Name]
			s.PendingChange = &val
		}

		settings.Settings = append(settings.Settings, s)
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		util.WriteError(err)
	}

	util.WriteOutput("Success", string(settingsJSON))
}

func populatePgSettings(dataDir string) (map[string]string, error) {
	pathToFile := fmt.Sprintf("%s/postgres/postgresql.conf", dataDir)
	file, err := os.Open(pathToFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	settings := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		elemSlice := strings.Split(scanner.Text(), " = ")
		val := strings.Trim(elemSlice[1], "'")
		settings[elemSlice[0]] = val
	}

	return settings, err
}
