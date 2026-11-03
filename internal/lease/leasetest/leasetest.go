// Package leasetest is the conformance harness every [lease.Store]
// implementation must pass.
//
// Two backends without a shared contract test are two subtly different
// behaviors. The rules most likely to drift are lazy expiry, the prune on
// claim, the caps counting live leases only, the token staying strictly
// increasing, when the version moves, and time precision, because a Go map
// and a SQL table make different things easy.
//
// Every assertion is deterministic. The suite drives a fake clock: each
// instant is passed in as `now`. Most instants are whole minutes; the
// precision tests use sub-second instants on purpose, to pin that every
// backend normalizes times with [lease.Instant].
package leasetest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/lease"
)

// Factory returns a fresh, empty store for one subtest. Cleanup is the
// factory's responsibility, normally via t.Cleanup.
type Factory func(t *testing.T) lease.Store

// base is an arbitrary fixed instant.
var base = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// at returns base plus n minutes.
func at(n int) time.Time { return base.Add(time.Duration(n) * time.Minute) }

var (
	alice = lease.Holder{Identity: "alice", User: "Alice", Tool: "mcp", Label: "agent-1"}
	bob   = lease.Holder{Identity: "bob", User: "Bob", Tool: "web"}
)

// Hash returns a well-formed lease hash derived from name, as the service
// derives one from a lease id.
func Hash(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

// public is l as [lease.Store.Get] returns it: without its lease hash.
func public(l lease.Lease) lease.Lease {
	l.LeaseHash = ""
	return l
}

// claim takes a lease that must succeed.
func claim(
	ctx context.Context, t *testing.T, s lease.Store, id string, h lease.Holder, hash string, now, until time.Time,
) lease.Lease {
	t.Helper()
	l, err := s.Claim(ctx, id, h, Hash(hash), now, until, "")
	require.NoError(t, err)
	return l
}

// requireHeld asserts err is a conflict naming holder and returns the current lease.
func requireHeld(t *testing.T, err error, holder lease.Holder) lease.Lease {
	t.Helper()
	var held *lease.HeldError
	require.ErrorAs(t, err, &held)
	require.Equal(t, holder, held.Current.Holder)
	require.Empty(t, held.Current.LeaseHash, "a conflict must not hand out the holder's lease hash")
	return held.Current
}

func version(ctx context.Context, t *testing.T, s lease.Store) uint64 {
	t.Helper()
	v, err := s.Version(ctx)
	require.NoError(t, err)
	return v
}

// get returns the live lease on id at now, and whether there is one.
func get(ctx context.Context, t *testing.T, s lease.Store, id string, now time.Time) (lease.Lease, bool) {
	t.Helper()
	got, err := s.Get(ctx, []string{id}, now)
	require.NoError(t, err)
	l, ok := got[id]
	return l, ok
}

// RunAll runs every conformance test against stores from f.
func RunAll(t *testing.T, f Factory) {
	t.Helper()
	t.Run("Claim", func(t *testing.T) { RunClaimTests(t, f) })
	t.Run("Input", func(t *testing.T) { RunInputTests(t, f) })
	t.Run("Renew", func(t *testing.T) { RunRenewTests(t, f) })
	t.Run("Lookup", func(t *testing.T) { RunLookupTests(t, f) })
	t.Run("Release", func(t *testing.T) { RunReleaseTests(t, f) })
	t.Run("Expiry", func(t *testing.T) { RunExpiryTests(t, f) })
	t.Run("Precision", func(t *testing.T) { RunPrecisionTests(t, f) })
	t.Run("Lifecycle", func(t *testing.T) { RunLifecycleTests(t, f) })
	t.Run("Drop", func(t *testing.T) { RunDropTests(t, f) })
	t.Run("Tokens", func(t *testing.T) { RunTokenTests(t, f) })
	t.Run("Caps", func(t *testing.T) { RunCapTests(t, f) })
	t.Run("Version", func(t *testing.T) { RunVersionTests(t, f) })
	t.Run("Concurrency", func(t *testing.T) { RunConcurrencyTests(t, f) })
}

// RunClaimTests covers claim, read back and conflict.
func RunClaimTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("claim, then read it back", func(t *testing.T) {
		s := f(t)
		l, err := s.Claim(ctx, "TKT-1", alice, Hash("h1"), at(0), at(15), "writing tests")
		require.NoError(t, err)
		require.Equal(t, "TKT-1", l.ID)
		require.Equal(t, alice, l.Holder)
		require.Equal(t, Hash("h1"), l.LeaseHash)
		require.Equal(t, "writing tests", l.Status)
		require.Equal(t, at(0), l.Since)
		require.Equal(t, at(15), l.Expires)
		require.NotZero(t, l.Token)

		got, err := s.Get(ctx, []string{"TKT-1", "TKT-2"}, at(1))
		require.NoError(t, err)
		require.Len(t, got, 1, "an id without a lease has no entry")
		require.Equal(t, public(l), got["TKT-1"], "get serves other people, so it carries no lease hash")

		mine, err := s.ListByHolder(ctx, alice.Identity, at(1))
		require.NoError(t, err)
		require.Equal(t, []lease.Lease{l}, mine, "the holder's own list carries the lease hash")
	})

	t.Run("a live lease refuses every other claim", func(t *testing.T) {
		s := f(t)
		first := claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))

		_, err := s.Claim(ctx, "TKT-1", bob, Hash("h2"), at(1), at(16), "")
		cur := requireHeld(t, err, alice)
		require.Equal(t, first.Token, cur.Token)
		require.Equal(t, at(15), cur.Expires)

		_, err = s.Claim(ctx, "TKT-1", alice, Hash("h3"), at(1), at(16), "")
		requireHeld(t, err, alice)

		got, _ := get(ctx, t, s, "TKT-1", at(2))
		require.Equal(t, public(first), got, "a refused claim changes nothing")
	})

	t.Run("a lease hash names one lease", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Claim(ctx, "TKT-2", alice, Hash("h1"), at(0), at(15), "")
		require.ErrorIs(t, err, lease.ErrInvalid)
	})

	t.Run("get with no ids is empty", func(t *testing.T) {
		s := f(t)
		got, err := s.Get(ctx, nil, at(0))
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// RunInputTests covers the input checks every method shares with the
// service in front of it.
func RunInputTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("claim refuses bad input", func(t *testing.T) {
		s := f(t)
		long := lease.Holder{Identity: "alice", Label: strings.Repeat("a", 33)}
		cases := []struct {
			name, id, hash, status string
			h                      lease.Holder
			until                  time.Time
		}{
			{"empty id", "", Hash("h"), "", alice, at(15)},
			{"empty identity", "TKT-1", Hash("h"), "", lease.Holder{User: "x"}, at(15)},
			{"empty hash", "TKT-1", "", "", alice, at(15)},
			{"short hash", "TKT-1", Hash("h")[1:], "", alice, at(15)},
			{"uppercase hash", "TKT-1", strings.ToUpper(Hash("h")), "", alice, at(15)},
			{"non-hex hash", "TKT-1", "g" + Hash("h")[1:], "", alice, at(15)},
			{"label with a space", "TKT-1", Hash("h"), "", lease.Holder{Identity: "alice", Label: "a b"}, at(15)},
			{"label too long", "TKT-1", Hash("h"), "", long, at(15)},
			{"status with a newline", "TKT-1", Hash("h"), "a\nb", alice, at(15)},
			{"status with a carriage return", "TKT-1", Hash("h"), "a\rb", alice, at(15)},
			{"status with a tab", "TKT-1", Hash("h"), "a\tb", alice, at(15)},
			{"status with a bidi override", "TKT-1", Hash("h"), "a\u202Eb", alice, at(15)},
			{"status with a bidi isolate", "TKT-1", Hash("h"), "a\u2066b", alice, at(15)},
			{"status with a bidi mark", "TKT-1", Hash("h"), "a\u200Fb", alice, at(15)},
			{"status too long", "TKT-1", Hash("h"), strings.Repeat("é", lease.MaxStatusRunes+1), alice, at(15)},
			{"status not UTF-8", "TKT-1", Hash("h"), "a\xffb", alice, at(15)},
			{"expiry at now", "TKT-1", Hash("h"), "", alice, at(0)},
			{"expiry before now", "TKT-1", Hash("h"), "", alice, at(-1)},
			{"expiry within the same microsecond", "TKT-1", Hash("h"), "", alice, at(0).Add(500 * time.Nanosecond)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := s.Claim(ctx, tc.id, tc.h, tc.hash, at(0), tc.until, tc.status)
				require.ErrorIs(t, err, lease.ErrInvalid)
			})
		}
		_, ok := get(ctx, t, s, "TKT-1", at(0))
		require.False(t, ok, "a refused claim stores nothing")
	})

	t.Run("claim takes the longest status and label", func(t *testing.T) {
		s := f(t)
		status := strings.Repeat("é", lease.MaxStatusRunes)
		h := lease.Holder{Identity: "alice", Label: strings.Repeat("a", 32)}
		l, err := s.Claim(ctx, "TKT-1", h, Hash("h"), at(0), at(15), status)
		require.NoError(t, err)
		require.Equal(t, status, l.Status)
	})

	t.Run("renew, lookup and release refuse a malformed hash", func(t *testing.T) {
		s := f(t)
		_, err := s.Renew(ctx, "nope", alice.Identity, at(0), at(15), nil)
		require.ErrorIs(t, err, lease.ErrInvalid)
		_, err = s.Lookup(ctx, "", alice.Identity, at(0))
		require.ErrorIs(t, err, lease.ErrInvalid)
		require.ErrorIs(t, s.Release(ctx, strings.ToUpper(Hash("h")), alice.Identity, at(0)), lease.ErrInvalid)
	})

	t.Run("renew refuses a bad status and keeps the old one", func(t *testing.T) {
		s := f(t)
		_, err := s.Claim(ctx, "TKT-1", alice, Hash("h1"), at(0), at(15), "start")
		require.NoError(t, err)
		bad := "line\nbreak"
		_, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(1), at(20), &bad)
		require.ErrorIs(t, err, lease.ErrInvalid)
		got, _ := get(ctx, t, s, "TKT-1", at(1))
		require.Equal(t, "start", got.Status)
		require.Equal(t, at(15), got.Expires)
	})
}

// RunRenewTests covers renew and the cases that lose a lease.
func RunRenewTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("renew moves the expiry and keeps the token", func(t *testing.T) {
		s := f(t)
		first, err := s.Claim(ctx, "TKT-1", alice, Hash("h1"), at(0), at(15), "start")
		require.NoError(t, err)

		l, err := s.Renew(ctx, Hash("h1"), alice.Identity, at(10), at(25), nil)
		require.NoError(t, err)
		require.Equal(t, at(25), l.Expires)
		require.Equal(t, first.Token, l.Token)
		require.Equal(t, at(0), l.Since, "renew is not a new claim")
		require.Equal(t, "start", l.Status, "a nil status keeps the old one")
		require.Equal(t, Hash("h1"), l.LeaseHash)

		status := "halfway"
		l, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(20), at(35), &status)
		require.NoError(t, err)
		require.Equal(t, "halfway", l.Status)

		got, _ := get(ctx, t, s, "TKT-1", at(30))
		require.Equal(t, public(l), got, "the renewed lease is live past its first expiry")
	})

	t.Run("another identity cannot renew", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Renew(ctx, Hash("h1"), bob.Identity, at(1), at(30), nil)
		require.ErrorIs(t, err, lease.ErrLost)

		got, _ := get(ctx, t, s, "TKT-1", at(1))
		require.Equal(t, at(15), got.Expires, "a refused renew changes nothing")
	})

	t.Run("an unknown hash is lost", func(t *testing.T) {
		s := f(t)
		_, err := s.Renew(ctx, Hash("nope"), alice.Identity, at(0), at(15), nil)
		require.ErrorIs(t, err, lease.ErrLost)
	})

	t.Run("renew after expiry is lost", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Renew(ctx, Hash("h1"), alice.Identity, at(15), at(30), nil)
		require.ErrorIs(t, err, lease.ErrLost, "a lease is not live at its expiry instant")
	})

	t.Run("renew after release is lost", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(1)))
		_, err := s.Renew(ctx, Hash("h1"), alice.Identity, at(2), at(17), nil)
		require.ErrorIs(t, err, lease.ErrLost)
	})

	t.Run("renew to an expiry at or before now is refused", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Renew(ctx, Hash("h1"), alice.Identity, at(5), at(5), nil)
		require.ErrorIs(t, err, lease.ErrInvalid)
	})
}

// RunLookupTests covers lookup, which answers like renew without changing
// anything.
func RunLookupTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("lookup returns the holder's live lease", func(t *testing.T) {
		s := f(t)
		l := claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		got, err := s.Lookup(ctx, Hash("h1"), alice.Identity, at(1))
		require.NoError(t, err)
		require.Equal(t, l, got, "lookup answers the holder, so it carries the lease hash")
	})

	t.Run("lookup is lost for another identity, an expired lease and a released one", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		claim(ctx, t, s, "TKT-2", alice, "h2", at(0), at(15))

		_, err := s.Lookup(ctx, Hash("h1"), bob.Identity, at(1))
		require.ErrorIs(t, err, lease.ErrLost)
		_, err = s.Lookup(ctx, Hash("h1"), alice.Identity, at(15))
		require.ErrorIs(t, err, lease.ErrLost, "a lease is not live at its expiry instant")
		_, err = s.Lookup(ctx, Hash("unknown"), alice.Identity, at(1))
		require.ErrorIs(t, err, lease.ErrLost)

		require.NoError(t, s.Release(ctx, Hash("h2"), alice.Identity, at(1)))
		_, err = s.Lookup(ctx, Hash("h2"), alice.Identity, at(1))
		require.ErrorIs(t, err, lease.ErrLost)
	})
}

// RunReleaseTests covers release, which answers the same for every input.
func RunReleaseTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("release frees the entity", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(1)))

		_, ok := get(ctx, t, s, "TKT-1", at(1))
		require.False(t, ok)
		claim(ctx, t, s, "TKT-1", bob, "h2", at(2), at(17))
	})

	t.Run("release answers nil whether or not the lease existed", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Release(ctx, Hash("unknown"), alice.Identity, at(1)))
		require.NoError(t, s.Release(ctx, Hash("h1"), bob.Identity, at(1)), "another identity gets the same answer")

		_, ok := get(ctx, t, s, "TKT-1", at(1))
		require.True(t, ok, "only the holder's identity releases")

		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(1)))
		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(1)), "a second release is not an error")
	})
}

// RunExpiryTests covers lazy expiry on reads, the prune on claim and the
// next expiry.
func RunExpiryTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("reads leave out expired leases", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))

		_, ok := get(ctx, t, s, "TKT-1", at(15))
		require.False(t, ok, "a lease is not live at its expiry instant")
		mine, err := s.ListByHolder(ctx, alice.Identity, at(15))
		require.NoError(t, err)
		require.Empty(t, mine)

		_, ok = get(ctx, t, s, "TKT-1", at(14))
		require.True(t, ok, "reads do not remove anything")
	})

	t.Run("an expired lease can be claimed by someone else", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		l := claim(ctx, t, s, "TKT-1", bob, "h2", at(15), at(30))
		require.Equal(t, bob, l.Holder)
	})

	t.Run("an expired lease's hash is free for a new claim", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		claim(ctx, t, s, "TKT-2", alice, "h1", at(20), at(30))
	})

	t.Run("list by holder is oldest claim first", func(t *testing.T) {
		s := f(t)
		b := claim(ctx, t, s, "TKT-B", alice, "hb", at(2), at(30))
		a := claim(ctx, t, s, "TKT-A", alice, "ha", at(3), at(30))
		c := claim(ctx, t, s, "TKT-C", alice, "hc", at(1), at(30))
		claim(ctx, t, s, "TKT-D", bob, "hd", at(1), at(30))

		mine, err := s.ListByHolder(ctx, alice.Identity, at(4))
		require.NoError(t, err)
		require.Equal(t, []lease.Lease{c, b, a}, mine)
	})

	t.Run("next expiry is the earliest live expiry", func(t *testing.T) {
		s := f(t)
		_, ok, err := s.NextExpiry(ctx, at(0))
		require.NoError(t, err)
		require.False(t, ok, "an empty store has no next expiry")

		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(30))
		claim(ctx, t, s, "TKT-2", bob, "h2", at(0), at(15))
		claim(ctx, t, s, "TKT-3", bob, "h3", at(0), at(45))

		next, ok, err := s.NextExpiry(ctx, at(1))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, at(15), next)

		next, ok, err = s.NextExpiry(ctx, at(15))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, at(30), next, "an expiry at now is past")

		_, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(16), at(60), nil)
		require.NoError(t, err)
		next, _, err = s.NextExpiry(ctx, at(16))
		require.NoError(t, err)
		require.Equal(t, at(45), next, "a renew moves the next expiry")

		_, ok, err = s.NextExpiry(ctx, at(60))
		require.NoError(t, err)
		require.False(t, ok, "no lease is live after the last expiry")
	})
}

// RunPrecisionTests covers sub-second instants: every backend normalizes
// with [lease.Instant], so all of them store, compare and return equal
// values.
func RunPrecisionTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("sub-second instants come back at microsecond precision", func(t *testing.T) {
		s := f(t)
		now := base.Add(123_456_789 * time.Nanosecond).In(time.FixedZone("CEST", 2*60*60))
		until := at(15).Add(987_654_321 * time.Nanosecond)
		l, err := s.Claim(ctx, "TKT-1", alice, Hash("h1"), now, until, "")
		require.NoError(t, err)
		require.Equal(t, lease.Instant(now), l.Since)
		require.Equal(t, lease.Instant(until), l.Expires)

		got, ok := get(ctx, t, s, "TKT-1", now)
		require.True(t, ok)
		require.Equal(t, public(l), got)
		found, err := s.Lookup(ctx, Hash("h1"), alice.Identity, now)
		require.NoError(t, err)
		require.Equal(t, l, found)
		next, _, err := s.NextExpiry(ctx, now)
		require.NoError(t, err)
		require.Equal(t, lease.Instant(until), next)

		renewTo := at(20).Add(555_555_555 * time.Nanosecond)
		renewed, err := s.Renew(ctx, Hash("h1"), alice.Identity, now.Add(time.Second), renewTo, nil)
		require.NoError(t, err)
		require.Equal(t, lease.Instant(renewTo), renewed.Expires)
	})

	t.Run("liveness is decided at microsecond precision", func(t *testing.T) {
		s := f(t)
		until := at(15).Add(987_654_321 * time.Nanosecond)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), until)

		_, ok := get(ctx, t, s, "TKT-1", until.Add(-500*time.Nanosecond))
		require.True(t, ok, "an instant in the microsecond before the expiry is live")
		_, ok = get(ctx, t, s, "TKT-1", until.Add(-100*time.Nanosecond))
		require.False(t, ok, "an instant in the expiry's own microsecond is the expiry")
		_, err := s.Lookup(ctx, Hash("h1"), alice.Identity, until.Add(-100*time.Nanosecond))
		require.ErrorIs(t, err, lease.ErrLost)
	})
}

// RunLifecycleTests covers rekey after an entity rename.
func RunLifecycleTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("rekey moves a live lease, which stays renewable", func(t *testing.T) {
		s := f(t)
		l := claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(15))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW", at(1)))

		got, err := s.Get(ctx, []string{"TKT-OLD", "TKT-NEW"}, at(1))
		require.NoError(t, err)
		require.NotContains(t, got, "TKT-OLD")
		moved := got["TKT-NEW"]
		require.Equal(t, l.Holder, moved.Holder)
		require.Equal(t, l.Since, moved.Since)
		require.Equal(t, l.Expires, moved.Expires)
		require.Greater(t, moved.Token, l.Token, "a moved lease takes a fresh token")

		renewed, err := s.Renew(ctx, Hash("h1"), alice.Identity, at(2), at(20), nil)
		require.NoError(t, err)
		require.Equal(t, "TKT-NEW", renewed.ID)
		require.Equal(t, moved.Token, renewed.Token)
	})

	t.Run("a moved lease's token is above every earlier lease on the new id", func(t *testing.T) {
		s := f(t)
		old := claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(30))
		earlier := claim(ctx, t, s, "TKT-NEW", bob, "h2", at(0), at(30))
		require.Greater(t, earlier.Token, old.Token)
		require.NoError(t, s.Release(ctx, Hash("h2"), bob.Identity, at(1)))

		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW", at(1)))
		got, ok := get(ctx, t, s, "TKT-NEW", at(1))
		require.True(t, ok)
		require.Greater(t, got.Token, earlier.Token, "tokens must increase per entity")
	})

	t.Run("rekey replaces an expired lease on the new id", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-NEW", bob, "h2", at(0), at(5))
		claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(30))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW", at(10)))

		got, ok := get(ctx, t, s, "TKT-NEW", at(10))
		require.True(t, ok)
		require.Equal(t, alice, got.Holder)
		_, err := s.Lookup(ctx, Hash("h1"), alice.Identity, at(10))
		require.NoError(t, err)
	})

	t.Run("rekey keeps a live lease on the new id and drops the old one", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(15))
		kept := claim(ctx, t, s, "TKT-NEW", bob, "h2", at(0), at(15))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW", at(1)))

		got, err := s.Get(ctx, []string{"TKT-OLD", "TKT-NEW"}, at(1))
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, public(kept), got["TKT-NEW"])
		_, err = s.Lookup(ctx, Hash("h1"), alice.Identity, at(1))
		require.ErrorIs(t, err, lease.ErrLost, "the dropped lease is lost")
	})

	t.Run("rekey drops an expired lease on the old id", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(5))
		kept := claim(ctx, t, s, "TKT-OTHER", bob, "h2", at(0), at(30))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW", at(10)))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-OTHER", at(10)))

		got, err := s.Get(ctx, []string{"TKT-OLD", "TKT-NEW", "TKT-OTHER"}, at(0))
		require.NoError(t, err)
		require.Equal(t, map[string]lease.Lease{"TKT-OTHER": public(kept)}, got,
			"read at an instant where the old lease would be live: it is gone, and nothing moved")
	})

	t.Run("rekey without a lease, or onto itself, changes nothing", func(t *testing.T) {
		s := f(t)
		kept := claim(ctx, t, s, "TKT-NEW", bob, "h2", at(0), at(15))
		require.NoError(t, s.Rekey(ctx, "TKT-NONE", "TKT-NEW", at(1)))
		require.NoError(t, s.Rekey(ctx, "TKT-NEW", "TKT-NEW", at(1)))

		got, _ := get(ctx, t, s, "TKT-NEW", at(1))
		require.Equal(t, public(kept), got, "a rekey from an id without a lease must not drop the target's")
	})

	t.Run("rekey refuses an empty id", func(t *testing.T) {
		s := f(t)
		require.ErrorIs(t, s.Rekey(ctx, "", "TKT-1", at(0)), lease.ErrInvalid)
		require.ErrorIs(t, s.Rekey(ctx, "TKT-1", "", at(0)), lease.ErrInvalid)
	})
}

// RunDropTests covers drop after an entity delete.
func RunDropTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("drop removes the lease, live or expired", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		claim(ctx, t, s, "TKT-2", alice, "h2", at(0), at(5))
		require.NoError(t, s.Drop(ctx, "TKT-1", at(1)))
		require.NoError(t, s.Drop(ctx, "TKT-1", at(1)), "dropping a missing lease is not an error")
		require.NoError(t, s.Drop(ctx, "TKT-2", at(10)))

		got, err := s.Get(ctx, []string{"TKT-1", "TKT-2"}, at(1))
		require.NoError(t, err)
		require.Empty(t, got)
		_, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(1), at(20), nil)
		require.ErrorIs(t, err, lease.ErrLost)
	})
}

// RunTokenTests covers the store-wide token.
func RunTokenTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("tokens are at least now in microseconds", func(t *testing.T) {
		s := f(t)
		l := claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.GreaterOrEqual(t, l.Token, uint64(at(0).UnixMicro()))
	})

	t.Run("tokens increase across claims, release and a clock that goes back", func(t *testing.T) {
		s := f(t)
		t1 := claim(ctx, t, s, "TKT-1", alice, "h1", at(10), at(60)).Token
		t2 := claim(ctx, t, s, "TKT-2", alice, "h2", at(10), at(60)).Token
		require.Greater(t, t2, t1, "two claims at one instant")

		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(11)))
		t3 := claim(ctx, t, s, "TKT-1", alice, "h3", at(11), at(60)).Token
		require.Greater(t, t3, t2, "a claim after a release")

		t4 := claim(ctx, t, s, "TKT-3", alice, "h4", at(-100), at(60)).Token
		require.Greater(t, t4, t3, "a claim on a clock that went back")

		require.NoError(t, s.Release(ctx, Hash("h3"), alice.Identity, at(-100)))
		t5 := claim(ctx, t, s, "TKT-1", alice, "h5", at(-100), at(60)).Token
		require.Greater(t, t5, t4, "a re-claim on a clock that went back")
	})
}

// RunCapTests covers the per-identity cap. The store-wide cap needs
// [lease.MaxPerStore] rows, so each backend seeds its own test of it.
func RunCapTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("an identity holds at most MaxPerIdentity live leases", func(t *testing.T) {
		s := f(t)
		for i := range lease.MaxPerIdentity {
			until := at(60)
			if i == 0 {
				until = at(15)
			}
			claim(ctx, t, s, fmt.Sprintf("TKT-%d", i), alice, fmt.Sprintf("h%d", i), at(0), until)
		}
		_, err := s.Claim(ctx, "TKT-X", alice, Hash("hx"), at(1), at(60), "")
		require.ErrorIs(t, err, lease.ErrLimit)

		claim(ctx, t, s, "TKT-B", bob, "hb", at(1), at(60))

		// Once one of alice's leases expires, she may claim again.
		claim(ctx, t, s, "TKT-X", alice, "hx", at(15), at(60))
	})
}

// RunVersionTests covers the version watchers poll: it moves exactly when
// what readers see changes (see [lease.Store]).
func RunVersionTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("every visible change moves the version and nothing else does", func(t *testing.T) {
		s := f(t)
		v := version(ctx, t, s)
		changed := func(what string) {
			t.Helper()
			next := version(ctx, t, s)
			require.NotEqual(t, v, next, "%s must move the version", what)
			v = next
		}
		unchanged := func(what string) {
			t.Helper()
			require.Equal(t, v, version(ctx, t, s), "%s must not move the version", what)
		}

		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		changed("claim")
		_, err := s.Renew(ctx, Hash("h1"), alice.Identity, at(1), at(20), nil)
		require.NoError(t, err)
		unchanged("a renew that only extends")
		same := ""
		_, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(1), at(21), &same)
		require.NoError(t, err)
		unchanged("a renew that extends and repeats the status")
		got, ok := get(ctx, t, s, "TKT-1", at(1))
		require.True(t, ok)
		require.Equal(t, at(21), got.Expires, "a renew that moves no version is still stored")
		status := "reviewing"
		_, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(1), at(22), &status)
		require.NoError(t, err)
		changed("a renew that changes the status")
		_, err = s.Renew(ctx, Hash("h1"), alice.Identity, at(1), at(18), nil)
		require.NoError(t, err)
		changed("a renew that shortens the lease")
		require.NoError(t, s.Rekey(ctx, "TKT-1", "TKT-2", at(2)))
		changed("rekey of a live lease")
		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(2)))
		changed("release of a live lease")
		claim(ctx, t, s, "TKT-3", alice, "h3", at(3), at(20))
		changed("claim")
		require.NoError(t, s.Drop(ctx, "TKT-3", at(3)))
		changed("drop of a live lease")

		claim(ctx, t, s, "TKT-4", alice, "h4", at(3), at(10))
		changed("claim")
		_, err = s.Claim(ctx, "TKT-4", bob, Hash("h5"), at(4), at(20), "")
		requireHeld(t, err, alice)
		unchanged("a claim refused as held")
		_, err = s.Claim(ctx, "TKT-5", bob, Hash("h4"), at(4), at(20), "")
		require.ErrorIs(t, err, lease.ErrInvalid)
		unchanged("a claim refused for its hash")
		_, err = s.Renew(ctx, Hash("unknown"), alice.Identity, at(4), at(20), nil)
		require.ErrorIs(t, err, lease.ErrLost)
		unchanged("a lost renew")

		require.NoError(t, s.Release(ctx, Hash("unknown"), alice.Identity, at(4)))
		unchanged("a release of nothing")
		require.NoError(t, s.Drop(ctx, "TKT-NONE", at(4)))
		unchanged("a drop of nothing")
		_, err = s.Get(ctx, []string{"TKT-4"}, at(4))
		require.NoError(t, err)
		_, err = s.Lookup(ctx, Hash("h4"), alice.Identity, at(4))
		require.NoError(t, err)
		_, _, err = s.NextExpiry(ctx, at(4))
		require.NoError(t, err)
		unchanged("a read")
	})

	t.Run("writes that only touch expired leases leave the version", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(5))
		claim(ctx, t, s, "TKT-2", alice, "h2", at(0), at(5))
		claim(ctx, t, s, "TKT-3", alice, "h3", at(0), at(5))
		claim(ctx, t, s, "TKT-4", alice, "h4", at(0), at(5))
		v := version(ctx, t, s)

		require.NoError(t, s.Release(ctx, Hash("h1"), alice.Identity, at(10)))
		require.Equal(t, v, version(ctx, t, s), "a release of an expired lease (a prune) must not move the version")
		require.NoError(t, s.Release(ctx, Hash("unknown"), alice.Identity, at(10)))
		require.Equal(t, v, version(ctx, t, s), "a release that only prunes must not move the version")
		require.NoError(t, s.Drop(ctx, "TKT-2", at(10)))
		require.Equal(t, v, version(ctx, t, s), "a drop of an expired lease must not move the version")
		require.NoError(t, s.Rekey(ctx, "TKT-3", "TKT-9", at(10)))
		require.Equal(t, v, version(ctx, t, s), "a rekey of an expired lease must not move the version")
		_, err := s.Renew(ctx, Hash("h4"), alice.Identity, at(10), at(20), nil)
		require.ErrorIs(t, err, lease.ErrLost)
		require.Equal(t, v, version(ctx, t, s), "a renew refused after expiry must not move the version")
	})
}

// RunConcurrencyTests covers concurrent writes: one winner per entity, the
// caps, and every kind of write racing claims without an error.
func RunConcurrencyTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("of 100 concurrent claims exactly one wins", func(t *testing.T) {
		s := f(t)
		const n = 100
		var (
			wg     sync.WaitGroup
			mu     sync.Mutex
			wins   int
			others []error
		)
		for i := range n {
			wg.Go(func() {
				h := lease.Holder{Identity: fmt.Sprintf("user-%d", i), User: "u", Tool: "t"}
				_, err := s.Claim(ctx, "TKT-HOT", h, Hash(fmt.Sprintf("hash-%d", i)), at(0), at(15), "")
				mu.Lock()
				defer mu.Unlock()
				var held *lease.HeldError
				switch {
				case err == nil:
					wins++
				case errors.As(err, &held):
				default:
					others = append(others, err)
				}
			})
		}
		wg.Wait()
		require.Empty(t, others, "a losing claim must report the conflict, nothing else")
		require.Equal(t, 1, wins)
	})

	t.Run("of 100 concurrent claims by one identity exactly MaxPerIdentity win", func(t *testing.T) {
		s := f(t)
		const n = 100
		var (
			wg     sync.WaitGroup
			mu     sync.Mutex
			wins   int
			others []error
		)
		for i := range n {
			wg.Go(func() {
				_, err := s.Claim(ctx, fmt.Sprintf("TKT-%d", i), alice, Hash(fmt.Sprintf("hash-%d", i)), at(0), at(15), "")
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					wins++
				case errors.Is(err, lease.ErrLimit):
				default:
					others = append(others, err)
				}
			})
		}
		wg.Wait()
		require.Empty(t, others, "a claim past the cap must report the limit, nothing else")
		require.Equal(t, lease.MaxPerIdentity, wins)
		mine, err := s.ListByHolder(ctx, alice.Identity, at(1))
		require.NoError(t, err)
		require.Len(t, mine, lease.MaxPerIdentity)
	})

	t.Run("every write races claims on live and expired leases without an error", func(t *testing.T) {
		for round := range 3 {
			t.Run(fmt.Sprintf("round %d", round), func(t *testing.T) { runMixedWrites(ctx, t, f(t)) })
		}
	})
}

// runMixedWrites seeds live and expired leases, runs every write against
// them at once, and then checks the invariants a reader relies on.
func runMixedWrites(ctx context.Context, t *testing.T, s lease.Store) {
	t.Helper()
	const n = 16
	entity := func(i int) string { return fmt.Sprintf("TKT-%d", i%n) }
	seed := func(i int) lease.Holder { return lease.Holder{Identity: fmt.Sprintf("seed-%d", i)} }
	claimer := func(i int) lease.Holder { return lease.Holder{Identity: fmt.Sprintf("claimer-%d", i)} }
	for i := range n {
		until := at(60)
		if i%2 == 1 {
			until = at(5)
		}
		claim(ctx, t, s, entity(i), seed(i), fmt.Sprintf("seed-%d", i), at(0), until)
	}

	now := at(10)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	run := func(fn func() error) {
		wg.Go(func() {
			err := fn()
			var held *lease.HeldError
			if err == nil || errors.As(err, &held) || errors.Is(err, lease.ErrLost) {
				return
			}
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		})
	}
	for i := range n {
		seedHash := Hash(fmt.Sprintf("seed-%d", i))
		run(func() error {
			_, err := s.Claim(ctx, entity(i), claimer(i), Hash(fmt.Sprintf("claim-%d", i)), now, at(60), "")
			return err
		})
		run(func() error {
			_, err := s.Claim(ctx, fmt.Sprintf("NEW-%d", i), claimer(i), Hash(fmt.Sprintf("new-%d", i)), now, at(60), "")
			return err
		})
		run(func() error { return s.Release(ctx, seedHash, seed(i).Identity, now) })
		run(func() error {
			_, err := s.Renew(ctx, seedHash, seed(i).Identity, now, at(70), nil)
			return err
		})
		run(func() error { return s.Drop(ctx, entity(i+5), now) })
		run(func() error { return s.Rekey(ctx, entity(i), entity(i+3), now) })
	}
	wg.Wait()
	require.Empty(t, errs, "a racing write may only be refused as held or lost")

	ids := make([]string, 0, 2*n)
	var mine []lease.Lease
	for i := range n {
		ids = append(ids, entity(i), fmt.Sprintf("NEW-%d", i))
		for _, h := range []lease.Holder{seed(i), claimer(i)} {
			list, err := s.ListByHolder(ctx, h.Identity, now)
			require.NoError(t, err)
			require.LessOrEqual(t, len(list), lease.MaxPerIdentity)
			mine = append(mine, list...)
		}
	}
	got, err := s.Get(ctx, ids, now)
	require.NoError(t, err)
	require.Len(t, mine, len(got), "every live lease belongs to exactly one holder's list")

	entities, hashes, tokens := map[string]bool{}, map[string]bool{}, map[uint64]bool{}
	for _, l := range mine {
		require.False(t, entities[l.ID], "one live lease per entity: %s", l.ID)
		require.False(t, hashes[l.LeaseHash], "one live lease per hash")
		require.False(t, tokens[l.Token], "one live lease per token")
		entities[l.ID], hashes[l.LeaseHash], tokens[l.Token] = true, true, true
		require.Equal(t, public(l), got[l.ID], "get and the holder's list agree")
	}
}
