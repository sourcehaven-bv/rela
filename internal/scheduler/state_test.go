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

// TestParseState_oldFileWithoutRetryFields pins backward compatibility with
// state files written before the retry ladder existed (BUG-ZKK2UL): they carry
// only "tasks", and the new maps must be non-nil so the scheduler can write to
// them without panicking.
func TestParseState_oldFileWithoutRetryFields(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	data := []byte(`{"tasks":{"daily":"` + ts.Format(time.RFC3339Nano) + `"}}`)

	s := parseState(data)

	if got := s.Tasks["daily"]; !got.Equal(ts) {
		t.Errorf("Tasks[daily] = %v, want %v", got, ts)
	}
	if s.Failures == nil {
		t.Error("Failures is nil — a write would panic")
	}
	if s.NextRetry == nil {
		t.Error("NextRetry is nil — a write would panic")
	}
	// Writing to the freshly-parsed maps must not panic.
	s.Failures["daily"] = 1
	s.NextRetry["daily"] = ts
}

func TestState_roundTripWithRetryFields(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	retry := ts.Add(5 * time.Minute)
	in := newState()
	in.Tasks["daily"] = ts
	in.Failures["daily"] = 3
	in.NextRetry["daily"] = retry

	data, err := in.marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := parseState(data)

	if got := out.Tasks["daily"]; !got.Equal(ts) {
		t.Errorf("Tasks[daily] = %v, want %v", got, ts)
	}
	if got := out.Failures["daily"]; got != 3 {
		t.Errorf("Failures[daily] = %d, want 3", got)
	}
	if got := out.NextRetry["daily"]; !got.Equal(retry) {
		t.Errorf("NextRetry[daily] = %v, want %v", got, retry)
	}
}
