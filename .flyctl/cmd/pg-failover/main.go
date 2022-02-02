package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg/stolon"
	"github.com/fly-examples/postgres-ha/pkg/util"
)

func main() {
	env, err := util.BuildEnv()
	if err != nil {
		util.WriteError(err)
	}

	data, err := clusterData(env)
	if err != nil {
		util.WriteError(err)
	}

	// Set this so we can compare it later.
	currentMasterUID := masterKeeperUID(data)

	// Discover keepers that are eligible for promotion.
	eligibleCount := 0
	for _, keeper := range data.Keepers {
		if keeper.Status.Healthy && keeper.Status.CanBeMaster != nil && keeper.UID != currentMasterUID {
			fmt.Printf("Keeper %s is eligible!  Master is %s\n", keeper.UID, currentMasterUID)
			eligibleCount++
		}
	}

	if eligibleCount == 0 {
		util.WriteError(fmt.Errorf("No eligible keepers available to accommodate failover"))
	}

	_, err = stolonCtl([]string{"failkeeper", currentMasterUID}, env)
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

			if currentMasterUID != masterKeeperUID(data) {
				util.WriteOutput("failover completed successfully", "")
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
	subProcess := exec.Command("stolonctl", args...)
	subProcess.Env = append(subProcess.Env, env...)

	return subProcess.CombinedOutput()
}

func masterKeeperUID(data *stolon.ClusterData) string {
	db := data.DBs[data.Cluster.Status.Master]
	return db.Spec.KeeperUID
}
