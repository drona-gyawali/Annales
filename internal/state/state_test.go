package state

import (
	"testing"

	"github.com/drona-gyawali/logStore/internal/log"
)

func TestStateApply(t *testing.T) {
	state := New()

	state.Apply(log.LogEntry{
		Operation: "SET",
		Key:       "user:1",
		Value:     "Alice",
	})

	state.Apply(log.LogEntry{
		Operation: "SET",
		Key:       "user:2",
		Value:     "Bob",
	})

	value, ok := state.Get("user:1")
	if !ok {
		t.Fatal("expected user:1 to exist")
	}

	if value != "Alice" {
		t.Fatalf("expected Alice, got %q", value)
	}

	value, ok = state.Get("user:2")
	if !ok {
		t.Fatal("expected user:2 to exist")
	}

	if value != "Bob" {
		t.Fatalf("expected Bob, got %q", value)
	}
}

func TestStateUpdate(t *testing.T) {
	state := New()

	state.Apply(log.LogEntry{
		Operation: "SET",
		Key:       "user:1",
		Value:     "Alice",
	})

	state.Apply(log.LogEntry{
		Operation: "SET",
		Key:       "user:1",
		Value:     "Charlie",
	})

	value, ok := state.Get("user:1")
	if !ok {
		t.Fatal("expected user:1 to exist")
	}

	if value != "Charlie" {
		t.Fatalf("expected Charlie, got %q", value)
	}
}

func TestStateDelete(t *testing.T) {
	state := New()

	state.Apply(log.LogEntry{
		Operation: "SET",
		Key:       "user:1",
		Value:     "Alice",
	})

	state.Apply(log.LogEntry{
		Operation: "DELETE",
		Key:       "user:1",
	})

	_, ok := state.Get("user:1")
	if ok {
		t.Fatal("expected user:1 to be deleted")
	}
}

func TestStateApplyBatch(t *testing.T) {
	state := New()

	batch := log.LogBatch{
		SequenceID: 1,
		Operations: []log.LogEntry{
			{
				Operation: "SET",
				Key:       "user:1",
				Value:     "Alice",
			},
			{
				Operation: "SET",
				Key:       "user:2",
				Value:     "Bob",
			},
			{
				Operation: "DELETE",
				Key:       "user:2",
			},
		},
	}

	state.ApplyBatch(batch)

	value, ok := state.Get("user:1")
	if !ok || value != "Alice" {
		t.Fatalf("expected user:1=Alice, got %q, exists=%v", value, ok)
	}

	_, ok = state.Get("user:2")
	if ok {
		t.Fatal("expected user:2 to be deleted")
	}
}
