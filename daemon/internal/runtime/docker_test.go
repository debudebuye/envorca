package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// scriptedRunner emulates wsl.exe for Docker-driver tests.
type scriptedRunner struct {
	distro     string // default distro listed by --list --verbose
	listErr    error  // error to return from --list --verbose
	dockerCmds map[string][]byte
	dockerErr  map[string]error
}

func (s *scriptedRunner) run(_ context.Context, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if joined == "--list --verbose" {
		if s.listErr != nil {
			return nil, s.listErr
		}
		if s.distro == "" {
			return []byte("  NAME            STATE       VERSION\nUbuntu       Stopped     2\n"), nil
		}
		return []byte("  NAME            STATE       VERSION\n* " + s.distro + "   Stopped     2\n"), nil
	}
	if strings.HasPrefix(joined, "--distribution ") {
		if b, ok := s.dockerCmds[joined]; ok {
			return b, s.dockerErr[joined]
		}
	}
	return nil, errors.New("unexpected invocation: " + joined)
}

func dockerArgs(distro, cmd string) string {
	return "--distribution " + distro + " -- docker " + cmd
}

func TestPingHealthy(t *testing.T) {
	s := &scriptedRunner{
		distro: "Ubuntu-24.04",
		dockerCmds: map[string][]byte{
			dockerArgs("Ubuntu-24.04", "version --format {{.Client.Version}}"): []byte("28.0.1\n"),
			dockerArgs("Ubuntu-24.04", "version --format {{.Server.Version}}"): []byte("28.0.1\n"),
		},
	}
	d := NewDocker(s.run)
	info := d.Ping(context.Background())
	if info.Err != nil {
		t.Fatalf("Ping: %v", info.Err)
	}
	if info.ClientVersion != "28.0.1" {
		t.Errorf("ClientVersion = %q, want 28.0.1", info.ClientVersion)
	}
	if info.ServerVersion != "28.0.1" {
		t.Errorf("ServerVersion = %q, want 28.0.1", info.ServerVersion)
	}
}

func TestPingServerDown(t *testing.T) {
	distro := "Ubuntu-24.04"
	clientCmd := "version --format {{.Client.Version}}"
	serverCmd := "version --format {{.Server.Version}}"
	s := &scriptedRunner{
		distro: distro,
		dockerCmds: map[string][]byte{
			dockerArgs(distro, clientCmd): []byte("28.0.1\n"),
		},
		dockerErr: map[string]error{
			dockerArgs(distro, serverCmd): errors.New("Cannot connect to the Docker daemon"),
		},
	}
	d := NewDocker(s.run)
	info := d.Ping(context.Background())
	if info.Err != nil {
		t.Fatalf("Ping should not report a transport error: %v", info.Err)
	}
	if info.ClientVersion != "28.0.1" {
		t.Errorf("ClientVersion = %q, want 28.0.1", info.ClientVersion)
	}
	if info.ServerVersion != "" {
		t.Errorf("ServerVersion = %q, want empty when daemon is down", info.ServerVersion)
	}
}

func TestPingCLIMissing(t *testing.T) {
	distro := "Ubuntu-24.04"
	cmd := "version --format {{.Client.Version}}"
	s := &scriptedRunner{
		distro: distro,
		dockerErr: map[string]error{
			dockerArgs(distro, cmd): errors.New("docker: command not found"),
		},
	}
	d := NewDocker(s.run)
	info := d.Ping(context.Background())
	if info.Err != nil {
		t.Fatalf("missing CLI must not be a transport error: %v", info.Err)
	}
	if info.ClientVersion != "" {
		t.Errorf("ClientVersion = %q, want empty", info.ClientVersion)
	}
}

func TestPingTransportFailureFromList(t *testing.T) {
	s := &scriptedRunner{listErr: context.DeadlineExceeded}
	d := NewDocker(s.run)
	info := d.Ping(context.Background())
	if info.Err == nil {
		t.Fatal("WSL list failure must surface as a transport error")
	}
}

func TestPingNoDistro(t *testing.T) {
	s := &scriptedRunner{}
	d := NewDocker(s.run)
	info := d.Ping(context.Background())
	if !errors.Is(info.Err, ErrNoDistro) {
		t.Fatalf("Err = %v, want ErrNoDistro", info.Err)
	}
}

func TestListContainers(t *testing.T) {
	distro := "Ubuntu-24.04"
	cmd := "ps --no-trunc --format {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}} -a"
	out := "12ab34cd56ef\tweb\tnginx:latest\trunning\tUp 5 minutes\n" +
		"98zy76xw54vu\told\tpostgres:16\texited\tExited (0) 2 hours ago\n"
	s := &scriptedRunner{
		distro:     distro,
		dockerCmds: map[string][]byte{dockerArgs(distro, cmd): []byte(out)},
	}
	d := NewDocker(s.run)
	containers, err := d.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("got %d containers, want 2", len(containers))
	}
	first := containers[0]
	if first.ID != "12ab34cd56ef" || first.Name != "web" || first.Image != "nginx:latest" {
		t.Errorf("first container parsed wrong: %+v", first)
	}
	if first.State != "running" || !strings.Contains(first.Status, "Up") {
		t.Errorf("first container state wrong: %+v", first)
	}
	if containers[1].State != "exited" {
		t.Errorf("second container state = %q, want exited", containers[1].State)
	}
}

func TestListContainersRunningOnly(t *testing.T) {
	distro := "Ubuntu-24.04"
	cmd := "ps --no-trunc --format {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}"
	s := &scriptedRunner{
		distro:     distro,
		dockerCmds: map[string][]byte{dockerArgs(distro, cmd): []byte("aabbcc\tweb\tnginx:latest\trunning\tUp 1 hour\n")},
	}
	d := NewDocker(s.run)
	containers, err := d.ListContainers(context.Background(), false)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 1 || containers[0].Name != "web" {
		t.Fatalf("got %+v, want one running container", containers)
	}
}