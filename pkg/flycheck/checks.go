package flycheck

import (
	"context"
	"encoding/json"
	"fmt"
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

	passed, failed = CheckVM(passed, failed)
	resp := buildPassFailResp(passed, failed)
	if len(failed) > 0 {
		handleError(w, fmt.Errorf(resp))
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func runPGChecks(w http.ResponseWriter, r *http.Request) {
	node, err := flypg.NewNode()
	if err != nil {
		handleError(w, err)
		return
	}

	var passed []string
	var failed []error

	ctx, cancel := context.WithTimeout(context.TODO(), (time.Second * 10))
	defer cancel()

	pgTime := time.Now()
	passed, failed = CheckPostgreSQL(ctx, node, passed, failed)

	resp := buildPassFailResp(passed, failed)
	if len(failed) > 0 {
		handleError(w, fmt.Errorf(resp))
		return
	}

	fmt.Printf("PG checks completed in: %v\n", time.Since(pgTime))

	json.NewEncoder(w).Encode(resp)
}

func runRoleCheck(w http.ResponseWriter, r *http.Request) {
	roleTime := time.Now()

	node, err := flypg.NewNode()
	if err != nil {
		handleError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), (time.Second * 10))
	defer cancel()
	role, err := PostgreSQLRole(ctx, node)
	if err != nil {
		handleError(w, err)
		return
	}

	fmt.Printf("Role check completed in: %v\n", time.Since(roleTime))

	json.NewEncoder(w).Encode(role)
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

func handleError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(err.Error())
}
