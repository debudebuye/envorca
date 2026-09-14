//go:build !windows

package diagnosis

import (
	"context"

	envorkav1 "envorka.dev/envorka/api/gen/go/envorka/v1"
	"envorka.dev/envorka/daemon/internal/health"
)

// On non-Windows development machines these checks are skipped and reported
// as UNKNOWN. The wsl probe uses wsl.exe availability and therefore also
// reports accurately (or "not installed") on any platform.

func windowsProbe() health.Probe {
	return func(context.Context) envorkav1.ComponentStatus {
		return envorkav1.ComponentStatus{Status: envorkav1.Status_UNKNOWN, Summary: "running on non-Windows; check skipped"}
	}
}

func virtualizationProbe() health.Probe {
	return func(context.Context) envorkav1.ComponentStatus {
		return envorkav1.ComponentStatus{Status: envorkav1.Status_UNKNOWN, Summary: "running on non-Windows; check skipped"}
	}
}
