package commands

import (
	"encoding/json"
	"fmt"
	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"os/exec"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg/admin"
	"github.com/fly-examples/postgres-ha/pkg/flypg/stolon"
	"github.com/fly-examples/postgres-ha/pkg/render"
	"github.com/fly-examples/postgres-ha/pkg/util"
)

func handleFailoverTrigger(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	env, err := util.BuildEnv()
	if err != nil {
		render.Err(w, err)
		return
	}

	data, err := stolon.FetchClusterData(env)
	if err != nil {
		render.Err(w, err)
		return
	}

	// Set this so we can compare it later.
	currentMasterUID := masterKeeperUID(data)

	// Discover keepers that are eligible for promotion.
	retries := 5

	eligibleCount := 0
	for retries > 0 {
		for _, keeper := range data.Keepers {
			if keeper.Status.Healthy && keeper.Status.CanBeMaster && keeper.UID != currentMasterUID {
				fmt.Printf("Keeper %s is eligible!  Master is %s\n", keeper.UID, currentMasterUID)
				eligibleCount++
			}
		}
		if eligibleCount == 0 {
			time.Sleep(10 * time.Second)
			retries = retries - 1
		}
	}

	if eligibleCount == 0 {
		err := fmt.Errorf("no eligible keepers available to accommodate failover")
		render.Err(w, err)
		return
	}

	_, err = stolon.Failkeeper(currentMasterUID, env)
	if err != nil {
		util.WriteError(err)
	}

	// Verify failover
	timeout := time.After(10 * time.Second)

	ticker := time.NewTicker(1 * time.Second)

	for {
		select {

		case <-timeout:
			render.Err(w, fmt.Errorf("timed out verifying failover"))
		case <-ticker.C:
			data, err := stolon.FetchClusterData(env)
			if err != nil {
				render.Err(w, fmt.Errorf("failed to verify failover with error: %w", err))
			}

			if currentMasterUID != masterKeeperUID(data) {
				res := failOverResponse{"failover completed successfully"}
				render.JSON(w, res, http.StatusOK)
				return
			}
		case <-ctx.Done():
			render.Err(w, ctx.Err())
			return
		}

	}
}

func handleRestart(w http.ResponseWriter, r *http.Request) {

	args := []string{"stolon", "pg_ctl", "-D", "/data/postgres", "restart"}

	cmd := exec.Command("gosu", args...)

	if err := cmd.Run(); err != nil {
		render.Err(w, err)
		return
	}

	if cmd.ProcessState.ExitCode() != 0 {
		err := fmt.Errorf(cmd.ProcessState.String())
		render.Err(w, err)
		return
	}

	res := &Response{Result: "Restart completed successfully"}

	render.JSON(w, res, http.StatusOK)
}

func handleRole(w http.ResponseWriter, r *http.Request) {
	conn, close, err := localConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	role, err := admin.ResolveRole(r.Context(), conn)
	if err != nil {
		render.Err(w, err)
		return
	}

	res := &Response{Result: role}

	render.JSON(w, res, http.StatusOK)
}

func handleViewSettings(w http.ResponseWriter, r *http.Request) {
	conn, close, err := proxyConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	in := []string{}

	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		render.Err(w, err)
		return
	}

	settings, err := admin.ResolveSettings(r.Context(), conn, in)
	if err != nil {
		render.Err(w, err)
		return
	}

	res := &Response{
		Result: settings,
	}
	render.JSON(w, res, http.StatusOK)
}

func handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	env, err := util.BuildEnv()
	if err != nil {
		render.Err(w, err)
		return
	}

	config, err := io.ReadAll(r.Body)

	defer r.Body.Close()

	if err != nil {
		err = fmt.Errorf("failed to read request body: %w", err)
		render.Err(w, err)
		return
	}

	if _, err := stolon.Ctl([]string{"update", "--patch", string(config)}, env); err != nil {
		render.Err(w, err)
		return
	}
	resp := &Response{Result: "Ok"}

	render.JSON(w, resp, http.StatusOK)
}

func handleReplicationStats(w http.ResponseWriter, r *http.Request) {
	conn, close, err := localConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	stats, err := admin.ResolveReplicationLag(r.Context(), conn)
	if err != nil {
		render.Err(w, err)
		return
	}

	resp := &Response{Result: stats}

	render.JSON(w, resp, http.StatusOK)
}

func handleStolonDBUid(w http.ResponseWriter, r *http.Request) {
	env, err := util.BuildEnv()
	if err != nil {
		render.Err(w, err)
	}

	data, err := stolon.FetchClusterData(env)
	if err != nil {
		render.Err(w, err)
	}

	node, err := flypg.NewNode()
	if err != nil {
		render.Err(w, err)
	}

	for _, db := range data.DBs {
		if db.Spec.KeeperUID == node.KeeperUID {
			resp := &Response{Result: db.UID}
			render.JSON(w, resp, http.StatusOK)
			return
		}
	}

	render.Err(w, errors.New("can't find db"))
}

func handleEnableReadonly(w http.ResponseWriter, r *http.Request) {
	conn, close, err := localConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	err = admin.SetReadonly(r.Context(), conn, true)
	if err != nil {
		render.Err(w, err)
	}

	args := []string{"root", "pkill", "haproxy"}

	cmd := exec.Command("gosu", args...)

	if err := cmd.Run(); err != nil {
		render.Err(w, err)
		return
	}

	if cmd.ProcessState.ExitCode() != 0 {
		err := fmt.Errorf(cmd.ProcessState.String())
		render.Err(w, err)
		return
	}

	resp := &Response{Result: true}

	render.JSON(w, resp, http.StatusOK)
}

func handleDisableReadonly(w http.ResponseWriter, r *http.Request) {
	conn, close, err := localConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	err = admin.SetReadonly(r.Context(), conn, false)
	if err != nil {
		render.Err(w, err)
	}

	args := []string{"root", "pkill", "haproxy"}

	cmd := exec.Command("gosu", args...)

	if err := cmd.Run(); err != nil {
		render.Err(w, err)
		return
	}

	if cmd.ProcessState.ExitCode() != 0 {
		err := fmt.Errorf(cmd.ProcessState.String())
		render.Err(w, err)
		return
	}

	resp := &Response{Result: true}

	render.JSON(w, resp, http.StatusOK)
}

func handleRestartHaproxy(w http.ResponseWriter, r *http.Request) {
	args := []string{"root", "pkill", "haproxy"}

	cmd := exec.Command("gosu", args...)

	if err := cmd.Run(); err != nil {
		render.Err(w, err)
		return
	}

	if cmd.ProcessState.ExitCode() != 0 {
		err := fmt.Errorf(cmd.ProcessState.String())
		render.Err(w, err)
		return
	}

	resp := &Response{Result: true}

	render.JSON(w, resp, http.StatusOK)
}
