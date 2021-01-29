package main

import (
	"fmt"
	"os"
)

func main() {
	categories := []string{"pg", "vm"}

	if len(os.Args) > 1 {
		categories = os.Args[1:]
	}

	app := os.Getenv("FLY_APP_NAME")
	hostname := fmt.Sprintf("%s.internal", app)

	var passed []string
	var failed []error

	for _, category := range categories {
		switch category {
		case "pg":
			passed, failed = CheckPostgreSQL(hostname, passed, failed)
		case "vm":
			passed, failed = CheckVM(passed, failed)
		case "role":
			PostgreSQLRole(hostname)
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
