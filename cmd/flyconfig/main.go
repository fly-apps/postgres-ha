package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/shirou/gopsutil/v3/mem"
)

type config struct {
	InitMode             string            `json:"initMode"`
	PGParameters         map[string]string `json:"pgParameters"`
	MaxStandbysPerSender int               `json:"maxStandbysPerSender"`
}

func main() {
	filename := "/fly/cluster-spec.json"

	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

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
		log.Fatalln("error loading cluster spec", err)
	}

	mem, err := memTotal()
	if err != nil {
		log.Fatalln("error fetching total system memory:", err)
	}

	fmt.Printf("system memory: %dmb vcpu count: %d\n", mem, runtime.NumCPU())

	workMem := max(4, (mem / 64))
	maintenanceWorkMem := max(64, (mem / 20))

	cfg = config{
		InitMode:             "new",
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
		log.Fatalln("error writing cluster-spec.json:", err)
	}

	fmt.Println("generated new config")
}

func readConfig(filename string) (cfg config, err error) {
	var data []byte
	data, err = os.ReadFile(filename)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &cfg)
	return
}

func writeConfig(filename string, cfg config) error {
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

func writeJson(w io.Writer, cfg config) error {
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
