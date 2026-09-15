//go:build windows

package diagnosis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/daemon/internal/health"
	"envorca.dev/envorca/daemon/internal/wsl"
)

func windowsProbe() health.Probe {
	return func(context.Context) envorcav1.ComponentStatus {
		v, err := rtlGetVersion()
		if err != nil {
			return envorcav1.ComponentStatus{
				Status:  envorcav1.Status_CRITICAL,
				Summary: "could not read Windows version",
				Reason:  "Envorca is a Windows application; a readable OS version is required.",
			}
		}
		return envorcav1.ComponentStatus{
			Status:  envorcav1.Status_HEALTHY,
			Summary: fmt.Sprintf("Windows %d.%d (build %d)", v.Major, v.Minor, v.Build),
		}
	}
}

// virtualizationProbe reports whether the virtualization platform works. A
// successful `wsl --status` is the decisive proof that the Virtual Machine
// Platform is present and functioning, so the healthy path stays cheap. When
// WSL fails to answer, dedicated checks (CPU firmware virtualization and the
// Hyper-V/vmcompute and WSL services) pinpoint what is actually broken.
func virtualizationProbe() health.Probe {
	return func(ctx context.Context) envorcav1.ComponentStatus {
		runner := wsl.DefaultRunner()
		if _, err := runner(ctx, "--status"); err == nil {
			return envorcav1.ComponentStatus{
				Status:  envorcav1.Status_HEALTHY,
				Summary: "CPU virtualization and Virtual Machine Platform available",
			}
		}
		return diagnoseVirtualization(ctx)
	}
}

// diagnoseVirtualization explains why WSL2 cannot run. Checks run from
// cheapest to most expensive and stop at the first definitive cause.
func diagnoseVirtualization(ctx context.Context) envorcav1.ComponentStatus {
	crit := func(summary, reason, rec string) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{
			Status:         envorcav1.Status_CRITICAL,
			Summary:        summary,
			Reason:         reason,
			Recommendation: rec,
			SafeToFix:      false,
		}
	}

	if cpu, slat := virtualizationFirmware(); !cpu {
		return crit("CPU virtualization is disabled",
			"WSL2 cannot run while virtualization is turned off in the processor or firmware.",
			"Enable Intel VT-x or AMD-V (virtualization) in BIOS/UEFI and/or in Windows' 'Virtual Machine Platform' settings.")
	} else if !slat {
		return crit("CPU second-level address translation unavailable",
			"WSL2 needs SLAT/EPT support to run a hypervisor.",
			"Enable the virtualization technology for the platform in BIOS/UEFI and confirm CPU SLAT support.")
	}

	if installed, running, err := serviceState(ctx, "vmcompute"); err != nil {
		// Fall through to the generic answer if the probe itself failed.
		_ = installed
		_ = running
	} else if !installed {
		return crit("Virtual Machine Platform is not installed",
			"WSL2 depends on the 'Virtual Machine Platform' optional feature; without it no Hyper-V VM can start.",
			"Enable it: run `wsl --install` or `dism /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart` from an elevated shell, then reboot.")
	} else if !running {
		return crit("Hyper-V host service is stopped",
			"The vmcompute service (Virtual Machine Platform) is installed but not running.",
			"Start the service: run `net start vmcompute` from an elevated shell, or restart Windows.")
	}

	if installed, running, err := serviceState(ctx, "WslService"); err == nil {
		if installed && !running {
			return crit("WSL service is stopped",
				"The WSL service is installed but not running.",
				"Start it: run `net start WslService` from an elevated shell, or run `wsl --shutdown` and start again.")
		}
	}

	return crit("CPU virtualization or Virtual Machine Platform unavailable",
		"WSL2 cannot run without virtualization support enabled in firmware and the VirtualMachinePlatform Windows feature.",
		"Enable virtualization in BIOS/UEFI and enable the 'Virtual Machine Platform' and 'Windows Subsystem for Linux' features.",
	)
}

// Processor feature flags (winnt.h).
const (
	pfSecondLevelAddressTranslation = 20
	pfVirtFirmwareEnabled           = 23
)

// virtualizationFirmware reports whether the CPU exposes the two pieces of
// hardware virtualization WSL2 needs, as seen by the running system.
func virtualizationFirmware() (virtEnabled, slat bool) {
	p := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessorFeaturePresent")
	r1, _, _ := p.Call(uintptr(pfVirtFirmwareEnabled))
	virtEnabled = r1 != 0
	r1, _, _ = p.Call(uintptr(pfSecondLevelAddressTranslation))
	slat = r1 != 0
	return virtEnabled, slat
}

const scServiceNotInstalled = 1060

// serviceState reports an installed Windows service's state. A service that
// is not installed returns (false, false, nil); other failures return an
// error so callers can fall back to a generic diagnosis.
func serviceState(ctx context.Context, name string) (installed, running bool, err error) {
	out, err := exec.CommandContext(ctx, "sc.exe", "query", name).CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == scServiceNotInstalled {
			return false, false, nil
		}
		return false, false, fmt.Errorf("sc query %s: %w", name, err)
	}
	return true, scQueryRunning(out), nil
}

// scQueryRunning scans `sc query` output for the running state marker.
func scQueryRunning(body []byte) bool {
	return bytes.Contains(body, []byte("RUNNING"))
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