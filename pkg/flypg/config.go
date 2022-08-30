package flypg

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/mem"
)

const InitModeNew = "new"
const InitModeExisting = "existing"

type Config struct {
	InitMode             string            `json:"initMode"`
	ExistingConfig       map[string]string `json:"existingConfig"`
	PGParameters         map[string]string `json:"pgParameters"`
	MaxStandbysPerSender int               `json:"maxStandbysPerSender"`
}

type KeeperState struct {
	UID        string `json:"UID"`
	ClusterUID string `json:"ClusterUID"`
}

func InitConfig(filename string) (*Config, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		log.Fatalln("error cleaning filename", err)
	}

	fmt.Println("cluster spec filename", filename)
	cfg, err := readConfig(filename)
	if err == nil {
		fmt.Println("cluster spec already exists")
		writeJson(os.Stdout, cfg)
		// return
	} else if !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "error loading cluster spec")
	}

	mem, err := memTotal()
	if err != nil {
		return nil, errors.Wrap(err, "error fetching total system memory")
	}

	fmt.Printf("system memory: %dmb vcpu count: %d\n", mem, runtime.NumCPU())

	workMem := max(4, (mem / 64))
	maintenanceWorkMem := max(64, (mem / 20))

	initMode := InitModeNew
	existingConfig := map[string]string{}

	// Don't blow away postgres directory if it exists.
	if _, err := os.Stat("/data/postgres"); err == nil {
		initMode = InitModeExisting

		_, err = os.Stat("/data/keeperstate")
		if os.IsNotExist(err) && initMode == InitModeExisting {
			// if the keeperstate file does not exist, seed it.
			// TODO - There is likely a better way to handle this, may take up to 2 minutes for Stolon
			// to re-register the cluster.
			data := []byte("{\"UID\":\"ab805b922\"}")
			if err = ioutil.WriteFile("/data/keeperstate", data, 0644); err != nil {
				return nil, err
			}
		}

		var keeperState KeeperState
		data, err := os.ReadFile("/data/keeperstate")
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(data, &keeperState)
		if err != nil {
			return nil, err
		}
		if keeperState.UID != "" {
			existingConfig["keeperUID"] = keeperState.UID
		}
	}

	cfg = Config{
		InitMode:             initMode,
		ExistingConfig:       existingConfig,
		MaxStandbysPerSender: 50,
		PGParameters: map[string]string{
			"random_page_cost":                "1.1",
			"effective_io_concurrency":        "200",
			"shared_buffers":                  fmt.Sprintf("%dMB", mem/4),
			"effective_cache_size":            fmt.Sprintf("%dMB", 3*mem/4),
			"maintenance_work_mem":            fmt.Sprintf("%dMB", maintenanceWorkMem),
			"work_mem":                        fmt.Sprintf("%dMB", workMem),
			"max_connections":                 "300",
			"max_worker_processes":            "8",
			"max_parallel_workers":            "8",
			"max_parallel_workers_per_gather": "2",
			"wal_compression":                 "on",
		},
	}

	if initMode == InitModeNew {
		var stat syscall.Statfs_t
		if err = syscall.Statfs("/data", &stat); err != nil {
			return nil, err
		}
		diskSizeBytes := stat.Blocks * uint64(stat.Bsize)

		// Set max_wal_size to 10% of disk capacity.
		maxWalBytes := (float64(diskSizeBytes) * 0.1)
		maxWalMb := maxWalBytes / (1024 * 1024)
		cfg.PGParameters["max_wal_size"] = fmt.Sprintf("%dMB", int(maxWalMb))

		// Set min_wal_size to 25% of max_wal_size
		minWalBytes := (maxWalBytes * 0.25)
		minWalMb := int(minWalBytes / (1024 * 1024))

		// Stolon hardcodes the segment size to 16mb and min_wal_size must be at least twice the size.
		if minWalMb < 32 {
			minWalMb = 32
		}
		cfg.PGParameters["min_wal_size"] = fmt.Sprintf("%dMB", int(minWalMb))

		versionStr := os.Getenv("PG_MAJOR")
		if versionStr != "" {
			version, err := strconv.Atoi(versionStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PG_MAJOR version: %v", err)
			}
			// Let the WAL manager handle WAL retention requirements for replication slots.
			if version >= 13 {
				cfg.PGParameters["wal_keep_size"] = "0"
			}
			if version == 12 {
				cfg.PGParameters["wal_keep_segments"] = "0"
			}
		}
	}

	writeJson(os.Stdout, cfg)

	if err := writeConfig(filename, cfg); err != nil {
		return nil, errors.Wrap(err, "error writing cluster-spec.json")
	}

	fmt.Println("generated new config")

	return &cfg, nil
}

func readConfig(filename string) (cfg Config, err error) {
	var data []byte
	data, err = os.ReadFile(filename)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &cfg)
	return
}

func writeConfig(filename string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return writeJson(f, cfg)
}

func writeJson(w io.Writer, cfg Config) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "    ")
	return e.Encode(cfg)
}

func max(n ...int64) (max int64) {
	for _, num := range n {
		if num > max {
			max = num
		}
	}
	return
}

func memTotal() (memoryMb int64, err error) {
	if raw := os.Getenv("FLY_VM_MEMORY_MB"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, err
		}
		memoryMb = parsed
	}

	if memoryMb == 0 {
		v, err := mem.VirtualMemory()
		if err != nil {
			return 0, err
		}
		memoryMb = int64(v.Total / 1024 / 1024)
	}

	return
}
