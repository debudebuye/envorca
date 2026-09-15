package main

import (
	"bytes"
	"strings"
	"testing"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
)

func TestPlainStatus(t *testing.T) {
	cases := []struct {
		in   envorcav1.Status
		want string
	}{
		{envorcav1.Status_HEALTHY, "HEALTHY"},
		{envorcav1.Status_WARNING, "WARNING"},
		{envorcav1.Status_CRITICAL, "CRITICAL"},
		{envorcav1.Status_UNKNOWN, "UNKNOWN"},
		{envorcav1.Status_STATUS_UNSPECIFIED, "UNKNOWN"},
	}
	for _, c := range cases {
		if got := plainStatus(c.in); got != c.want {
			t.Errorf("plainStatus(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDecorateStatusNoTerminalColor(t *testing.T) {
	// Without a terminal, decorateStatus must be a no-op pass-through.
	if got := decorateStatus(envorcav1.Status_HEALTHY, "HEALTHY"); got != "HEALTHY" {
		t.Errorf("decorateStatus = %q, want plain", got)
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		secs int64
		want string
	}{
		{0, "0s"},
		{59, "59s"},
		{60, "1m 0s"},
		{61, "1m 1s"},
		{3600, "1h 0m"},
		{3661, "1h 1m"},
	}
	for _, c := range cases {
		if got := formatUptime(c.secs); got != c.want {
			t.Errorf("formatUptime(%d) = %q, want %q", c.secs, got, c.want)
		}
	}
}

func TestOutcomeLabel(t *testing.T) {
	if got := outcomeLabel("verified"); got != " (verified)" {
		t.Errorf("outcomeLabel(verified) = %q", got)
	}
	if got := outcomeLabel(""); got != "" {
		t.Errorf("outcomeLabel(empty) = %q, want empty", got)
	}
}

func TestPromptYesNo(t *testing.T) {
	var out bytes.Buffer
	answer := ""
	if ok, err := promptYesNo(strings.NewReader(answer+"\n"), &out); err != nil || ok {
		t.Errorf("blank answer: ok=%v err=%v, want no", ok, err)
	}
	answer = "y"
	if ok, err := promptYesNo(strings.NewReader(answer+"\n"), &out); err != nil || !ok {
		t.Errorf("y answer: ok=%v err=%v, want yes", ok, err)
	}
	if ok, err := promptYesNo(strings.NewReader("Y\n"), &out); err != nil || !ok {
		t.Errorf("Y answer: ok=%v err=%v, want yes", ok, err)
	}
	if ok, err := promptYesNo(strings.NewReader("yes\n"), &out); err != nil || !ok {
		t.Errorf("yes answer: ok=%v err=%v, want yes", ok, err)
	}
	if ok, err := promptYesNo(strings.NewReader("n\n"), &out); err != nil || ok {
		t.Errorf("n answer: ok=%v err=%v, want no", ok, err)
	}
	// EOF counts as no.
	if ok, err := promptYesNo(strings.NewReader(""), &out); err != nil || ok {
		t.Errorf("EOF answer: ok=%v err=%v, want no", ok, err)
	}
}

func statusWith(id string, st envorcav1.Status, summary string) *envorcav1.ComponentStatus {
	return &envorcav1.ComponentStatus{Id: id, Status: st, Summary: summary}
}

func TestRenderStatus(t *testing.T) {
	var buf bytes.Buffer
	renderStatus(&buf, &envorcav1.EnvironmentStatus{
		DaemonState:   "running",
		Version:       "0.1.0-dev",
		Socket:        "/tmp/test.sock",
		UptimeSeconds: 65,
		OverallStatus: envorcav1.Status_HEALTHY,
		Components: []*envorcav1.ComponentStatus{
			statusWith("daemon", envorcav1.Status_HEALTHY, "daemon running"),
			statusWith("wsl", envorcav1.Status_CRITICAL, "broken"),
		},
	})
	out := buf.String()
	for _, want := range []string{"ENVORCA DAEMON", "running", "0.1.0-dev", "/tmp/test.sock", "1m 5s", "daemon", "wsl", "HEALTHY", "CRITICAL"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderDoctorHealthyAndProblems(t *testing.T) {
	var buf bytes.Buffer
	renderDoctor(&buf, &envorcav1.EnvironmentStatus{
		OverallStatus: envorcav1.Status_WARNING,
		Components: []*envorcav1.ComponentStatus{
			statusWith("daemon", envorcav1.Status_HEALTHY, "daemon running"),
			{
				Id: "wsl", Status: envorcav1.Status_CRITICAL,
				Summary:        "Linux environment failed to start",
				Reason:         "boot failed",
				Recommendation: "restart wsl",
				SafeToFix:      true,
			},
		},
	})
	out := buf.String()
	for _, want := range []string{"ENVORCA DOCTOR", "Overall:", "WARNING", "CRITICAL", "Why: boot failed", "Recommend: restart wsl", "envorca repair"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderEvent(t *testing.T) {
	ev := &envorcav1.Event{
		Seq:       7,
		Timestamp: "2026-09-15T10:30:00.123Z",
		Level:     "info",
		Component: "recovery",
		Name:      "repair_executed",
		Fields:    map[string]string{"result": "ok", "action": "wsl.start"},
	}
	out := renderEvent(ev)
	for _, want := range []string{"10:30:00", "info", "recovery", "repair_executed", "action=\"wsl.start\"", "result=\"ok\""} {
		if !strings.Contains(out, want) {
			t.Errorf("renderEvent missing %q: %s", want, out)
		}
	}
}

func TestTimeOnly(t *testing.T) {
	if got := timeOnly("2026-09-15T10:30:00.123Z"); got != "10:30:00" {
		t.Errorf("timeOnly = %q, want 10:30:00", got)
	}
	if got := timeOnly(""); got != "" {
		t.Errorf("timeOnly empty = %q", got)
	}
	if got := timeOnly("garbage"); got != "" {
		t.Errorf("timeOnly garbage = %q", got)
	}
}

func TestDisplayTime(t *testing.T) {
	in := "2026-09-15T10:30:00Z"
	out := displayTime(in)
	if !strings.Contains(out, "2026-09-15") {
		t.Errorf("displayTime(%q) = %q, want date prefix", in, out)
	}
	// Malformed timestamps pass through untouched.
	if got := displayTime("nope"); got != "nope" {
		t.Errorf("displayTime(bad) = %q, want passthrough", got)
	}
}

func TestStatusFromString(t *testing.T) {
	cases := []struct {
		in   string
		want envorcav1.Status
	}{
		{"HEALTHY", envorcav1.Status_HEALTHY},
		{"WARNING", envorcav1.Status_WARNING},
		{"CRITICAL", envorcav1.Status_CRITICAL},
		{"UNKNOWN", envorcav1.Status_UNKNOWN},
		{"bogus", envorcav1.Status_UNKNOWN},
	}
	for _, c := range cases {
		if got := statusFromString(c.in); got != c.want {
			t.Errorf("statusFromString(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSnapshotLine(t *testing.T) {
	short := `{"overall_status":"HEALTHY"}`
	if got := snapshotLine(short); got != short {
		t.Errorf("snapshotLine short = %q", got)
	}
	long := strings.Repeat("x", 100)
	if got := snapshotLine(long); len(got) != 60+3 {
		t.Errorf("snapshotLine long length = %d, want 63", len(got))
	}
}

func TestRenderHistoryEmpty(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnostics(&buf, nil)
	if !strings.Contains(buf.String(), "(none recorded)") {
		t.Errorf("empty diagnostics should say none recorded:\n%s", buf.String())
	}
	buf.Reset()
	renderRepairHistory(&buf, nil)
	if !strings.Contains(buf.String(), "(none recorded)") {
		t.Errorf("empty repairs should say none recorded:\n%s", buf.String())
	}
}