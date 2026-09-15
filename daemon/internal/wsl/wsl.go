package wsl

// Package wsl is the single place the daemon talks to wsl.exe (ADR-0001).
// V1 funnels every Windows/Linux bridge call through wsl.exe argument
// slices (never shell strings). Parsing is defensive: piped wsl.exe output
// is UTF-16 on some Windows builds, so all output goes through decodeOutput.

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Runner executes wsl.exe with the given args and returns raw stdout.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DefaultRunner runs wsl.exe found on PATH.
func DefaultRunner() Runner {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, wslExe(), args...)
		return cmd.Output()
	}
}

// wslExe returns the wsl executable name, honoring ENVORCA_WSL_EXE for tests
// and unusual setups.
func wslExe() string {
	if e := os.Getenv("ENVORCA_WSL_EXE"); e != "" {
		return e
	}
	return "wsl.exe"
}

// Available reports whether wsl.exe can be found on PATH. An explicit
// ENVORCA_WSL_EXE override is trusted so tests can simulate WSL on any
// platform; a missing executable surfaces downstream as command errors.
func Available() bool {
	if os.Getenv("ENVORCA_WSL_EXE") != "" {
		return true
	}
	_, err := exec.LookPath(wslExe())
	return err == nil
}

// Distro is one entry of `wsl --list --verbose`.
type Distro struct {
	Name    string
	Default bool
	Running bool
	Version int
}

var verboseLine = regexp.MustCompile(`^(\*?)\s*(.+?)\s{2,}(Stopped|Running|Installing|Uninstalling|Converting)\s+(2|1)(?:\s*)$`)

// List parses `wsl --list --verbose`. Unparseable lines are skipped.
func List(ctx context.Context, run Runner) ([]Distro, error) {
	out, err := run(ctx, "--list", "--verbose")
	if err != nil {
		return nil, err
	}
	var distros []Distro
	for _, raw := range strings.Split(decodeOutput(out), "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "NAME") {
			continue
		}
		m := verboseLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		version, _ := strconv.Atoi(strings.TrimSpace(m[4]))
		distros = append(distros, Distro{
			Name:    strings.TrimSpace(m[2]),
			Default: strings.TrimSpace(m[1]) == "*",
			Running: m[3] == "Running",
			Version: version,
		})
	}
	return distros, nil
}

// DefaultDistro returns the default distribution name, or "" when none.
func DefaultDistro(ctx context.Context, run Runner) (string, error) {
	distros, err := List(ctx, run)
	if err != nil {
		return "", err
	}
	for _, d := range distros {
		if d.Default {
			return d.Name, nil
		}
	}
	return "", nil
}

// KernelVersion parses the WSL kernel version from `wsl --version`.
func KernelVersion(ctx context.Context, run Runner) (string, error) {
	out, err := run(ctx, "--version")
	if err != nil {
		return "", err
	}
	for _, raw := range strings.Split(decodeOutput(out), "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if v, ok := strings.CutPrefix(line, "Kernel version:"); ok {
			return strings.TrimSpace(v), nil
		}
	}
	return "", nil
}

// Boot starts the given distribution (automatic for WSL2) and returns nil
// once a command inside it succeeds.
func Boot(ctx context.Context, run Runner, distro string) error {
	_, err := run(ctx, "--distribution", distro, "--", "echo", "envorca:boot")
	return err
}

// Shutdown stops the WSL virtual machine. Affects all distributions.
func Shutdown(ctx context.Context, run Runner) error {
	_, err := run(ctx, "--shutdown")
	return err
}

// Distributions lists installed Linux distribution names
// (wsl --list --quiet). Returns an empty slice when none are installed.
func Distributions(ctx context.Context, run Runner) ([]string, error) {
	out, err := run(ctx, "--list", "--quiet")
	if err != nil {
		return nil, err
	}
	text := decodeOutput(out)
	var names []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || isHeader(line) {
			continue
		}
		names = append(names, line)
	}
	return names, nil
}

func isHeader(line string) bool {
	if strings.Contains(line, "Windows Subsystem for Linux") {
		return true
	}
	if strings.Contains(strings.ToLower(line), "no installed distributions") {
		return true
	}
	return false
}

// decodeOutput normalizes wsl.exe stdout to trimmed UTF-8 text. It handles
// UTF-16LE/BE with or without a byte-order mark, detecting UTF-16 by NUL
// bytes in the leading bytes when no BOM is present.
func decodeOutput(b []byte) string {
	if len(b) >= 2 {
		switch {
		case b[0] == 0xff && b[1] == 0xfe:
			return decodeUTF16(b[2:], binary.LittleEndian)
		case b[0] == 0xfe && b[1] == 0xff:
			return decodeUTF16(b[2:], binary.BigEndian)
		}
	}
	n := len(b)
	if n > 256 {
		n = 256
	}
	if bytes.IndexByte(b[:n], 0) >= 0 {
		return decodeUTF16(b, binary.LittleEndian)
	}
	return strings.TrimSpace(string(b))
}

func decodeUTF16(b []byte, order binary.ByteOrder) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, order.Uint16(b[i:]))
	}
	return strings.TrimSpace(string(utf16.Decode(u)))
}