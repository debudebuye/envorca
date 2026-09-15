//go:build windows

package system

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

// memoryStatusEx mirrors the MEMORYSTATUSEX structure returned by
// GlobalMemoryStatusEx. Only the fields needed by Envorca are populated.
type memoryStatusEx struct {
	Length                     uint32
	MemoryLoad                 uint32
	TotalPhysical              uint64
	AvailablePhysical          uint64
	TotalPageFile              uint64
	AvailablePageFile          uint64
	TotalVirtual               uint64
	AvailableVirtual           uint64
	AvailableExtendedVirtual   uint64
}

func memory() (Resources, error) {
	var st memoryStatusEx
	st.Length = uint32(unsafe.Sizeof(st))
	proc := kernel32.NewProc("GlobalMemoryStatusEx")
	r1, _, e1 := proc.Call(uintptr(unsafe.Pointer(&st)))
	if r1 == 0 {
		return Resources{}, e1
	}
	return Resources{
		TotalMemoryBytes:     st.TotalPhysical,
		AvailableMemoryBytes: st.AvailablePhysical,
	}, nil
}