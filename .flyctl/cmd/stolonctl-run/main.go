package main

import (
	"fmt"
	"os"
	"os/exec"

	"encoding/base64"

	"github.com/fly-examples/postgres-ha/pkg/util"
	"github.com/google/shlex"
)

func main() {
	env, err := util.BuildEnv()
	if err != nil {
		util.WriteError(err)
	}

	encodedArg := os.Args[1]

	argBytes, err := base64.StdEncoding.DecodeString(encodedArg)
	if err != nil {
		util.WriteError(err)
	}

	args, err := shlex.Split(string(argBytes))
	if err != nil {
		util.WriteError(fmt.Errorf("error parsing argument: %w", err))
	}

	cmd := exec.Command("stolonctl", args...)
	cmd.Env = append(cmd.Env, env...)

	result, err := cmd.CombinedOutput()
	if err != nil {
		util.WriteError(err)
	}

	util.WriteOutput("Command completed successfully", string(result))
}
