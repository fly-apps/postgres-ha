package flycheck

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
)

const Port = 5500

func StartCheckListener() {
	http.HandleFunc("/flycheck/vm", runVMChecks)
	http.HandleFunc("/flycheck/pg", runPGChecks)
	http.HandleFunc("/flycheck/role", runRoleCheck)

	fmt.Printf("Listening on port %d", Port)
	http.ListenAndServe(fmt.Sprintf(":%d", Port), nil)
}

func runVMChecks(w http.ResponseWriter, r *http.Request) {
	var passed []string
	var failed []error
	json.NewEncoder(w).Encode(buildPassFailResp(CheckVM(passed, failed)))
}

func runPGChecks(w http.ResponseWriter, r *http.Request) {
	node, err := flypg.NewNode()
	if err != nil {
		panic(err)
	}
	var passed []string
	var failed []error

	ctx, cancel := context.WithTimeout(context.TODO(), (time.Second * 10))
	resp := buildPassFailResp(CheckPostgreSQL(ctx, node, passed, failed))
	cancel()

	json.NewEncoder(w).Encode(resp)
}

func runRoleCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("Checking PG Role")

	node, err := flypg.NewNode()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), (time.Second * 10))
	role, err := PostgreSQLRole(ctx, node)
	cancel()

	resp := role
	if err != nil {
		resp += fmt.Sprintf(": %s", err.Error())
	}

	json.NewEncoder(w).Encode(resp)
}

func buildPassFailResp(passed []string, failed []error) string {
	var resp []string
	for _, v := range failed {
		resp = append(resp, fmt.Sprintf("[✗] %s", v))
	}
	for _, v := range passed {
		resp = append(resp, fmt.Sprintf("[✓] %s", v))
	}

	return strings.Join(resp, "\n")
}
