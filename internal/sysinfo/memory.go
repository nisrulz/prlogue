package sysinfo

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type RAMInfo struct {
	TotalRAMGB     float64
	OSReserveGB    float64
	AvailableRAMGB float64
}

func DetectRAM(osReserveGB float64) (*RAMInfo, error) {
	totalGB, err := totalRAMGB()
	if err != nil {
		return nil, fmt.Errorf("detect RAM: %w", err)
	}
	if osReserveGB <= 0 {
		osReserveGB = defaultOSReserve()
	}
	avail := totalGB - osReserveGB
	if avail < 1 {
		avail = 1
	}
	return &RAMInfo{
		TotalRAMGB:     totalGB,
		OSReserveGB:    osReserveGB,
		AvailableRAMGB: avail,
	}, nil
}

func defaultOSReserve() float64 {
	switch runtime.GOOS {
	case "darwin":
		return 5.0
	case "linux":
		return 4.0
	case "windows":
		return 6.0
	default:
		return 5.0
	}
}

func totalRAMGB() (float64, error) {
	switch runtime.GOOS {
	case "darwin":
		return darwinTotalRAM()
	case "linux":
		return linuxTotalRAM()
	case "windows":
		return windowsTotalRAM()
	default:
		return 0, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func darwinTotalRAM() (float64, error) {
	out, err := exec.Command("sysctl", "hw.memsize").Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ": ")
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected sysctl output: %s", out)
	}
	bytes, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse memsize: %w", err)
	}
	return float64(bytes) / (1024 * 1024 * 1024), nil
}

func linuxTotalRAM() (float64, error) {
	out, err := exec.Command("free", "-b").Output()
	if err != nil {
		return 0, fmt.Errorf("free -b: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected free output")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return 0, fmt.Errorf("unexpected free mem line")
	}
	bytes, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse total memory: %w", err)
	}
	return float64(bytes) / (1024 * 1024 * 1024), nil
}

func windowsTotalRAM() (float64, error) {
	out, err := exec.Command("wmic", "MemoryChip", "get", "Capacity").Output()
	if err != nil {
		return 0, fmt.Errorf("wmic: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	var total uint64
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		val, err := strconv.ParseUint(line, 10, 64)
		if err != nil {
			continue
		}
		total += val
	}
	if total == 0 {
		return 0, fmt.Errorf("could not detect RAM via wmic")
	}
	return float64(total) / (1024 * 1024 * 1024), nil
}

// CalcMaxContext estimates a context window that fits in available RAM using
// a heuristic based on model weights and KV cache growth. Defaults approximate
// a 7B model (~4.3 GB weights).
func CalcMaxContext(availRAMGB float64, modelMaxContext int, modelSizeGB float64) int {
	if modelSizeGB <= 0 {
		modelSizeGB = 4.28
	}
	if modelMaxContext > 0 {
		baseMem := 2.0 + 0.7*modelSizeGB
		gbPer128K := 0.2 + 0.65*modelSizeGB
		maxTokensForRAM := int((availRAMGB - baseMem) / gbPer128K * 131072)
		if maxTokensForRAM < 4096 {
			maxTokensForRAM = 4096
		}
		if maxTokensForRAM > modelMaxContext {
			maxTokensForRAM = modelMaxContext
		}
		return roundDownContext(maxTokensForRAM)
	}

	return 131072 // safe fallback
}

func roundDownContext(ctx int) int {
	// Round down to nearest common value
	thresholds := []int{1000000, 512000, 262144, 131072, 65536, 32768, 16384, 8192, 4096}
	for _, t := range thresholds {
		if ctx >= t {
			return t
		}
	}
	return 4096
}
