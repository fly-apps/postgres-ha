package main

import (
	"fmt"
	"os/exec"

	"github.com/fly-apps/postgres-ha/pkg/util"
)

func main() {
	args := []string{"stolon", "pg_ctl", "-D", "/data/postgres", "restart"}
	subProcess := exec.Command("gosu", args...)

	if err := subProcess.Run(); err != nil {
		util.WriteError(err)
	}

	if subProcess.ProcessState.ExitCode() != 0 {
		util.WriteError(fmt.Errorf(subProcess.ProcessState.String()))
	}

	util.WriteOutput("Restart completed successfully", "")
}
