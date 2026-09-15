package runtime

// Docker is the V1 RuntimeDriver (ADR-0001): it reaches the docker CLI inside
// the default WSL2 distribution through the project's single wsl.exe bridge,
// always with argument slices (never shell strings).

import (
	"context"
	"errors"
	"strings"

	"envorca.dev/envorca/daemon/internal/wsl"
)

// NewDocker creates a Docker driver that runs docker commands through run.
// A nil run uses the default wsl.exe runner.
func NewDocker(run wsl.Runner) *Docker {
	if run == nil {
		run = wsl.DefaultRunner()
	}
	return &Docker{run: run}
}

// Docker is the Docker-in-WSL RuntimeDriver.
type Docker struct {
	run wsl.Runner
}

// Ping implements Driver.
func (d *Docker) Ping(ctx context.Context) Info {
	var info Info

	out, err := d.exec(ctx, "version", "--format", "{{.Client.Version}}")
	if err != nil {
		if d.transportFail(err) {
			info.Err = err
		}
		return info
	}
	info.ClientVersion = strings.TrimSpace(string(out))

	out, err = d.exec(ctx, "version", "--format", "{{.Server.Version}}")
	if err == nil {
		info.ServerVersion = strings.TrimSpace(string(out))
	}
	return info
}

// ListContainers implements Driver. Each container is parsed from a
// tab-separated docker ps template line.
func (d *Docker) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	args := []string{"ps", "--no-trunc", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}"}
	if all {
		args = append(args, "-a")
	}
	out, err := d.exec(ctx, args...)
	if err != nil {
		return nil, err
	}
	var containers []Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		c := Container{}
		if len(fields) > 0 {
			c.ID = fields[0]
		}
		if len(fields) > 1 {
			c.Name = fields[1]
		}
		if len(fields) > 2 {
			c.Image = fields[2]
		}
		if len(fields) > 3 {
			c.State = fields[3]
		}
		if len(fields) > 4 {
			c.Status = fields[4]
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// exec resolves the default distribution and runs the docker command with
// the given args inside it.
func (d *Docker) exec(ctx context.Context, args ...string) ([]byte, error) {
	distro, err := wsl.DefaultDistro(ctx, d.run)
	if err != nil {
		return nil, &transportError{err: err}
	}
	if distro == "" {
		return nil, &transportError{err: ErrNoDistro}
	}
	full := append([]string{"--distribution", distro, "--", "docker"}, args...)
	return d.run(ctx, full...)
}

// transportFail reports whether err originates from the transport (WSL or
// distribution resolution) rather than from the docker command itself.
func (d *Docker) transportFail(err error) bool {
	var te *transportError
	return errors.As(err, &te)
}

// transportError marks failures that originate outside docker: resolving the
// distribution or invoking wsl.exe itself.
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }