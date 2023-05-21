package main

import (
	"fmt"
	"os"

	"encoding/base64"

	"github.com/fly-apps/postgres-ha/pkg/flypg/stolon"
	"github.com/fly-apps/postgres-ha/pkg/util"
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

	result, err := stolon.Ctl(args, env)
	if err != nil {
		util.WriteError(err)
	}

	util.WriteOutput("Command completed successfully", string(result))
}
