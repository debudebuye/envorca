//go:build windows

package diagnosis

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	envorkav1 "envorka.dev/envorka/api/gen/go/envorka/v1"
	"envorka.dev/envorka/daemon/internal/health"
	"envorka.dev/envorka/daemon/internal/wsl"
)

func windowsProbe() health.Probe {
	return func(context.Context) envorkav1.ComponentStatus {
		v, err := rtlGetVersion()
		if err != nil {
			return envorkav1.ComponentStatus{
				Status:  envorkav1.Status_CRITICAL,
				Summary: "could not read Windows version",
				Reason:  "Envorka is a Windows application; a readable OS version is required.",
			}
		}
		return envorkav1.ComponentStatus{
			Status:  envorkav1.Status_HEALTHY,
			Summary: fmt.Sprintf("Windows %d.%d (build %d)", v.Major, v.Minor, v.Build),
		}
	}
}

// virtualizationProbe reports whether the virtualization platform works. For
// V1 it treats a successful WSL2 query as the real signal; dedicated
// Hyper-V/VirtualMachinePlatform checks land with installer work.
func virtualizationProbe() health.Probe {
	return func(ctx context.Context) envorkav1.ComponentStatus {
		// `wsl --status` is the fastest reliable proof that the
		// VirtualMachinePlatform is present and functioning.
		runner := wsl.DefaultRunner()
		if _, err := runner(ctx, "--status"); err != nil {
			return envorkav1.ComponentStatus{
				Status:         envorkav1.Status_CRITICAL,
				Summary:        "CPU virtualization or Virtual Machine Platform unavailable",
				Reason:         "WSL2 cannot run without virtualization support enabled in firmware and the VirtualMachinePlatform Windows feature.",
				Recommendation: "Enable virtualization in BIOS/UEFI and enable the 'Virtual Machine Platform' and 'Windows Subsystem for Linux' features.",
				SafeToFix:      false,
			}
		}
		return envorkav1.ComponentStatus{
			Status:  envorkav1.Status_HEALTHY,
			Summary: "CPU virtualization and Virtual Machine Platform available",
		}
	}
}

type osVersion struct {
	Major, Minor, Build uint32
}

func rtlGetVersion() (*osVersion, error) {
	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetVersion")
	var out struct {
		OSVersionInfoSize uint32
		Major             uint32
		Minor             uint32
		Build             uint32
		PlatformID        uint32
		CSDVersion        [128]uint16
	}
	out.OSVersionInfoSize = uint32(unsafe.Sizeof(out))
	r1, _, e1 := proc.Call(uintptr(unsafe.Pointer(&out)))
	if r1 != 0 {
		return nil, e1
	}
	return &osVersion{Major: out.Major, Minor: out.Minor, Build: out.Build}, nil
}
