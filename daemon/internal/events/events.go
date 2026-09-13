package events

import (
	"sync"
	"time"
)

// Event is a structured daemon event streamed to subscribers and, later,
// persisted for diagnostics. It follows the structured-logging vocabulary:
// {level, component, event, project?, fields...}.
type Event struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Name      string         `json:"event"`
	Project   string         `json:"project,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// Bus is an in-memory publish/subscribe bus with a bounded replay buffer.
// The daemon is the only publisher; clients subscribe via gRPC streams.
type Bus struct {
	mu     sync.Mutex
	subs   map[chan Event]struct{}
	buffer []Event
	max    int
	seq    uint64
}

// New creates a bus that retains up to max recent events for replay.
func New(max int) *Bus {
	return &Bus{subs: map[chan Event]struct{}{}, max: max}
}

// Publish broadcasts ev to all subscribers and records it in the buffer.
func (b *Bus) Publish(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	b.mu.Lock()
	b.seq++
	b.buffer = append(b.buffer, ev)
	if len(b.buffer) > b.max {
		b.buffer = b.buffer[len(b.buffer)-b.max:]
	}
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	b.mu.Unlock()
}

// Subscribe registers a buffered channel and returns an unsubscribe func.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 64)
	b.subs[ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

// Snapshot returns a copy of the retained events, oldest first.
func (b *Bus) Snapshot() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, len(b.buffer))
	copy(out, b.buffer)
	return out
}