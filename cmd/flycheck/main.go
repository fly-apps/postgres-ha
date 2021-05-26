package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
)

func main() {
	node, err := flypg.NewNode()
	if err != nil {
		panic(err)
	}

	ctx := context.TODO()

	categories := []string{"pg", "vm"}

	if len(os.Args) > 1 {
		categories = os.Args[1:]
	}

	var passed []string
	var failed []error

	for _, category := range categories {
		switch category {
		case "pg":
			passed, failed = CheckPostgreSQL(ctx, node, passed, failed)
		case "vm":
			passed, failed = CheckVM(passed, failed)
		case "role":
			PostgreSQLRole(ctx, node)
			return
		}
	}

	for _, v := range failed {
		fmt.Printf("[✗] %s\n", v)
	}

	for _, v := range passed {
		fmt.Printf("[✓] %s\n", v)
	}

	if len(failed) > 0 {
		os.Exit(2)
	}
}
