package main

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/fly-examples/postgres-ha/.flyd/scripts/util"
)

func main() {
	args := []string{"-m", "smart", "-D", "/data/postgres/", "restart"}
	subProcess := exec.Command("gosu stolon pg_ctl", args...)

	subProcess.Stdout = io.Discard

	if err := subProcess.Run(); err != nil {
		util.WriteError(err)
	}

	if subProcess.ProcessState.ExitCode() != 0 {
		util.WriteError(fmt.Errorf(subProcess.ProcessState.String()))
	}

	util.WriteOutput("Success")
}
