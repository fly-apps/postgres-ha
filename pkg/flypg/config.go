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

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/mem"
)

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

func InitConfig(filename string) error {
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
		return errors.Wrap(err, "error loading cluster spec")
	}

	mem, err := memTotal()
	if err != nil {
		return errors.Wrap(err, "error fetching total system memory")
	}

	fmt.Printf("system memory: %dmb vcpu count: %d\n", mem, runtime.NumCPU())

	workMem := max(4, (mem / 64))
	maintenanceWorkMem := max(64, (mem / 20))

	initMode := "new"
	existingConfig := map[string]string{}

	// Don't blow away postgres directory if it exists present.
	if _, err := os.Stat("/data/postgres"); err == nil {
		initMode = "existing"

		// if the keeperstate file does not exist, seed it.
		_, err = os.Stat("/data/keeperstate")
		if os.IsNotExist(err) && initMode == "existing" {
			data := []byte("{\"UID\":\"ab805b922\",\"ClusterUID\":\"889e599a\"}")
			if err = ioutil.WriteFile("/data/keeperstate", data, 0644); err != nil {
				return err
			}
		}

		var keeperState KeeperState
		data, err := os.ReadFile("/data/keeperstate")
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, &keeperState)
		if err != nil {
			return err
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
		},
	}

	writeJson(os.Stdout, cfg)

	if err := writeConfig(filename, cfg); err != nil {
		return errors.Wrap(err, "error writing cluster-spec.json")
	}

	fmt.Println("generated new config")

	return nil
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
