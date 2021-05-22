package flypg

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

type StolonConfig struct {
	KeeperUID    string
	PrivateIP    net.IP
	ClusterName  string
	StoreBackend string
	StoreURL     string
	StoreNode    string
}

// func GetStolonConfig() (*StolonConfig, error) {
// 	cfg := &StolonConfig{
// 		ClusterName:  FlyAppName(),
// 		StoreBackend: "consul",
// 	}

// 	privateIP, err := privnet.PrivateIPv6()
// 	if err != nil {
// 		return nil, errors.Wrap(err, "error getting private ip")
// 	}
// 	cfg.PrivateIP = privateIP
// 	cfg.KeeperUID = keeperUID(privateIP)

// 	rawConsulURL := os.Getenv("FLY_CONSUL_URL")
// 	if rawConsulURL == "" {
// 		rawConsulURL = os.Getenv("CONSUL_URL")
// 	}
// 	if rawConsulURL == "" {
// 		return nil, errors.New("FLY_CONSUL_URL or CONSUL_URL are required")
// 	}
// 	consulURL, err := url.Parse(rawConsulURL)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "error parsing consul url")
// 	}
// 	cfg.StoreURL = consulURL.String()
// 	cfg.StoreNode = strings.TrimPrefix(path.Join(consulURL.Path, cfg.KeeperUID), "/")

// 	return cfg, nil
// }

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

// func FlyAppName() string {
// 	appName := os.Getenv("FLY_APP_NAME")
// 	if appName == "" {
// 		appName = "local"
// 	}
// 	return appName
// }

type StolonStatus struct {
	Sentinels []struct {
		UID    string
		Leader bool
	}
	Proxies []struct {
		UID        string
		Generation int
	}
	Keepers []struct {
		UID                 string
		ListenAddress       string `json:"listen_address"`
		Healthy             bool
		PGHealthy           bool `json:"pg_healthy"`
		PGWantedGeneration  int  `json:"pg_wanted_generation"`
		PGCurrentGeneration int  `json:"pg_current_generation"`
	}
	Cluster struct {
		Available       bool
		MasterKeeperUID string `json:"master_keeper_uid"`
		MasterDBUID     string `json:"master_db_uid"`
	}
}

func (n *Node) GetStolonStatus() (s StolonStatus, err error) {
	cmd := exec.Command("stolonctl", "status", "-f", "json")
	cmd.Env = append(os.Environ(),
		"STOLONCTL_CLUSTER_NAME="+n.AppName,
		"STOLONCTL_STORE_BACKEND="+"consul",
		"STOLONCTL_STORE_URL="+n.ConsulURL.String(),
		"STOLONCTL_STORE_NODE="+n.StoreNode,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return s, errors.Wrap(err, "error checking stolon status: "+string(output))
	}

	err = json.Unmarshal(output, &s)

	return
}
