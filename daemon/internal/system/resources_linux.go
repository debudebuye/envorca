//go:build !windows

package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func memory() (Resources, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Resources{}, err
	}
	defer f.Close()
	var res Resources
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			res.TotalMemoryBytes = parseMemInfo(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			res.AvailableMemoryBytes = parseMemInfo(line)
		}
		if res.TotalMemoryBytes > 0 && res.AvailableMemoryBytes > 0 {
			break
		}
	}
	return res, sc.Err()
}

func parseMemInfo(line string) uint64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	kb, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return kb * 1024
}
