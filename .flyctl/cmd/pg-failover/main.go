package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/flypg/stolon"
	"github.com/fly-examples/postgres-ha/pkg/util"
)

func main() {
	node, err := flypg.NewNode()

	// Resolve environment
	pathToEnv := filepath.Join(node.DataDir, ".env")

	file, err := os.Open(pathToEnv)
	if err != nil {
		util.WriteError(err)
	}
	defer file.Close()

	env := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		env = append(env, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		util.WriteError(err)
	}

	// Pull cluster configuration
	args := []string{"clusterdata", "read"}
	subProcess := exec.Command("stolonctl", args...)
	subProcess.Env = append(subProcess.Env, env...)

	result, err := subProcess.CombinedOutput()
	if err != nil {
		util.WriteError(err)
	}

	var data stolon.ClusterData
	if err := json.Unmarshal(result, &data); err != nil {
		util.WriteError(err)
	}

	// Determine the number of keepers eligible for promotion.
	eligibleCount := 0
	for _, keeper := range data.Keepers {
		if keeper.Status.Healthy && keeper.Status.CanBeMaster != nil {
			eligibleCount++
		}
	}

	// TODO - Review this logic. The idea is that current master should be eligible
	// for master, so in order to achieve a failover there should be more than 1.
	if eligibleCount <= 1 {
		util.WriteError(fmt.Errorf("No eligible keepers available to accommodate failover"))
	}

	// Resolve current master
	db := data.DBs[data.Cluster.Status.Master]
	masterKeeper := db.Spec.KeeperUID

	// Perform failover
	failKeeperArgs := []string{"failkeeper", masterKeeper}
	failKeeperCmd := exec.Command("stolonctl", failKeeperArgs...)
	failKeeperCmd.Env = append(failKeeperCmd.Env, env...)
	_, err = failKeeperCmd.CombinedOutput()
	if err != nil {
		util.WriteError(err)
	}

	// Verify failove
	timeout := time.After(10 * time.Second)
	ticker := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			util.WriteError(fmt.Errorf("timed out verifying failover"))
		case <-ticker:
			args := []string{"clusterdata", "read"}
			subProcess := exec.Command("stolonctl", args...)
			subProcess.Env = append(subProcess.Env, env...)
			result, err := subProcess.CombinedOutput()
			if err != nil {
				util.WriteError(err)
			}

			var data stolon.ClusterData
			if err := json.Unmarshal(result, &data); err != nil {
				util.WriteError(err)
			}

			db := data.DBs[data.Cluster.Status.Master]
			newMasterKeeper := db.Spec.KeeperUID
			if newMasterKeeper != masterKeeper {
				util.WriteOutput("success")
				return
			}
		}
	}
}
