package lease

import (
	"strings"
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

// goodHash is a well-formed lease hash.
var goodHash = strings.Repeat("0123456789abcdef", 4)

func TestCheckClaim(t *testing.T) {
	t.Parallel()
	h := Holder{Identity: "alice", Label: "agent-1"}
	require.NoError(t, CheckClaim("TKT-1", h, goodHash, t0, t0.Add(time.Second), "fine"))
	cases := []struct {
		name, id, hash, status string
		h                      Holder
		until                  time.Time
	}{
		{"empty id", "", goodHash, "", h, t0.Add(time.Second)},
		{"empty identity", "TKT-1", goodHash, "", Holder{}, t0.Add(time.Second)},
		{"bad label", "TKT-1", goodHash, "", Holder{Identity: "alice", Label: "a/b"}, t0.Add(time.Second)},
		{"empty hash", "TKT-1", "", "", h, t0.Add(time.Second)},
		{"bad status", "TKT-1", goodHash, "a\nb", h, t0.Add(time.Second)},
		{"expiry at now", "TKT-1", goodHash, "", h, t0},
		{"expiry in the same microsecond", "TKT-1", goodHash, "", h, t0.Add(999 * time.Nanosecond)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, CheckClaim(tc.id, tc.h, tc.hash, t0, tc.until, tc.status), ErrInvalid)
		})
	}
}

func TestCheckRenew(t *testing.T) {
	t.Parallel()
	ok, bad := "halfway", "a\tb"
	require.NoError(t, CheckRenew(goodHash, t0, t0.Add(time.Second), nil))
	require.NoError(t, CheckRenew(goodHash, t0, t0.Add(time.Second), &ok))
	require.ErrorIs(t, CheckRenew(goodHash, t0, t0.Add(time.Second), &bad), ErrInvalid)
	require.ErrorIs(t, CheckRenew("nope", t0, t0.Add(time.Second), nil), ErrInvalid)
	require.ErrorIs(t, CheckRenew(goodHash, t0, t0, nil), ErrInvalid)
}

func TestCheckHash(t *testing.T) {
	t.Parallel()
	require.NoError(t, CheckHash(goodHash))
	for _, h := range []string{"", goodHash[1:], goodHash + "0", strings.ToUpper(goodHash), "g" + goodHash[1:]} {
		require.ErrorIs(t, CheckHash(h), ErrInvalid, "%q", h)
	}
}

func TestCheckLabel(t *testing.T) {
	t.Parallel()
	for _, l := range []string{"", "agent-1", "A.b_c-9", strings.Repeat("x", 32)} {
		require.NoError(t, CheckLabel(l), "%q", l)
	}
	for _, l := range []string{strings.Repeat("x", 33), "a b", "a/b", "é", "a\n"} {
		require.ErrorIs(t, CheckLabel(l), ErrInvalid, "%q", l)
	}
}

func TestCheckStatus(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "writing tests", "日本語", strings.Repeat("é", MaxStatusRunes)} {
		require.NoError(t, CheckStatus(s), "%q", s)
	}
	bad := []string{
		strings.Repeat("é", MaxStatusRunes+1), "a\nb", "a\rb", "a\tb", "a\x00b", "a\x7fb", "a\u0085b",
		"a\u202Ab", "a\u202Eb", "a\u2066b", "a\u2069b", "a\u200Eb", "a\u200Fb", "a\xffb",
	}
	for _, s := range bad {
		require.ErrorIs(t, CheckStatus(s), ErrInvalid, "%q", s)
	}
}

func TestInstant(t *testing.T) {
	t.Parallel()
	in := time.Date(2026, 10, 1, 14, 0, 0, 123_456_789, time.FixedZone("CEST", 2*60*60))
	require.Equal(t, time.Date(2026, 10, 1, 12, 0, 0, 123_456_000, time.UTC), Instant(in))
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
