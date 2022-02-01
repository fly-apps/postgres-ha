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
	// Resolve environment
	env, err := buildEnv()
	if err != nil {
		util.WriteError(err)
	}

	// Pull cluster configuration
	data, err := clusterData(env)
	if err != nil {
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

	// Set this so we can compare it later.
	currentMaster := masterKeeperUID(data)

	// Perform failover
	failKeeperArgs := []string{"failkeeper", currentMaster}
	_, err = stolonCtl(failKeeperArgs, env)
	if err != nil {
		util.WriteError(err)
	}

	// Verify failover
	timeout := time.After(10 * time.Second)
	ticker := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			util.WriteError(fmt.Errorf("timed out verifying failover"))
		case <-ticker:
			data, err := clusterData(env)
			if err != nil {
				util.WriteError(fmt.Errorf("failed to verify failover with error: %w", err))
			}

			if currentMaster != masterKeeperUID(data) {
				util.WriteOutput("success")
				return
			}
		}
	}
}

func clusterData(env []string) (*stolon.ClusterData, error) {
	args := []string{"clusterdata", "read"}
	result, err := stolonCtl(args, env)
	if err != nil {
		return nil, err
	}
	var data stolon.ClusterData
	if err := json.Unmarshal(result, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func stolonCtl(args []string, env []string) ([]byte, error) {
	// args := []string{"clusterdata", "read"}
	subProcess := exec.Command("stolonctl", args...)
	subProcess.Env = append(subProcess.Env, env...)

	return subProcess.CombinedOutput()
}

func masterKeeperUID(data *stolon.ClusterData) string {
	db := data.DBs[data.Cluster.Status.Master]
	return db.Spec.KeeperUID
}

func buildEnv() ([]string, error) {
	node, err := flypg.NewNode()
	pathToEnv := filepath.Join(node.DataDir, ".env")

	file, err := os.Open(pathToEnv)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		env = append(env, scanner.Text())
	}

	return env, scanner.Err()
}
