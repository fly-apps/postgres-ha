package flypg

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/fly-examples/postgres-ha/pkg/flypg/stolon"
	"github.com/pkg/errors"
)

var ErrClusterNotInitialized = errors.New("cluster not initialized")

func keeperUID(privateIP net.IP) string {
	if data, err := os.ReadFile("/data/keeperstate"); err == nil {
		keeperstate := map[string]string{}
		if err := json.Unmarshal(data, &keeperstate); err == nil {
			if uid, ok := keeperstate["UID"]; ok && uid != "" {
				return uid
			}
		}
	}

	if privateIP == nil || privateIP.IsUnspecified() || privateIP.IsLoopback() {
		return "local"
	}

	parts := strings.Split(privateIP.String(), ":")
	return strings.Join(parts[4:], "")
}

func (n *Node) GetStolonClusterData() (s stolon.ClusterData, err error) {
	cmd := exec.Command("stolonctl", "clusterdata", "read")
	cmd.Env = append(os.Environ(),
		"STOLONCTL_CLUSTER_NAME="+n.AppName,
		"STOLONCTL_STORE_BACKEND="+"consul",
		"STOLONCTL_STORE_URL="+n.ConsulURL.String(),
		"STOLONCTL_STORE_NODE="+n.StoreNode,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.HasPrefix(string(output), "nil cluster data: ") {
			return s, ErrClusterNotInitialized
		}
		return s, errors.Wrap(err, "error checking stolon status: "+string(output))
	}

	err = json.Unmarshal(output, &s)

	return
}
