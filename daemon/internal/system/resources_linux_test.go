//go:build !windows

package system

import "testing"

func TestMeminfo(t *testing.T) {
	res, err := Memory()
	if err != nil {
		t.Skipf("no /proc/meminfo on this platform: %v", err)
	}
	if res.TotalMemoryBytes == 0 {
		t.Error("total memory = 0")
	}
	_ = res.MemoryNote
}
