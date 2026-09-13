//go:build !windows

package diagnosis

import (
	"context"

	runorkav1 "runorka.dev/runorka/api/gen/go/runorka/v1"
	"runorka.dev/runorka/daemon/internal/health"
)

// On non-Windows development machines these checks are skipped and reported
// as UNKNOWN. The wsl probe uses wsl.exe availability and therefore also
// reports accurately (or "not installed") on any platform.

func windowsProbe() health.Probe {
	return func(context.Context) runorkav1.ComponentStatus {
		return runorkav1.ComponentStatus{Status: runorkav1.Status_UNKNOWN, Summary: "running on non-Windows; check skipped"}
	}
}

func virtualizationProbe() health.Probe {
	return func(context.Context) runorkav1.ComponentStatus {
		return runorkav1.ComponentStatus{Status: runorkav1.Status_UNKNOWN, Summary: "running on non-Windows; check skipped"}
	}
}
