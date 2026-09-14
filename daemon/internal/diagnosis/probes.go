package diagnosis

// Package diagnosis wires the health component probes for the daemon. Each
// probe returns a ComponentStatus that explains what is wrong, why it
// matters, what Envorka recommends, and whether it can safely fix it.

import (
	"context"
	"fmt"
	"strings"
	"time"

	envorkav1 "envorka.dev/envorka/api/gen/go/envorka/v1"
	"envorka.dev/envorka/daemon/internal/health"
	"envorka.dev/envorka/daemon/internal/system"
	"envorka.dev/envorka/daemon/internal/wsl"
)

// Options configure the default registry.
type Options struct {
	// WSLRunner is the wsl.exe invocation runner. Defaults to
	// wsl.DefaultRunner when nil.
	WSLRunner wsl.Runner
}

// Registry builds the default component registry for the daemon. Order is
// the display order used by status and doctor.
func Registry(opts Options) (*health.Registry, error) {
	if opts.WSLRunner == nil {
		opts.WSLRunner = wsl.DefaultRunner()
	}
	reg := health.NewRegistry()
	type entry struct {
		id     string
		probe  health.Probe
		budget time.Duration
	}
	entries := []entry{
		{"daemon", daemonProbe(), 0},
		{"windows", windowsProbe(), 3 * time.Second},
		{"virtualization", virtualizationProbe(), 3 * time.Second},
		{"wsl", wslProbe(opts.WSLRunner), 8 * time.Second},
		{"linux_kernel", kernelProbe(opts.WSLRunner), 3 * time.Second},
		{"resources", resourcesProbe(), time.Second},
	}
	for _, e := range entries {
		p := e.probe
		if e.budget > 0 {
			p = health.WithTimeout(p, e.budget)
		}
		if err := reg.Register(e.id, p); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

func daemonProbe() health.Probe {
	return func(context.Context) envorkav1.ComponentStatus {
		return envorkav1.ComponentStatus{Status: envorkav1.Status_HEALTHY, Summary: "daemon running"}
	}
}

func wslProbe(runner wsl.Runner) health.Probe {
	return func(ctx context.Context) envorkav1.ComponentStatus {
		if !wsl.Available() {
			return envorkav1.ComponentStatus{
				Status:         envorkav1.Status_CRITICAL,
				Summary:        "WSL is not installed",
				Reason:         "Envorka requires WSL2 to run Linux containers.",
				Recommendation: "Enable WSL2 (optional Windows component); on Windows 11 run `wsl --install`.",
				SafeToFix:      false,
			}
		}
		distro, err := wsl.DefaultDistro(ctx, runner)
		if err != nil {
			return envorkav1.ComponentStatus{
				Status:         envorkav1.Status_CRITICAL,
				Summary:        "could not query WSL status",
				Reason:         "WSL is broken or the Virtual Machine Platform is unavailable.",
				Recommendation: "Restart the WSL environment or repair Windows virtualization.",
				SafeToFix:      true,
			}
		}
		if distro == "" {
			return envorkav1.ComponentStatus{
				Status:         envorkav1.Status_WARNING,
				Summary:        "no default Linux distribution",
				Reason:         "Containers need a Linux distribution to run in.",
				Recommendation: "Install a distribution (e.g. `wsl --install -d Ubuntu`).",
				SafeToFix:      false,
			}
		}
		if err := wsl.Boot(ctx, runner, distro); err != nil {
			return envorkav1.ComponentStatus{
				Status:               envorkav1.Status_CRITICAL,
				Summary:              "Linux environment failed to start",
				Reason:               "Containers cannot start while the WSL environment is broken.",
				Recommendation:       "Restart the WSL environment.",
				SafeToFix:            true,
				RequiresConfirmation: false,
			}
		}
		summary := "WSL2 ready; default distro " + distro
		if distros, err := wsl.List(ctx, runner); err == nil {
			for _, d := range distros {
				if d.Default {
					if !d.Running {
						summary += " (stopped; boots on demand)"
					} else {
						summary += " (running)"
					}
					break
				}
			}
		}
		return envorkav1.ComponentStatus{Status: envorkav1.Status_HEALTHY, Summary: summary}
	}
}

func kernelProbe(runner wsl.Runner) health.Probe {
	return func(ctx context.Context) envorkav1.ComponentStatus {
		if !wsl.Available() {
			return envorkav1.ComponentStatus{Status: envorkav1.Status_UNKNOWN, Summary: "WSL not installed; kernel probe skipped"}
		}
		kv, err := wsl.KernelVersion(ctx, runner)
		if err != nil || kv == "" {
			return envorkav1.ComponentStatus{
				Status:  envorkav1.Status_UNKNOWN,
				Summary: "kernel version unavailable",
			}
		}
		return envorkav1.ComponentStatus{Status: envorkav1.Status_HEALTHY, Summary: kv}
	}
}

func resourcesProbe() health.Probe {
	return func(ctx context.Context) envorkav1.ComponentStatus {
		res, err := system.Memory()
		if err != nil {
			return envorkav1.ComponentStatus{
				Status:  envorkav1.Status_UNKNOWN,
				Summary: "memory probe failed: " + err.Error(),
			}
		}
		if res.TotalMemoryBytes == 0 {
			return envorkav1.ComponentStatus{Status: envorkav1.Status_UNKNOWN, Summary: "memory probe unavailable"}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "memory: %.1f GB installed", gb(res.TotalMemoryBytes))
		if res.AvailableMemoryBytes > 0 {
			fmt.Fprintf(&b, ", %.1f GB available", gb(res.AvailableMemoryBytes))
		}
		return envorkav1.ComponentStatus{Status: envorkav1.Status_HEALTHY, Summary: b.String()}
	}
}

func gb(n uint64) float64 {
	return float64(n) / (1024 * 1024 * 1024)
}
