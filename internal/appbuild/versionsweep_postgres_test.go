//go:build postgres

package appbuild

import (
	"testing"
	"time"
)

func TestSweepConfigFromEnv(t *testing.T) {
	t.Run("all unset yields zero config (pgstore defaults apply)", func(t *testing.T) {
		t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "")
		t.Setenv("RELA_VERSION_SWEEP_IDLE", "")
		t.Setenv("RELA_VERSION_SWEEP_MAX_STALENESS", "")
		got := sweepConfigFromEnv()
		if got.Interval != 0 || got.Idle != 0 || got.MaxStaleness != 0 {
			t.Fatalf("unset env should yield zero-value config, got %+v", got)
		}
	})

	t.Run("valid durations are parsed", func(t *testing.T) {
		t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "500ms")
		t.Setenv("RELA_VERSION_SWEEP_IDLE", "150ms")
		t.Setenv("RELA_VERSION_SWEEP_MAX_STALENESS", "2s")
		got := sweepConfigFromEnv()
		if got.Interval != 500*time.Millisecond {
			t.Errorf("Interval = %v, want 500ms", got.Interval)
		}
		if got.Idle != 150*time.Millisecond {
			t.Errorf("Idle = %v, want 150ms", got.Idle)
		}
		if got.MaxStaleness != 2*time.Second {
			t.Errorf("MaxStaleness = %v, want 2s", got.MaxStaleness)
		}
	})

	t.Run("invalid duration is ignored (falls back to zero), never fails boot", func(t *testing.T) {
		t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "not-a-duration")
		t.Setenv("RELA_VERSION_SWEEP_IDLE", "")
		t.Setenv("RELA_VERSION_SWEEP_MAX_STALENESS", "")
		got := sweepConfigFromEnv()
		if got.Interval != 0 {
			t.Errorf("invalid Interval should be ignored (0), got %v", got.Interval)
		}
	})
}
