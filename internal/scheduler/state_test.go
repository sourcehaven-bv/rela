package scheduler

import (
	"testing"
	"time"
)

func TestParseState_valid(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	data := []byte(`{"tasks":{"daily":"` + ts.Format(time.RFC3339Nano) + `"}}`)

	state := parseState(data)
	if got := state.Tasks["daily"]; !got.Equal(ts) {
		t.Errorf("tasks[daily] = %v, want %v", got, ts)
	}
}

func TestParseState_corrupted(t *testing.T) {
	t.Parallel()

	state := parseState([]byte("not json"))
	if state.Tasks == nil {
		t.Fatal("expected non-nil map")
	}
	if len(state.Tasks) != 0 {
		t.Errorf("expected empty map, got %d entries", len(state.Tasks))
	}
}

func TestParseState_empty(t *testing.T) {
	t.Parallel()

	state := parseState([]byte("{}"))
	if state.Tasks == nil {
		t.Fatal("expected non-nil map")
	}
}

func TestState_roundTrip(t *testing.T) {
	t.Parallel()

	s := newState()
	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	s.Tasks["daily"] = ts

	data, err := s.marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s2 := parseState(data)
	if got := s2.Tasks["daily"]; !got.Equal(ts) {
		t.Errorf("round-trip: got %v, want %v", got, ts)
	}
}
