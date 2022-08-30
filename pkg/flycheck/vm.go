package flycheck

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/check"
)

// CheckVM for system / disk checks
func CheckVM(checks *check.CheckSuite) *check.CheckSuite {

	checks.AddCheck("checkDisk", func() (string, error) {
		return checkDisk("/data/")
	})

	checks.AddCheck("checkLoad", func() (string, error) {
		return checkLoad()
	})

	pressureNames := []string{"memory", "cpu", "io"}
	for _, n := range pressureNames {
		name := n
		checks.AddCheck(name, func() (string, error) {
			return checkPressure(name)
		})
	}

	return checks
}

func checkPressure(name string) (string, error) {
	var avg10, avg60, avg300, counter float64
	//var rest string
	raw, err := ioutil.ReadFile("/proc/pressure/" + name)
	if err != nil {
		return "", err
	}

	_, err = fmt.Sscanf(
		string(raw),
		"some avg10=%f avg60=%f avg300=%f total=%f",
		&avg10, &avg60, &avg300, &counter,
	)
	if err != nil {
		return "", err
	}

	avg10Dur, err := pressureToDuration(avg10, 10.0)
	if err != nil {
		return "", err
	}
	avg60Dur, err := pressureToDuration(avg60, 60.0)
	if err != nil {
		return "", err
	}

	avg300Dur, err := pressureToDuration(avg300, 300.0)
	if err != nil {
		return "", err
	}

	// Trigger failure if pressure exceeds 10 percent.
	if avg10 > 10 {
		return "", fmt.Errorf("system spent %s of the last 10 seconds waiting on %s", check.RoundDuration(avg10Dur, 2), name)
	}

	if avg60 > 10 {
		return "", fmt.Errorf("system spent %s of the last 60 seconds waiting on %s", check.RoundDuration(avg60Dur, 2), name)
	}

	if avg300 > 10 {
		return "", fmt.Errorf("system spent %s of the last 300 seconds waiting on %s", check.RoundDuration(avg300Dur, 2), name)
	}

	return fmt.Sprintf("system spent %s of the last 60s waiting on %s", check.RoundDuration(avg60Dur, 2), name), nil
}

func checkLoad() (string, error) {
	var loadAverage1, loadAverage5, loadAverage10 float64
	var runningProcesses, totalProcesses, lastProcessID int
	raw, err := ioutil.ReadFile("/proc/loadavg")

	if err != nil {
		return "", err
	}

	cpus := float64(runtime.NumCPU())
	_, err = fmt.Sscanf(string(raw), "%f %f %f %d/%d %d",
		&loadAverage1, &loadAverage5, &loadAverage10,
		&runningProcesses, &totalProcesses,
		&lastProcessID)
	if err != nil {
		return "", err
	}

	if loadAverage1/cpus > 10 {
		return "", fmt.Errorf("1 minute load average is very high: %.2f", loadAverage1)
	}
	if loadAverage5/cpus > 4 {
		return "", fmt.Errorf("5 minute load average is high: %.2f", loadAverage5)
	}
	if loadAverage10/cpus > 2 {
		return "", fmt.Errorf("10 minute load average is high: %.2f", loadAverage10)
	}

	return fmt.Sprintf("load averages: %.2f %.2f %.2f", loadAverage10, loadAverage5, loadAverage1), nil
}

func checkDisk(dir string) (string, error) {
	// Available blocks * size per block = available space in bytes
	size, available, err := diskUsage(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %s", dir, err)
	}

	pct := float64(available) / float64(size)
	msg := fmt.Sprintf("%s (%.1f%%) free space on %s", dataSize(available), pct*100, dir)

	if pct < 0.1 {
		return "", errors.New(msg)
	}

	return msg, nil
}

func diskUsage(dir string) (size uint64, available uint64, err error) {
	var stat syscall.Statfs_t

	err = syscall.Statfs(dir, &stat)

	if err != nil {
		return 0, 0, fmt.Errorf("%s: %s", dir, err)
	}

	size = stat.Blocks * uint64(stat.Bsize)
	available = stat.Bavail * uint64(stat.Bsize)
	return size, available, nil

}

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}
func dataSize(size uint64) string {
	var suffixes [5]string
	suffixes[0] = "B"
	suffixes[1] = "KB"
	suffixes[2] = "MB"
	suffixes[3] = "GB"
	suffixes[4] = "TB"

	base := math.Log(float64(size)) / math.Log(1024)
	getSize := round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	getSuffix := suffixes[int(math.Floor(base))]
	return fmt.Sprint(strconv.FormatFloat(getSize, 'f', -1, 64) + " " + string(getSuffix))
}

func pressureToDuration(pressure float64, base float64) (time.Duration, error) {
	seconds := base * (pressure / 100)
	secondsStr := fmt.Sprintf("%fs", seconds)
	return time.ParseDuration(secondsStr)
}
