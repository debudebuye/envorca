//go:build windows

package ipc

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

var pipeConfig = winio.PipeConfig{
	SecurityDescriptor: buildSDDL(),
	InputBufferSize:    65536,
	OutputBufferSize:   65536,
}

func dialContext(ctx context.Context, endpoint string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, endpoint)
}

func listen(endpoint string) (net.Listener, error) {
	return winio.ListenPipe(endpoint, &pipeConfig)
}

func cleanup(string) {}

// buildSDDL grants full access to SYSTEM, built-in Administrators, and the
// current interactive user. Everything else is denied.
func buildSDDL() string {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "D:P(A;;GA;;;SY)(A;;GA;;;BA)"
	}
	return "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;" + user.User.Sid.String() + ")"
}