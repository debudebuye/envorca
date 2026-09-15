//go:build windows

package diagnosis

import "testing"

func TestScQueryRunning(t *testing.T) {
	if !scQueryRunning([]byte("        STATE              : 4  RUNNING")) {
		t.Error("RUNNING marker not detected")
	}
	if scQueryRunning([]byte("        STATE              : 1  STOPPED")) {
		t.Error("STOPPED reported as running")
	}
	if scQueryRunning([]byte("")) {
		t.Error("empty output reported as running")
	}
}