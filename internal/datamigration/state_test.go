package datamigration

import (
	"errors"
	"testing"
	"time"
)

func TestState_WithApplied(t *testing.T) {
	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)
	proj := metaV1().ShapeProjection()

	s, err := (*State)(nil).WithApplied("20260919143022-first.yaml", proj, now)
	if err != nil {
		t.Fatalf("WithApplied on nil state: %v", err)
	}
	if !s.HasApplied("20260919143022-first.yaml") {
		t.Fatal("WithApplied must record the name")
	}
	if s.FormatVersion != StateFormatVersion {
		t.Errorf("FormatVersion = %d, want %d", s.FormatVersion, StateFormatVersion)
	}

	s2, err := s.WithApplied("20260920090000-second.yaml", proj, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("WithApplied: %v", err)
	}
	if len(s2.Applied) != 2 {
		t.Fatalf("Applied has %d entries, want 2", len(s2.Applied))
	}
	// The receiver must not be mutated: callers hold the pre-write state while
	// deciding whether the write succeeded.
	if len(s.Applied) != 1 {
		t.Error("WithApplied must not mutate its receiver")
	}
}

// Re-running a migration is the documented crash-recovery path, so recording
// the same name twice must not make the record claim two runs.
func TestState_WithApplied_IsIdempotentPerName(t *testing.T) {
	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)
	proj := metaV1().ShapeProjection()

	s, err := (*State)(nil).WithApplied("20260919143022-first.yaml", proj, now)
	if err != nil {
		t.Fatalf("WithApplied: %v", err)
	}
	again, err := s.WithApplied("20260919143022-first.yaml", proj, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("WithApplied: %v", err)
	}
	if len(again.Applied) != 1 {
		t.Fatalf("re-recording a name must not append, got %d entries", len(again.Applied))
	}
	if !again.Applied[0].AppliedAt.Equal(now) {
		t.Error("the first run's timestamp should be kept, not overwritten by the re-run")
	}
}

func TestState_HasAppliedAndNames_NilSafe(t *testing.T) {
	var s *State
	if s.HasApplied("anything") {
		t.Error("a nil state has applied nothing")
	}
	if got := s.AppliedNames(); got != nil {
		t.Errorf("AppliedNames on nil = %v, want nil", got)
	}
}

func TestValidateState(t *testing.T) {
	proj := metaV1().ShapeProjection()
	good, err := NewState(proj, []AppliedEntry{{Name: "20260919143022-ok.yaml"}}, time.Now())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	if err := ValidateState(good); err != nil {
		t.Errorf("a well-formed state must validate, got: %v", err)
	}
	if err := ValidateState(nil); err != nil {
		t.Errorf("a nil state is the un-bootstrapped case, not an error, got: %v", err)
	}

	t.Run("rejects a bad name", func(t *testing.T) {
		bad, err := NewState(proj, []AppliedEntry{{Name: "../escape"}}, time.Now())
		if err != nil {
			t.Fatalf("NewState: %v", err)
		}
		if err := ValidateState(bad); err == nil {
			t.Error("a name outside the allowlist must be refused")
		}
	})

	t.Run("rejects a missing projection", func(t *testing.T) {
		if err := ValidateState(&State{FormatVersion: StateFormatVersion}); err == nil {
			t.Error("state without a projection must be refused: gen and the gate both need it")
		}
	})

	t.Run("refuses a future format version", func(t *testing.T) {
		future := *good
		future.FormatVersion = StateFormatVersion + 1
		err := ValidateState(&future)
		var fromFuture *StateFromFutureError
		if !errors.As(err, &fromFuture) {
			t.Fatalf("want StateFromFutureError, got %v", err)
		}
		if fromFuture.Found != StateFormatVersion+1 {
			t.Errorf("Found = %d, want %d", fromFuture.Found, StateFormatVersion+1)
		}
	})

	t.Run("accepts an older format version", func(t *testing.T) {
		older := *good
		older.FormatVersion = StateFormatVersion - 1
		if err := ValidateState(&older); err != nil {
			t.Errorf("an older format must still be readable (that is the point of upgrading), got: %v", err)
		}
	})
}

// The projection must survive a JSON round trip with its hash intact — it is
// re-hashed to identify the shape, so any re-encoding would make an unchanged
// store look changed.
func TestState_ProjectionRoundTripsWithStableHash(t *testing.T) {
	proj := metaV1().ShapeProjection()
	s, err := NewState(proj, nil, time.Now())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	got, err := s.ShapeProjection()
	if err != nil {
		t.Fatalf("ShapeProjection: %v", err)
	}
	if got.Hash() != proj.Hash() {
		t.Errorf("hash changed through State: %s != %s", got.Hash(), proj.Hash())
	}
}
