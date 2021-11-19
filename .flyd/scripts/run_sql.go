package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fly-examples/postgres-ha/.flyd/scripts/util"
)

func main() {
	userPtr := flag.String("user", "postgres", "Auth user")
	passPtr := flag.String("password", "", "Auth password")
	databasePtr := flag.String("database", "postgres", "Database to target")
	cmdPtr := flag.String("command", "", "SQL command to run")
	flag.Parse()

	decodeCommand, err := base64.StdEncoding.DecodeString(*cmdPtr)
	if err != nil {
		util.WriteError(err)
	}

	args := []string{connectionString(*userPtr, *passPtr, *databasePtr), "-t", "-c", string(decodeCommand)}
	subProcess := exec.Command("psql", args...)

	var outBuf, errBuf bytes.Buffer
	subProcess.Stdout = &outBuf
	subProcess.Stderr = &errBuf

	err = subProcess.Start()
	if err != nil {
		util.WriteError(err)
	}

	subProcess.Wait()

	out := strings.Trim(outBuf.String(), "\n")
	out = strings.TrimSpace(out)

	errOut := strings.Trim(errBuf.String(), "\n")
	errOut = strings.TrimSpace(errOut)

	if subProcess.ProcessState.ExitCode() != 0 || errBuf.String() != "" {
		util.WriteError(fmt.Errorf(errOut))
	}

	util.WriteOutput(out)
}

func connectionString(user, pass, database string) string {
	appName := os.Getenv("FLY_APP_NAME")
	if pass == "" {
		pass = os.Getenv("OPERATOR_PASSWORD")
	}

	return fmt.Sprintf("postgres://%s:%s@%s.internal:5432/%s", user, pass, appName, database)
}
