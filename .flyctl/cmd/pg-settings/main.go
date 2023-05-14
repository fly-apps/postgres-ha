package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fly-apps/postgres-ha/pkg/flypg"
	"github.com/fly-apps/postgres-ha/pkg/util"
)

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

	var values []flypg.Setting
	var confMap map[string]string
	for rows.Next() {

		s := flypg.Setting{}

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

		values = append(values, s)
	}

	var settings = flypg.Settings{
		Settings: values,
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
