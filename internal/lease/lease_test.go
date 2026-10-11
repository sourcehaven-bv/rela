package lease

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestNextToken(t *testing.T) {
	t.Parallel()
	us := uint64(t0.UnixMicro())
	cases := []struct {
		name string
		last uint64
		now  time.Time
		want uint64
	}{
		{"first claim takes now", 0, t0, us},
		{"counter behind the clock takes now", us - 10, t0, us},
		{"counter ahead of the clock counts on", us + 5, t0, us + 6},
		{"same instant counts on", us, t0, us + 1},
		{"before the epoch counts from zero", 0, time.Unix(-5, 0), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, NextToken(tc.last, tc.now))
		})
	}
}

func TestCheckClaim(t *testing.T) {
	t.Parallel()
	h := Holder{Identity: "alice"}
	require.NoError(t, CheckClaim("TKT-1", h, "hash", t0, t0.Add(time.Second)))
	cases := []struct {
		name, id, hash string
		h              Holder
		until          time.Time
	}{
		{"empty id", "", "hash", h, t0.Add(time.Second)},
		{"empty identity", "TKT-1", "hash", Holder{}, t0.Add(time.Second)},
		{"empty hash", "TKT-1", "", h, t0.Add(time.Second)},
		{"expiry at now", "TKT-1", "hash", h, t0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, CheckClaim(tc.id, tc.h, tc.hash, t0, tc.until), ErrInvalid)
		})
	}
}

func TestLease_Live(t *testing.T) {
	t.Parallel()
	l := Lease{Expires: t0}
	require.True(t, l.Live(t0.Add(-time.Nanosecond)))
	require.False(t, l.Live(t0), "a lease is not live at its expiry instant")
}

func TestHeldError_NamesEntityAndExpiryOnly(t *testing.T) {
	t.Parallel()
	err := &HeldError{Current: Lease{ID: "TKT-1", Expires: t0, Status: "secret note"}}
	require.Contains(t, err.Error(), "TKT-1")
	require.Contains(t, err.Error(), "2026-10-01T12:00:00Z")
	require.NotContains(t, err.Error(), "secret note")
}
