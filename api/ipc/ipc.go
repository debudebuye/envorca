package ipc

// Package ipc provides the local transport shared by the Envorca daemon and
// its clients. Windows builds use a named pipe restricted to the current
// user; non-Windows builds use a Unix domain socket so the full stack is
// testable without Windows. Endpoint resolution, security, dial, and listen
// semantics have a single implementation (ADR-0003).

import (
	"context"
	"net"
	"os"
)

// EnvOverride forces the transport endpoint for tests and unusual setups.
const EnvOverride = "ENVORCA_SOCKET"

// EnvStateDirOverride forces the state directory used to derive directories
// and paths. It is honoured by DefaultStateDir only; endpoints derived
// through DefaultEndpoint prefer EnvOverride.
const EnvStateDirOverride = "ENVORCA_STATE_DIR"

// DefaultStateDir returns the platform default Envorca state directory.
func DefaultStateDir() string {
	if s := os.Getenv(EnvStateDirOverride); s != "" {
		return s
	}
	return defaultStateDir()
}

// DefaultEndpoint returns the default transport endpoint for a state
// directory, honoring the ENVORCA_SOCKET override.
func DefaultEndpoint(stateDir string) string {
	if s := os.Getenv(EnvOverride); s != "" {
		return s
	}
	return platformEndpoint(stateDir)
}

// DialContext opens a client connection to the endpoint.
func DialContext(ctx context.Context, endpoint string) (net.Conn, error) {
	return dialContext(ctx, endpoint)
}

// Listen opens a server listener on the endpoint. It fails if the endpoint
// is already in use.
func Listen(endpoint string) (net.Listener, error) {
	return listen(endpoint)
}

// Cleanup removes residual endpoint state (socket files). No-op on Windows.
func Cleanup(endpoint string) { cleanup(endpoint) }