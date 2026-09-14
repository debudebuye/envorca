package health

import (
	"context"
	"testing"
	"time"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
)

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("a", nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("a", nil); err == nil {
		t.Error("expected duplicate ID error")
	}
}

func TestSnapshot(t *testing.T) {
	r := NewRegistry()
	r.Register("daemon", func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: "ok"}
	})
	r.Register("panicky", func(context.Context) envorcav1.ComponentStatus {
		panic("boom")
	})
	r.Register("unstamped", func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{}
	})
	r.Register("explicit-unknown", func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_UNKNOWN}
	})

	got := r.Snapshot(context.Background())
	if len(got) != 4 {
		t.Fatalf("snapshot length = %d, want 4", len(got))
	}
	if got[0].Id != "daemon" || got[0].Status != envorcav1.Status_HEALTHY {
		t.Errorf("daemon = %+v", got[0])
	}
	if got[1].Id != "panicky" || got[1].Status != envorcav1.Status_CRITICAL {
		t.Errorf("panicky = %+v", got[1])
	}
	if got[2].Status != envorcav1.Status_UNKNOWN {
		t.Errorf("unstamped = %+v", got[2])
	}
	if got[3].Status != envorcav1.Status_UNKNOWN {
		t.Errorf("explicit-unknown = %+v", got[3])
	}
}

func TestNotYetDiagnosed(t *testing.T) {
	r := NewRegistry()
	r.Register("wsl", NotYetDiagnosed("wsl"))
	got := r.Snapshot(context.Background())
	if got[0].Id != "wsl" || got[0].Status != envorcav1.Status_UNKNOWN {
		t.Errorf("wsl = %+v", got[0])
	}
}

func TestWithTimeout(t *testing.T) {
	slow := func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY}
	}
	p := WithTimeout(func(ctx context.Context) envorcav1.ComponentStatus {
		<-ctx.Done()
		return slow(ctx)
	}, 10*time.Millisecond)
	st := p(context.Background())
	if st.Status != envorcav1.Status_HEALTHY {
		t.Errorf("WithTimeout status = %s", st.Status)
	}
}

func TestOverallStatus(t *testing.T) {
	comp := func(s envorcav1.Status) *envorcav1.ComponentStatus {
		return &envorcav1.ComponentStatus{Id: "x", Status: s}
	}
	cases := []struct {
		name string
		in   []*envorcav1.ComponentStatus
		want envorcav1.Status
	}{
		{"all healthy", []*envorcav1.ComponentStatus{comp(envorcav1.Status_HEALTHY), comp(envorcav1.Status_HEALTHY)}, envorcav1.Status_HEALTHY},
		{"warning", []*envorcav1.ComponentStatus{comp(envorcav1.Status_HEALTHY), comp(envorcav1.Status_WARNING)}, envorcav1.Status_WARNING},
		{"critical dominates warning", []*envorcav1.ComponentStatus{comp(envorcav1.Status_WARNING), comp(envorcav1.Status_CRITICAL)}, envorcav1.Status_CRITICAL},
		{"unknown with healthy", []*envorcav1.ComponentStatus{comp(envorcav1.Status_HEALTHY), comp(envorcav1.Status_UNKNOWN)}, envorcav1.Status_UNKNOWN},
		{"unknown with warning", []*envorcav1.ComponentStatus{comp(envorcav1.Status_WARNING), comp(envorcav1.Status_UNKNOWN)}, envorcav1.Status_WARNING},
		{"empty", nil, envorcav1.Status_HEALTHY},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := OverallStatus(c.in); got != c.want {
				t.Errorf("OverallStatus = %v, want %v", got, c.want)
			}
		})
	}
}