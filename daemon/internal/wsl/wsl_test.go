package wsl

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeOutput(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"ascii", []byte("  Ubuntu-22.04  \n"), "Ubuntu-22.04"},
		{"utf16le with BOM", []byte{0xff, 0xfe, 'U', 0, 'b', 0, 'u', 0, 'n', 0, 't', 0, 'u', 0, '\n', 0}, "Ubuntu"},
		{"utf16le without BOM", []byte{'D', 0, 'o', 0, 'c', 0, 'k', 0, 'e', 0, 'r', 0, '\n', 0}, "Docker"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decodeOutput(c.in); got != c.want {
				t.Errorf("decodeOutput = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDistributions(t *testing.T) {
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("Windows Subsystem for Linux Distributions:\nUbuntu-22.04\nDocker\n"), nil
	}
	got, err := Distributions(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Ubuntu-22.04", "Docker"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Distributions = %v, want %v", got, want)
	}
}

func TestDistributionsNone(t *testing.T) {
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("There are no installed distributions.\n"), nil
	}
	got, err := Distributions(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no distributions, got %v", got)
	}
}

func TestDistributionsError(t *testing.T) {
	boom := errors.New("wsl missing")
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, boom
	}
	if _, err := Distributions(context.Background(), run); err != boom {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestListVerbose(t *testing.T) {
	raw := strings.Join([]string{
		"NAME                   STATE           VERSION",
		"* Ubuntu-22.04         Running         2",
		"  Alpine               Stopped         2",
		"  Fedora               Installing      2",
		"",
	}, "\n")
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte(raw), nil
	}
	distros, err := List(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if len(distros) != 3 {
		t.Fatalf("got %d distros, want 3", len(distros))
	}
	if !distros[0].Default || !distros[0].Running || distros[0].Version != 2 {
		t.Errorf("ubuntu = %+v", distros[0])
	}
	if distros[1].Default {
		t.Error("Alpine should not be default")
	}
	if distros[1].Running {
		t.Error("Alpine should not be running")
	}
	if distros[2].Running {
		t.Error("Fedora Installing → Running=false")
	}
}

func TestDefaultDistro(t *testing.T) {
	raw := strings.Join([]string{
		"NAME                   STATE           VERSION",
		"* Ubuntu-22.04         Running         2",
		"  Alpine               Stopped         2",
		"",
	}, "\n")
	run := func(ctx context.Context, args ...string) ([]byte, error) { return []byte(raw), nil }
	name, err := DefaultDistro(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Ubuntu-22.04" {
		t.Errorf("DefaultDistro = %q, want Ubuntu-22.04", name)
	}
}

func TestDefaultDistroNone(t *testing.T) {
	run := func(ctx context.Context, args ...string) ([]byte, error) { return []byte(""), nil }
	name, _ := DefaultDistro(context.Background(), run)
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

func TestKernelVersion(t *testing.T) {
	raw := strings.Join([]string{
		"WSL version: 2.0.0",
		"Kernel version: 5.15.133.1-microsoft-standard-WSL2",
		"WSLg version: 1.0.60",
	}, "\n")
	run := func(ctx context.Context, args ...string) ([]byte, error) { return []byte(raw), nil }
	kv, err := KernelVersion(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if kv != "5.15.133.1-microsoft-standard-WSL2" {
		t.Errorf("KernelVersion = %q", kv)
	}
}

func TestKernelVersionMissing(t *testing.T) {
	run := func(ctx context.Context, args ...string) ([]byte, error) { return []byte("WSL version: 1.0.0\n"), nil }
	kv, _ := KernelVersion(context.Background(), run)
	if kv != "" {
		t.Errorf("expected empty, got %q", kv)
	}
}

func TestBootSuccess(t *testing.T) {
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		if !strings.Contains(strings.Join(args, " "), "Ubuntu-22.04") {
			return nil, errors.New("wrong args")
		}
		return []byte("envorca:boot\n"), nil
	}
	if err := Boot(context.Background(), run, "Ubuntu-22.04"); err != nil {
		t.Error(err)
	}
}

func TestShutdown(t *testing.T) {
	called := false
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		called = true
		if len(args) != 1 || args[0] != "--shutdown" {
			t.Errorf("args = %v", args)
		}
		return nil, nil
	}
	if err := Shutdown(context.Background(), run); err != nil {
		t.Error(err)
	}
	if !called {
		t.Error("Shutdown runner not called")
	}
}