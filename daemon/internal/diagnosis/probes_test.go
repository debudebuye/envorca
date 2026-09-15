package diagnosis

import (
	"context"
	"strings"
	"testing"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/daemon/internal/wsl"
)

func fakeRunner(script map[string]func() error) wsl.Runner {
	return func(_ context.Context, args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		if run, ok := script[key]; ok {
			if err := run(); err != nil {
				return nil, err
			}
		}
		switch key {
		case "--list --verbose":
			return []byte("  NAME                    STATE           VERSION\n* Ubuntu-24.04            Running         2\n"), nil
		case "--version":
			return []byte("Kernel version: 6.6.13.1-2\n"), nil
		default:
			return []byte(""), nil
		}
	}
}

func TestRegistryRegistersAllComponents(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	reg, err := Registry(Options{WSLRunner: fakeRunner(nil)})
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	want := []string{"daemon", "windows", "virtualization", "wsl", "linux_kernel", "resources", "container_runtime"}
	comps := reg.Snapshot(context.Background())
	if len(comps) != len(want) {
		t.Fatalf("got %d components, want %d", len(comps), len(want))
	}
	for i, id := range want {
		if comps[i].Id != id {
			t.Errorf("component %d = %q, want %q", i, comps[i].Id, id)
		}
	}
}

func TestWSLProbeHealthy(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	st := statusFor(t, "wsl", fakeRunner(nil))
	if st.Status != envorcav1.Status_HEALTHY {
		t.Fatalf("wsl = %s %q, want HEALTHY", st.Status, st.Summary)
	}
	if !strings.Contains(st.Summary, "Ubuntu-24.04") {
		t.Errorf("summary %q should name the default distro", st.Summary)
	}
}

func TestWSLProbeBootFailureIsCriticalAndRepairable(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	runner := fakeRunner(map[string]func() error{
		"--distribution Ubuntu-24.04 -- echo envorca:boot": func() error { return context.DeadlineExceeded },
	})
	st := statusFor(t, "wsl", runner)
	if st.Status != envorcav1.Status_CRITICAL {
		t.Fatalf("wsl = %s, want CRITICAL", st.Status)
	}
	if !st.SafeToFix {
		t.Error("boot failure should be SafeToFix")
	}
	if !strings.Contains(st.Summary, "failed to start") {
		t.Errorf("summary %q should mention failed to start", st.Summary)
	}
}

func TestWSLProbeNoDefaultDistroWarns(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	runner := func(_ context.Context, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "--list --verbose" {
			return []byte("  NAME                    STATE           VERSION\nUbuntu-22.04              Stopped         2\n"), nil
		}
		return nil, nil
	}
	st := statusFor(t, "wsl", runner)
	if st.Status != envorcav1.Status_WARNING {
		t.Fatalf("wsl = %s, want WARNING", st.Status)
	}
}

func TestDaemonAndResourcesHealthy(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	runner := fakeRunner(nil)
	if st := statusFor(t, "daemon", runner); st.Status != envorcav1.Status_HEALTHY {
		t.Errorf("daemon = %s, want HEALTHY", st.Status)
	}
	if st := statusFor(t, "resources", runner); st.Status != envorcav1.Status_HEALTHY {
		t.Errorf("resources = %s, want HEALTHY", st.Status)
	}
}

func statusFor(t *testing.T, id string, runner wsl.Runner) *envorcav1.ComponentStatus {
	t.Helper()
	reg, err := Registry(Options{WSLRunner: runner})
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	for _, c := range reg.Snapshot(context.Background()) {
		if c.Id == id {
			return c
		}
	}
	t.Fatalf("component %q not registered", id)
	return nil
}