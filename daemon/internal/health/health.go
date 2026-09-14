package health

// Package health owns the component-registry model used by GetStatus and,
// later, Envorka Doctor. Every component is either healthy, warning,
// critical, or unknown. Diagnostics must explain what is wrong, why it
// matters, what Envorka recommends, and whether it can safely fix it.

import (
	"context"
	"fmt"
	"time"

	envorkav1 "envorka.dev/envorka/api/gen/go/envorka/v1"
)

// Probe evaluates a single component.
type Probe func(context.Context) envorkav1.ComponentStatus

// Component pairs a stable ID with its probe.
type Component struct {
	ID    string
	Probe Probe
}

// Registry holds components in registration (display) order.
type Registry struct {
	components []Component
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a component. Duplicate IDs are rejected.
func (r *Registry) Register(id string, p Probe) error {
	for _, c := range r.components {
		if c.ID == id {
			return fmt.Errorf("component %q already registered", id)
		}
	}
	if p == nil {
		p = func(context.Context) envorkav1.ComponentStatus {
			return envorkav1.ComponentStatus{Status: envorkav1.Status_UNKNOWN}
		}
	}
	r.components = append(r.components, Component{ID: id, Probe: p})
	return nil
}

// Snapshot evaluates every component sequentially and returns the status
// list in registration order. A panicking probe reports critical rather
// than killing the daemon.
func (r *Registry) Snapshot(ctx context.Context) []*envorkav1.ComponentStatus {
	out := make([]*envorkav1.ComponentStatus, 0, len(r.components))
	for _, c := range r.components {
		st := func() (st envorkav1.ComponentStatus) {
			defer func() {
				if p := recover(); p != nil {
					st = envorkav1.ComponentStatus{
						Id:      c.ID,
						Status:  envorkav1.Status_CRITICAL,
						Summary: fmt.Sprintf("probe panicked: %v", p),
					}
				}
			}()
			return c.Probe(ctx)
		}()
		st.Id = c.ID
		if st.Status == envorkav1.Status_STATUS_UNSPECIFIED {
			st.Status = envorkav1.Status_UNKNOWN
		}
		out = append(out, &st)
	}
	return out
}

// NotYetDiagnosed is the placeholder probe for components whose real
// diagnostics land in a later milestone.
func NotYetDiagnosed(id string) Probe {
	return func(context.Context) envorkav1.ComponentStatus {
		return envorkav1.ComponentStatus{
			Status:  envorkav1.Status_UNKNOWN,
			Summary: "not yet diagnosed",
		}
	}
}

// WithTimeout bounds a probe with a context deadline so a hung external
// command (wsl.exe, etc.) cannot block a status refresh indefinitely.
func WithTimeout(p Probe, d time.Duration) Probe {
	return func(ctx context.Context) envorkav1.ComponentStatus {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return p(ctx)
	}
}

// OverallStatus aggregates component states: any critical dominates, then
// any warning, then unknown, else healthy.
func OverallStatus(components []*envorkav1.ComponentStatus) envorkav1.Status {
	overall := envorkav1.Status_HEALTHY
	for _, c := range components {
		switch c.Status {
		case envorkav1.Status_CRITICAL:
			return envorkav1.Status_CRITICAL
		case envorkav1.Status_WARNING:
			overall = envorkav1.Status_WARNING
		case envorkav1.Status_UNKNOWN:
			if overall != envorkav1.Status_WARNING {
				overall = envorkav1.Status_UNKNOWN
			}
		}
	}
	return overall
}