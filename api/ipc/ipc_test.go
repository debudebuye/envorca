//go:build !windows

package ipc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListenDialRoundTrip(t *testing.T) {
	ep := filepath.Join(t.TempDir(), "envorka.sock")
	ln, err := Listen(ep)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		buf := make([]byte, 8)
		n, err := conn.Read(buf)
		if err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
		if string(buf[:n]) != "hello" {
			serverErr <- errors.New("bad payload")
			return
		}
	}()

	conn, err := DialContext(context.Background(), ep)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server handler timed out")
	}
}

func TestListenRefusesInUse(t *testing.T) {
	ep := filepath.Join(t.TempDir(), "envorka.sock")
	ln, err := Listen(ep)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if _, err := Listen(ep); err == nil {
		t.Error("expected error binding in-use endpoint")
	}
}

func TestEndpointOverride(t *testing.T) {
	t.Setenv(EnvOverride, "/tmp/custom.sock")
	if got := DefaultEndpoint("/ignored"); got != "/tmp/custom.sock" {
		t.Errorf("DefaultEndpoint = %q, want override", got)
	}
	t.Setenv(EnvOverride, "")
	if got := DefaultEndpoint("/state"); !filepath.IsAbs(got) {
		t.Errorf("DefaultEndpoint = %q, want absolute path", got)
	}
}

func TestCleanupRemovesSocket(t *testing.T) {
	ep := filepath.Join(t.TempDir(), "envorka.sock")
	ln, err := Listen(ep)
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
	Cleanup(ep)
	if _, err := os.Stat(ep); !os.IsNotExist(err) {
		t.Errorf("socket file still present after cleanup: %v", err)
	}
}