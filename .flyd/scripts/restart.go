package main

import (
	"fmt"
	"os/exec"

	"github.com/fly-examples/postgres-ha/.flyd/scripts/util"
)

func main() {
	subProcess := exec.Command("gosu stolon pg_ctl -w -m smart -t 10 -D /data/postgres/ restart >/dev/null")

	if err := subProcess.Run(); err != nil {
		util.WriteError(err)
	}

	if subProcess.ProcessState.ExitCode() != 0 {
		util.WriteError(fmt.Errorf(subProcess.ProcessState.String()))
	}

	util.WriteOutput("Success")
}
