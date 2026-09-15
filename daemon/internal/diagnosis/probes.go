package diagnosis

// Package diagnosis wires the health component probes for the daemon. Each
// probe returns a ComponentStatus that explains what is wrong, why it
// matters, what Envorca recommends, and whether it can safely fix it.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/daemon/internal/health"
	"envorca.dev/envorca/daemon/internal/runtime"
	"envorca.dev/envorca/daemon/internal/system"
	"envorca.dev/envorca/daemon/internal/wsl"
)

// minKernelMajor and minKernelMinor are the lowest WSL2 kernel version the
// daemon considers healthy. WSL2 has shipped kernels >= 5.10 for years; an
// older kernel indicates a stale WSL install that should be updated.
const (
	minKernelMajor = 5
	minKernelMinor = 10
)

// Options configure the default registry.
type Options struct {
	// WSLRunner is the wsl.exe invocation runner. Defaults to
	// wsl.DefaultRunner when nil.
	WSLRunner wsl.Runner
	// RuntimeDriver is the container bridge. Defaults to a Docker driver
	// using WSLRunner when nil.
	RuntimeDriver runtime.Driver
}

// Registry builds the default component registry for the daemon. Order is
// the display order used by status and doctor.
func Registry(opts Options) (*health.Registry, error) {
	if opts.WSLRunner == nil {
		opts.WSLRunner = wsl.DefaultRunner()
	}
	if opts.RuntimeDriver == nil {
		opts.RuntimeDriver = runtime.NewDocker(opts.WSLRunner)
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
		{"container_runtime", containerProbe(opts.RuntimeDriver), 8 * time.Second},
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
	return func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: "daemon running"}
	}
}

func wslProbe(runner wsl.Runner) health.Probe {
	return func(ctx context.Context) envorcav1.ComponentStatus {
		if !wsl.Available() {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_CRITICAL,
				Summary:        "WSL is not installed",
				Reason:         "Envorca requires WSL2 to run Linux containers.",
				Recommendation: "Enable WSL2 (optional Windows component); on Windows 11 run `wsl --install`.",
				SafeToFix:      false,
			}
		}
		distro, err := wsl.DefaultDistro(ctx, runner)
		if err != nil {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_CRITICAL,
				Summary:        "could not query WSL status",
				Reason:         "WSL is broken or the Virtual Machine Platform is unavailable.",
				Recommendation: "Restart the WSL environment or repair Windows virtualization.",
				SafeToFix:      true,
			}
		}
		if distro == "" {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_WARNING,
				Summary:        "no default Linux distribution",
				Reason:         "Containers need a Linux distribution to run in.",
				Recommendation: "Install a distribution (e.g. `wsl --install -d Ubuntu`).",
				SafeToFix:      false,
			}
		}
		if err := wsl.Boot(ctx, runner, distro); err != nil {
			return envorcav1.ComponentStatus{
				Status:               envorcav1.Status_CRITICAL,
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
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: summary}
	}
}

func kernelProbe(runner wsl.Runner) health.Probe {
	return func(ctx context.Context) envorcav1.ComponentStatus {
		if !wsl.Available() {
			return envorcav1.ComponentStatus{Status: envorcav1.Status_UNKNOWN, Summary: "WSL not installed; kernel probe skipped"}
		}
		kv, err := wsl.KernelVersion(ctx, runner)
		if err != nil || kv == "" {
			return envorcav1.ComponentStatus{
				Status:  envorcav1.Status_UNKNOWN,
				Summary: "kernel version unavailable",
			}
		}
		maj, min, ok := parseKernelVersion(kv)
		if !ok {
			return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: kv}
		}
		if maj < minKernelMajor || (maj == minKernelMajor && min < minKernelMinor) {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_WARNING,
				Summary:        fmt.Sprintf("kernel %s is older than the required %d.%d", kv, minKernelMajor, minKernelMinor),
				Reason:         "An outdated WSL kernel can cause VM and container stability problems.",
				Recommendation: "Update WSL: run `wsl --update` from an elevated shell.",
				SafeToFix:      false,
			}
		}
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: kv}
	}
}

// parseKernelVersion extracts the leading numeric major/minor from a kernel
// version string such as "5.15.90.1-microsoft-standard-WSL2".
func parseKernelVersion(s string) (major, minor int, ok bool) {
	core := s
	if i := strings.IndexAny(s, "-+_"); i >= 0 {
		core = s[:i]
	}
	parts := strings.Split(core, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}

func resourcesProbe() health.Probe {
	return func(ctx context.Context) envorcav1.ComponentStatus {
		res, err := system.Memory()
		if err != nil {
			return envorcav1.ComponentStatus{
				Status:  envorcav1.Status_UNKNOWN,
				Summary: "memory probe failed: " + err.Error(),
			}
		}
		if res.TotalMemoryBytes == 0 {
			return envorcav1.ComponentStatus{Status: envorcav1.Status_UNKNOWN, Summary: "memory probe unavailable"}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "memory: %.1f GB installed", gb(res.TotalMemoryBytes))
		if res.AvailableMemoryBytes > 0 {
			fmt.Fprintf(&b, ", %.1f GB available", gb(res.AvailableMemoryBytes))
		}
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: b.String()}
	}
}

func gb(n uint64) float64 {
	return float64(n) / (1024 * 1024 * 1024)
}

// containerProbe diagnoses the container runtime reachable through the
// default WSL distribution (ADR-0001). It reports UNKNOWN when there is no
// Linux environment to run in, WARNING when docker is not installed, and
// CRITICAL when the docker daemon is not answering.
func containerProbe(driver runtime.Driver) health.Probe {
	return func(ctx context.Context) envorcav1.ComponentStatus {
		if !wsl.Available() {
			return envorcav1.ComponentStatus{Status: envorcav1.Status_UNKNOWN, Summary: "WSL not installed; container check skipped"}
		}
		info := driver.Ping(ctx)
		if info.Err != nil {
			return envorcav1.ComponentStatus{
				Status:  envorcav1.Status_UNKNOWN,
				Summary: "cannot reach container runtime",
				Reason:  info.Err.Error(),
			}
		}
		if info.ClientVersion == "" {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_WARNING,
				Summary:        "Docker CLI is not installed in the default distribution",
				Reason:         "Development containers need the Docker CLI inside the WSL distribution.",
				Recommendation: "Install Docker (e.g. `sudo apt-get install docker.io` inside the distribution) or use Docker Desktop with WSL integration enabled.",
				SafeToFix:      false,
			}
		}
		if info.ServerVersion == "" {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_CRITICAL,
				Summary:        "Docker CLI present but the Docker daemon is not responding",
				Reason:         "Containers cannot run while the Docker daemon is stopped.",
				Recommendation: "Start the Docker service (`sudo service docker start` in the distribution) or start Docker Desktop.",
				SafeToFix:      false,
			}
		}
		summary := fmt.Sprintf("Docker ready (server %s)", info.ServerVersion)
		containers, err := driver.ListContainers(ctx, true)
		if err == nil {
			running := 0
			for _, c := range containers {
				if c.State == "running" {
					running++
				}
			}
			summary += fmt.Sprintf("; %d running, %d total", running, len(containers))
		}
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: summary}
	}
}
