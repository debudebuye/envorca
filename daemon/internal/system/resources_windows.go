//go:build windows

package system

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

func memory() (Resources, error) {
	var totalKB uint64
	proc := kernel32.NewProc("GetPhysicallyInstalledSystemMemory")
	r1, _, e1 := proc.Call(uintptr(unsafe.Pointer(&totalKB)))
	if r1 == 0 {
		return Resources{}, e1
	}
	return Resources{
		TotalMemoryBytes: totalKB * 1024,
		MemoryNote:       "available memory not reported on Windows in V1",
	}, nil
}
