package stolon

import (
	"encoding/json"
	"os/exec"
)

// Ctl runs stolonctl give an bunch of args and some env variables
func Ctl(args []string, env []string) ([]byte, error) {
	subProcess := exec.Command("stolonctl", args...)
	subProcess.Env = append(subProcess.Env, env...)

	return subProcess.CombinedOutput()
}

func Failkeeper(currentMaster string, env []string) ([]byte, error) {
	var cmd, args = "failkeeper", currentMaster

	return Ctl([]string{cmd, args}, env)
}

func FetchClusterData(env []string) (*ClusterData, error) {
	args := []string{"clusterdata", "read"}
	result, err := Ctl(args, env)
	if err != nil {
		return nil, err
	}
	data := new(ClusterData)
	if err := json.Unmarshal(result, data); err != nil {
		return nil, err
	}

	return data, nil
}
