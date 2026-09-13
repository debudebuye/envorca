package events

import (
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	b := New(16)
	ch, unsub := b.Subscribe()
	defer unsub()

	b.Publish(Event{Level: "info", Component: "daemon", Name: "started"})
	select {
	case ev := <-ch:
		if ev.Name != "started" {
			t.Errorf("got event %q, want started", ev.Name)
		}
		if ev.Timestamp.IsZero() {
			t.Error("timestamp not set")
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := New(16)
	ch, unsub := b.Subscribe()
	b.Publish(Event{Name: "a"})
	<-ch
	unsub()
	b.Publish(Event{Name: "b"})
	select {
	case ev := <-ch:
		t.Errorf("received %q after unsubscribe", ev.Name)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSnapshotBounded(t *testing.T) {
	b := New(3)
	for i := 0; i < 5; i++ {
		b.Publish(Event{Name: "e"})
	}
	snap := b.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("snapshot length = %d, want 3", len(snap))
	}
	if b.max != 3 {
		t.Errorf("max = %d, want 3", b.max)
	}
}

func TestSnapshotOrderOldestFirst(t *testing.T) {
	b := New(10)
	b.Publish(Event{Name: "first"})
	b.Publish(Event{Name: "second"})
	snap := b.Snapshot()
	if snap[0].Name != "first" || snap[1].Name != "second" {
		t.Errorf("unexpected order: %+v", snap)
	}
}