package diagnosis

import (
	"context"
	"errors"
	goruntime "runtime"
	"strings"
	"testing"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	daemonruntime "envorca.dev/envorca/daemon/internal/runtime"
)

// fakeDriver lets container probe tests control driver behavior directly.
type fakeDriver struct {
	info       daemonruntime.Info
	containers []daemonruntime.Container
	listErr    error
}

func (f *fakeDriver) Ping(context.Context) daemonruntime.Info { return f.info }
func (f *fakeDriver) ListContainers(context.Context, bool) ([]daemonruntime.Container, error) {
	return f.containers, f.listErr
}

func containerStatus(t *testing.T, driver daemonruntime.Driver) *envorcav1.ComponentStatus {
	t.Helper()
	t.Setenv("ENVORCA_WSL_EXE", "/bin/echo")
	reg, err := Registry(Options{WSLRunner: fakeRunner(nil), RuntimeDriver: driver})
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	for _, c := range reg.Snapshot(context.Background()) {
		if c.Id == "container_runtime" {
			return c
		}
	}
	t.Fatal("container_runtime not registered")
	return nil
}

func TestContainerProbeHealthy(t *testing.T) {
	st := containerStatus(t, &fakeDriver{
		info: daemonruntime.Info{ClientVersion: "28.0.1", ServerVersion: "28.0.1"},
		containers: []daemonruntime.Container{
			{Name: "web", State: "running"},
			{Name: "db", State: "exited"},
		},
	})
	if st.Status != envorcav1.Status_HEALTHY {
		t.Fatalf("container_runtime = %s %q, want HEALTHY", st.Status, st.Summary)
	}
	if !strings.Contains(st.Summary, "1 running, 2 total") {
		t.Errorf("summary %q should report container counts", st.Summary)
	}
}

func TestContainerProbeWarnsWhenCLIMissing(t *testing.T) {
	st := containerStatus(t, &fakeDriver{info: daemonruntime.Info{}})
	if st.Status != envorcav1.Status_WARNING {
		t.Fatalf("container_runtime = %s %q, want WARNING", st.Status, st.Summary)
	}
	if !strings.Contains(st.Summary, "Docker CLI") {
		t.Errorf("summary %q should mention Docker CLI", st.Summary)
	}
}

func TestContainerProbeCriticalWhenDaemonDown(t *testing.T) {
	st := containerStatus(t, &fakeDriver{info: daemonruntime.Info{ClientVersion: "28.0.1"}})
	if st.Status != envorcav1.Status_CRITICAL {
		t.Fatalf("container_runtime = %s %q, want CRITICAL", st.Status, st.Summary)
	}
	if !strings.Contains(st.Recommendation, "service docker start") {
		t.Errorf("recommendation %q should suggest starting the docker service", st.Recommendation)
	}
}

func TestContainerProbeUnknownOnTransportError(t *testing.T) {
	st := containerStatus(t, &fakeDriver{info: daemonruntime.Info{Err: errors.New("wsl broken")}})
	if st.Status != envorcav1.Status_UNKNOWN {
		t.Fatalf("container_runtime = %s %q, want UNKNOWN", st.Status, st.Summary)
	}
}

func TestContainerProbeSkipsWithoutWSL(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("wsl.exe is always present on Windows; the no-WSL guard needs a PATH without it")
	}
	t.Setenv("ENVORCA_WSL_EXE", "")
	t.Setenv("PATH", t.TempDir())
	reg, err := Registry(Options{WSLRunner: fakeRunner(nil), RuntimeDriver: &fakeDriver{info: daemonruntime.Info{ClientVersion: "1"}}})
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	for _, c := range reg.Snapshot(context.Background()) {
		if c.Id == "container_runtime" && c.Status != envorcav1.Status_UNKNOWN {
			t.Fatalf("container_runtime = %s, want UNKNOWN when WSL missing", c.Status)
		}
	}
}
