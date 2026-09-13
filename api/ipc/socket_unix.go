//go:build !windows

package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"time"
)

func dialContext(ctx context.Context, endpoint string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", endpoint)
}

func listen(endpoint string) (net.Listener, error) {
	if _, err := net.DialTimeout("unix", endpoint, 200*time.Millisecond); err == nil {
		return nil, errors.New("ipc: endpoint already in use")
	}
	os.Remove(endpoint)
	return net.Listen("unix", endpoint)
}

func cleanup(endpoint string) {
	os.Remove(endpoint)
}