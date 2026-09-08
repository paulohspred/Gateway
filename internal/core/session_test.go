package core

import (
	"testing"
	"time"
)

func TestSessionRegistryLifecycleAndAccounting(t *testing.T) {
	r := NewSessionRegistry()
	t0 := time.Unix(100, 0).UTC()
	r.Open(Session{ID: "b", ListenerID: "listener-b", OpenedAt: t0, LastSeenAt: t0})
	r.Open(Session{ID: "a", ListenerID: "listener-a", OpenedAt: t0, LastSeenAt: t0})
	if got := r.Count(); got != 2 {
		t.Fatalf("Count()=%d want 2", got)
	}

	t1 := t0.Add(time.Second)
	r.Touch("a", "field_to_consumer", 11, t1)
	r.Touch("a", "consumer_to_field", 7, t1.Add(time.Second))
	r.Touch("a", "unknown", 99, t1.Add(2*time.Second))
	r.Touch("missing", "field_to_consumer", 5, t1)

	snapshot := r.Snapshot()
	if len(snapshot) != 2 || snapshot[0].ID != "a" || snapshot[1].ID != "b" {
		t.Fatalf("Snapshot() not sorted/stable: %#v", snapshot)
	}
	a := snapshot[0]
	if a.BytesFieldToConsumer != 11 || a.BytesConsumerToField != 7 {
		t.Fatalf("unexpected byte accounting: %#v", a)
	}
	if !a.LastSeenAt.Equal(t1.Add(2 * time.Second)) {
		t.Fatalf("LastSeenAt=%v want %v", a.LastSeenAt, t1.Add(2*time.Second))
	}

	r.Close("a")
	r.Close("missing")
	if got := r.Count(); got != 1 {
		t.Fatalf("Count() after Close=%d want 1", got)
	}
}
