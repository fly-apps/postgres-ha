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

type pgSettings struct {
	Settings []pgSetting `json:"settings"`
}

type pgSetting struct {
	Name           *string   `json:"name,omitempty"`
	Setting        *string   `json:"setting,omitempty"`
	Context        *string   `json:"context,omitempty"`
	VarType        *string   `json:"vartype,omitempty"`
	MinVal         *string   `json:"min_val,omitempty"`
	MaxVal         *string   `json:"max_val,omitempty"`
	EnumVals       []*string `json:"enumvals`
	Unit           *string   `json:"unit,omitempty"`
	ShortDesc      *string   `json:"short_desc,omitempty"`
	PendingChange  *string   `json:"pending_change,omitempty"`
	PendingRestart bool      `json:"pending_restart,omitempty"`
}

func main() {
	encodedArg := os.Args[1]

	sBytes, err := base64.StdEncoding.DecodeString(encodedArg)
	if err != nil {
		util.WriteError(err)
	}

	sList := strings.Split(string(sBytes), ",")
	if len(sList) == 0 {
		util.WriteError(fmt.Errorf("no settings were specified"))
	}

	node, err := flypg.NewNode()
	if err != nil {
		util.WriteError(err)
	}

	conn, err := node.NewLocalConnection(context.TODO())
	if err != nil {
		util.WriteError(err)
	}

	sValues := "'" + strings.Join(sList, "', '") + "'"

	sql := fmt.Sprintf(`select 
	name, setting, vartype, min_val, max_val, enumvals, context, unit, short_desc, pending_restart 
	FROM pg_settings WHERE name IN (%s);  	
`, sValues)

	rows, err := conn.Query(context.Background(), sql)
	if err != nil {
		util.WriteError(err)
	}
	defer rows.Close()

	var settings pgSettings
	var confMap map[string]string
	for rows.Next() {
		s := pgSetting{}
		err := rows.Scan(
			&s.Name,
			&s.Setting,
			&s.VarType,
			&s.MinVal,
			&s.MaxVal,
			&s.EnumVals,
			&s.Context,
			&s.Unit,
			&s.ShortDesc,
			&s.PendingRestart,
		)
		if err != nil {
			util.WriteError(err)
		}
		if s.PendingRestart {
			if len(confMap) == 0 {
				confMap, err = populatePgSettings(node.DataDir)
				if err != nil {
					util.WriteError(err)
				}
			}
			val := confMap[*s.Name]
			s.PendingChange = &val
		}

		settings.Settings = append(settings.Settings, s)
	}

	respBytes, err := json.Marshal(settings)
	if err != nil {
		util.WriteError(err)
	}

	util.WriteOutput("Success", string(respBytes))
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
