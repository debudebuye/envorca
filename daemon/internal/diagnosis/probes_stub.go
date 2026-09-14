//go:build !windows

package diagnosis

import (
	"context"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/daemon/internal/health"
)

// On non-Windows development machines these checks are skipped and reported
// as UNKNOWN. The wsl probe uses wsl.exe availability and therefore also
// reports accurately (or "not installed") on any platform.

func windowsProbe() health.Probe {
	return func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_UNKNOWN, Summary: "running on non-Windows; check skipped"}
	}
}

func virtualizationProbe() health.Probe {
	return func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_UNKNOWN, Summary: "running on non-Windows; check skipped"}
	}
}
