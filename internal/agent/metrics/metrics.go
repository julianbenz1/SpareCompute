package metrics

import (
	"bufio"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

type ReserveConfig struct {
	CPUPercent int
	RAMMB      int64
	DiskMB     int64
}

func Collect(nodeID string, reserve ReserveConfig, labels map[string]string) (common.NodeRegisterRequest, common.NodeHeartbeatRequest, error) {
	totalRAM, availRAM, err := readMemInfoMB()
	if err != nil {
		return common.NodeRegisterRequest{}, common.NodeHeartbeatRequest{}, err
	}
	totalDisk, freeDisk, err := readDiskMB("/")
	if err != nil {
		return common.NodeRegisterRequest{}, common.NodeHeartbeatRequest{}, err
	}
	cpuCores := runtime.NumCPU()
	load1 := readLoadAvg1()
	cpuUsage := int(math.Min(100, (load1/float64(cpuCores))*100))

	shareableCPU := maxInt(0, 100-reserve.CPUPercent-cpuUsage)
	shareableRAM := maxInt64(0, availRAM-reserve.RAMMB)
	shareableDisk := maxInt64(0, freeDisk-reserve.DiskMB)

	register := common.NodeRegisterRequest{
		ID:                 nodeID,
		Name:               nodeID,
		Labels:             labels,
		TotalCPUCores:      cpuCores,
		TotalRAMMB:         totalRAM,
		TotalDiskMB:        totalDisk,
		ReservedCPUPercent: reserve.CPUPercent,
		ReservedRAMMB:      reserve.RAMMB,
		ReservedDiskMB:     reserve.DiskMB,
	}

	heartbeat := common.NodeHeartbeatRequest{
		NodeID:         nodeID,
		CPUUsagePct:    cpuUsage,
		LoadAvg1:       load1,
		AvailableRAMMB: availRAM,
		FreeDiskMB:     freeDisk,
		ShareableCPU:   shareableCPU,
		ShareableRAMMB: shareableRAM,
		ShareableDisk:  shareableDisk,
		Status:         common.NodeOnline,
		Maintenance:    false,
	}
	return register, heartbeat, nil
}

func readMemInfoMB() (totalMB int64, availableMB int64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			totalMB = parseKBLineToMB(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			availableMB = parseKBLineToMB(line)
		}
	}
	return totalMB, availableMB, sc.Err()
}

func readLoadAvg1() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(data))
	if len(parts) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	return v
}

func readDiskMB(path string) (totalMB int64, freeMB int64, err error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return 0, 0, err
	}
	total := int64(s.Blocks) * int64(s.Bsize)
	free := int64(s.Bavail) * int64(s.Bsize)
	return total / (1024 * 1024), free / (1024 * 1024), nil
}

func parseKBLineToMB(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return v / 1024
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

