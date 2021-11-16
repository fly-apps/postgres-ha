package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/fly-examples/postgres-ha/.flyd/scripts/util"
)

func main() {
	userPtr := flag.String("user", "postgres", "Auth user")
	passPtr := flag.String("password", "", "Auth password")
	databasePtr := flag.String("database", "postgres", "Database to target")
	cmdPtr := flag.String("command", "", "SQL command to run")
	flag.Parse()

	subProcess := exec.Command("psql", connectionString(*userPtr, *passPtr, *databasePtr))

	stdin, err := subProcess.StdinPipe()
	if err != nil {
		panic(err)
	}
	defer stdin.Close()

	var outBuf, errBuf bytes.Buffer
	subProcess.Stdout = &outBuf
	subProcess.Stderr = &errBuf

	if err = subProcess.Start(); err != nil {
		panic(err)
	}

	io.WriteString(stdin, *cmdPtr+"\n")
	io.WriteString(stdin, "\\q"+"\n")

	subProcess.Wait()

	if subProcess.ProcessState.ExitCode() != 0 {
		util.WriteError(fmt.Errorf(errBuf.String()))
		os.Exit(0)
	}

	util.WriteOutput(outBuf.String())
	os.Exit(0)
}

func connectionString(user, pass, database string) string {
	appName := os.Getenv("FLY_APP_NAME")
	if pass == "" {
		pass = os.Getenv("OPERATOR_PASSWORD")
	}

	return fmt.Sprintf("postgres://%s:%s@%s.internal:5432/%s", user, pass, appName, database)
}
