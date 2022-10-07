package main

import (
	"fmt"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flypg/stolon"
	"github.com/fly-examples/postgres-ha/pkg/util"
)

func main() {
	env, err := util.BuildEnv()
	if err != nil {
		util.WriteError(err)
	}

	data, err := stolon.FetchClusterData(env)
	if err != nil {
		util.WriteError(err)
	}

	// Set this so we can compare it later.
	currentMasterUID := masterKeeperUID(data)

	// Discover keepers that are eligible for promotion.
	eligibleCount := 0
	for _, keeper := range data.Keepers {
		if keeper.Status.Healthy && keeper.Status.CanBeMaster && keeper.UID != currentMasterUID {
			fmt.Printf("Keeper %s is eligible!  Master is %s\n", keeper.UID, currentMasterUID)
			eligibleCount++
		}
	}

	if eligibleCount == 0 {
		util.WriteError(fmt.Errorf("No eligible keepers available to accommodate failover"))
	}

	_, err = stolon.Failkeeper(currentMasterUID, env)
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
			data, err := stolon.FetchClusterData(env)
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

func masterKeeperUID(data *stolon.ClusterData) string {
	db := data.DBs[data.Cluster.Status.Master]
	return db.Spec.KeeperUID
}
