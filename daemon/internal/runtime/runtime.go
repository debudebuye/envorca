// Package runtime implements the container bridge between the Windows daemon
// and the Linux development environment (ADR-0001). Every container operation
// crosses the Windows/Linux boundary through a RuntimeDriver; the only V1
// driver runs the Docker CLI inside the default WSL2 distribution.
package runtime

import (
	"context"
	"errors"
)

// ErrNoDistro reports that no default WSL distribution exists to run
// container commands in.
var ErrNoDistro = errors.New("no default WSL distribution available")

// Info summarises container-runtime reachability.
type Info struct {
	// ClientVersion is the docker CLI version when the CLI is reachable
	// inside the distribution, else "".
	ClientVersion string
	// ServerVersion is the docker daemon version when the daemon answers,
	// else "".
	ServerVersion string
	// Err is set only when the transport itself failed (for example WSL is
	// broken or has no default distribution). A missing CLI or a silent
	// daemon are reported as empty versions, not as Err.
	Err error
}

// Container is one entry from the runtime's container listing.
type Container struct {
	ID     string
	Name   string
	Image  string
	State  string // "running", "exited", ...
	Status string // human-readable, e.g. "Up 5 minutes"
}

// Driver is the container-runtime seam defined by ADR-0001. The daemon and
// its probes depend only on this interface.
type Driver interface {
	// Ping checks whether the runtime CLI is present and the daemon answers.
	Ping(ctx context.Context) Info
	// ListContainers lists containers. When all is false, only running
	// containers are returned.
	ListContainers(ctx context.Context, all bool) ([]Container, error)
}