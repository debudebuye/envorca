package diagnosis

import (
	"context"
	"strings"
	"testing"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
)

func TestParseKernelVersion(t *testing.T) {
	cases := []struct {
		in           string
		major, minor int
		ok           bool
	}{
		{"6.6.13.1-2", 6, 6, true},
		{"5.15.90.1-microsoft-standard-WSL2", 5, 15, true},
		{"5.10.0.3-generic", 5, 10, true},
		{"4.19.128-microsoft-standard", 4, 19, true},
		{"not-a-kernel", 0, 0, false},
		{"5", 0, 0, false},
		{"", 0, 0, false},
		{"5.15.90.1-2", 5, 15, true},
	}
	for _, c := range cases {
		maj, min, ok := parseKernelVersion(c.in)
		if maj != c.major || min != c.minor || ok != c.ok {
			t.Errorf("parseKernelVersion(%q) = (%d,%d,%v), want (%d,%d,%v)",
				c.in, maj, min, ok, c.major, c.minor, c.ok)
		}
	}
}

func TestKernelProbeHealthy(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	st := statusFor(t, "linux_kernel", fakeRunner(nil))
	if st.Status != envorcav1.Status_HEALTHY {
		t.Fatalf("kernel = %s %q, want HEALTHY", st.Status, st.Summary)
	}
	if !strings.Contains(st.Summary, "6.6.13") {
		t.Errorf("summary %q should contain kernel version", st.Summary)
	}
}

func TestKernelProbeWarnsOnOutdatedKernel(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	runner := func(_ context.Context, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "--version" {
			return []byte("Kernel version: 4.19.128-microsoft-standard\n"), nil
		}
		return nil, nil
	}
	st := statusFor(t, "linux_kernel", runner)
	if st.Status != envorcav1.Status_WARNING {
		t.Fatalf("kernel = %s %q, want WARNING", st.Status, st.Summary)
	}
	if !strings.Contains(st.Recommendation, "wsl --update") {
		t.Errorf("recommendation %q should suggest wsl --update", st.Recommendation)
	}
	if st.SafeToFix {
		t.Error("kernel update must not be auto-fixed")
	}
}

func TestKernelProbeUnknownOnUnavailable(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	runner := fakeRunner(map[string]func() error{
		"--version": func() error { return context.DeadlineExceeded },
	})
	st := statusFor(t, "linux_kernel", runner)
	if st.Status != envorcav1.Status_UNKNOWN {
		t.Fatalf("kernel = %s, want UNKNOWN", st.Status)
	}
}

func TestKernelProbeSkipsWithoutWSL(t *testing.T) {
	t.Setenv("ENVORCA_WSL_EXE", "/definitely/missing/wsl.exe")
	st := kernelStatus(t)
	if st.Status != envorcav1.Status_UNKNOWN {
		t.Fatalf("kernel = %s, want UNKNOWN when WSL is missing", st.Status)
	}
}

func kernelStatus(t *testing.T) *envorcav1.ComponentStatus {
	t.Helper()
	return statusFor(t, "linux_kernel", nil)
}