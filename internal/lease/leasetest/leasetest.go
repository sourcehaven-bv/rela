// Package leasetest is the conformance harness every [lease.Store]
// implementation must pass.
//
// Two backends without a shared contract test are two subtly different
// behaviors. The rules most likely to drift are lazy expiry, the prune on
// claim, the caps counting live leases only, and the token staying strictly
// increasing, because a Go map and a SQL table make different things easy.
//
// Every assertion is deterministic. The suite drives a fake clock: each
// instant is passed in as `now`, in whole seconds so a database backend's
// microsecond precision cannot make a round-tripped time differ.
package leasetest

import (
	"context"
	"errors"
	"fmt"
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

// claim takes a lease that must succeed.
func claim(
	ctx context.Context, t *testing.T, s lease.Store, id string, h lease.Holder, hash string, now, until time.Time,
) lease.Lease {
	t.Helper()
	l, err := s.Claim(ctx, id, h, hash, now, until, "")
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

// RunAll runs every conformance test against stores from f.
func RunAll(t *testing.T, f Factory) {
	t.Helper()
	t.Run("Claim", func(t *testing.T) { RunClaimTests(t, f) })
	t.Run("Renew", func(t *testing.T) { RunRenewTests(t, f) })
	t.Run("Release", func(t *testing.T) { RunReleaseTests(t, f) })
	t.Run("Expiry", func(t *testing.T) { RunExpiryTests(t, f) })
	t.Run("Lifecycle", func(t *testing.T) { RunLifecycleTests(t, f) })
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
		l, err := s.Claim(ctx, "TKT-1", alice, "h1", at(0), at(15), "writing tests")
		require.NoError(t, err)
		require.Equal(t, "TKT-1", l.ID)
		require.Equal(t, alice, l.Holder)
		require.Equal(t, "h1", l.LeaseHash)
		require.Equal(t, "writing tests", l.Status)
		require.True(t, l.Since.Equal(at(0)))
		require.True(t, l.Expires.Equal(at(15)))
		require.NotZero(t, l.Token)

		got, err := s.Get(ctx, []string{"TKT-1", "TKT-2"}, at(1))
		require.NoError(t, err)
		require.Len(t, got, 1, "an id without a lease has no entry")
		require.Equal(t, l, got["TKT-1"])

		mine, err := s.ListByHolder(ctx, alice.Identity, at(1))
		require.NoError(t, err)
		require.Equal(t, []lease.Lease{l}, mine)
	})

	t.Run("a live lease refuses every other claim", func(t *testing.T) {
		s := f(t)
		first := claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))

		_, err := s.Claim(ctx, "TKT-1", bob, "h2", at(1), at(16), "")
		cur := requireHeld(t, err, alice)
		require.Equal(t, first.Token, cur.Token)
		require.True(t, cur.Expires.Equal(at(15)))

		_, err = s.Claim(ctx, "TKT-1", alice, "h3", at(1), at(16), "")
		requireHeld(t, err, alice)

		got, err := s.Get(ctx, []string{"TKT-1"}, at(2))
		require.NoError(t, err)
		require.Equal(t, first, got["TKT-1"], "a refused claim changes nothing")
	})

	t.Run("a lease hash names one lease", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Claim(ctx, "TKT-2", alice, "h1", at(0), at(15), "")
		require.ErrorIs(t, err, lease.ErrInvalid)
	})

	t.Run("bad input is refused", func(t *testing.T) {
		s := f(t)
		cases := []struct {
			name, id, hash string
			h              lease.Holder
			until          time.Time
		}{
			{"empty id", "", "h", alice, at(15)},
			{"empty identity", "TKT-1", "h", lease.Holder{User: "x"}, at(15)},
			{"empty hash", "TKT-1", "", alice, at(15)},
			{"expiry at now", "TKT-1", "h", alice, at(0)},
			{"expiry before now", "TKT-1", "h", alice, at(-1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := s.Claim(ctx, tc.id, tc.h, tc.hash, at(0), tc.until, "")
				require.ErrorIs(t, err, lease.ErrInvalid)
			})
		}
	})

	t.Run("get with no ids is empty", func(t *testing.T) {
		s := f(t)
		got, err := s.Get(ctx, nil, at(0))
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// RunRenewTests covers renew and the cases that lose a lease.
func RunRenewTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("renew moves the expiry and keeps the token", func(t *testing.T) {
		s := f(t)
		first, err := s.Claim(ctx, "TKT-1", alice, "h1", at(0), at(15), "start")
		require.NoError(t, err)

		l, err := s.Renew(ctx, "h1", alice.Identity, at(10), at(25), nil)
		require.NoError(t, err)
		require.True(t, l.Expires.Equal(at(25)))
		require.Equal(t, first.Token, l.Token)
		require.True(t, l.Since.Equal(at(0)), "renew is not a new claim")
		require.Equal(t, "start", l.Status, "a nil status keeps the old one")

		status := "halfway"
		l, err = s.Renew(ctx, "h1", alice.Identity, at(20), at(35), &status)
		require.NoError(t, err)
		require.Equal(t, "halfway", l.Status)

		got, err := s.Get(ctx, []string{"TKT-1"}, at(30))
		require.NoError(t, err)
		require.Equal(t, l, got["TKT-1"], "the renewed lease is live past its first expiry")
	})

	t.Run("another identity cannot renew", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Renew(ctx, "h1", bob.Identity, at(1), at(30), nil)
		require.ErrorIs(t, err, lease.ErrLost)

		got, err := s.Get(ctx, []string{"TKT-1"}, at(1))
		require.NoError(t, err)
		require.True(t, got["TKT-1"].Expires.Equal(at(15)), "a refused renew changes nothing")
	})

	t.Run("an unknown hash is lost", func(t *testing.T) {
		s := f(t)
		_, err := s.Renew(ctx, "nope", alice.Identity, at(0), at(15), nil)
		require.ErrorIs(t, err, lease.ErrLost)
		_, err = s.Renew(ctx, "", alice.Identity, at(0), at(15), nil)
		require.ErrorIs(t, err, lease.ErrLost)
	})

	t.Run("renew after expiry is lost", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Renew(ctx, "h1", alice.Identity, at(15), at(30), nil)
		require.ErrorIs(t, err, lease.ErrLost, "a lease is not live at its expiry instant")
	})

	t.Run("renew after release is lost", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Release(ctx, "h1", alice.Identity, at(1)))
		_, err := s.Renew(ctx, "h1", alice.Identity, at(2), at(17), nil)
		require.ErrorIs(t, err, lease.ErrLost)
	})

	t.Run("renew to an expiry at or before now is refused", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		_, err := s.Renew(ctx, "h1", alice.Identity, at(5), at(5), nil)
		require.ErrorIs(t, err, lease.ErrInvalid)
	})
}

// RunReleaseTests covers release, which answers the same for every input.
func RunReleaseTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("release frees the entity", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Release(ctx, "h1", alice.Identity, at(1)))

		got, err := s.Get(ctx, []string{"TKT-1"}, at(1))
		require.NoError(t, err)
		require.Empty(t, got)
		claim(ctx, t, s, "TKT-1", bob, "h2", at(2), at(17))
	})

	t.Run("release answers nil whether or not the lease existed", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Release(ctx, "unknown", alice.Identity, at(1)))
		require.NoError(t, s.Release(ctx, "h1", bob.Identity, at(1)), "another identity gets the same answer")

		got, err := s.Get(ctx, []string{"TKT-1"}, at(1))
		require.NoError(t, err)
		require.Contains(t, got, "TKT-1", "only the holder's identity releases")

		require.NoError(t, s.Release(ctx, "h1", alice.Identity, at(1)))
		require.NoError(t, s.Release(ctx, "h1", alice.Identity, at(1)), "a second release is not an error")
	})
}

// RunExpiryTests covers lazy expiry on reads and the prune on claim.
func RunExpiryTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("reads leave out expired leases", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))

		got, err := s.Get(ctx, []string{"TKT-1"}, at(15))
		require.NoError(t, err)
		require.Empty(t, got, "a lease is not live at its expiry instant")
		mine, err := s.ListByHolder(ctx, alice.Identity, at(15))
		require.NoError(t, err)
		require.Empty(t, mine)

		got, err = s.Get(ctx, []string{"TKT-1"}, at(14))
		require.NoError(t, err)
		require.Len(t, got, 1, "reads do not remove anything")
	})

	t.Run("an expired lease can be claimed by someone else", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		l := claim(ctx, t, s, "TKT-1", bob, "h2", at(15), at(30))
		require.Equal(t, bob, l.Holder)
	})

	t.Run("claim prunes every expired lease", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		claim(ctx, t, s, "TKT-2", alice, "h2", at(0), at(60))
		claim(ctx, t, s, "TKT-3", bob, "h3", at(20), at(40))

		// Read at an instant where TKT-1 would still be live: it is gone.
		got, err := s.Get(ctx, []string{"TKT-1", "TKT-2", "TKT-3"}, at(10))
		require.NoError(t, err)
		require.NotContains(t, got, "TKT-1", "the claim at minute 20 must have removed the expired lease")
		require.Contains(t, got, "TKT-2")

		// The pruned lease's hash is free again, and its renew is lost.
		_, err = s.Renew(ctx, "h1", alice.Identity, at(10), at(25), nil)
		require.ErrorIs(t, err, lease.ErrLost)
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
}

// RunLifecycleTests covers rekey and drop after an entity rename or delete.
func RunLifecycleTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("rekey moves the lease, which stays renewable", func(t *testing.T) {
		s := f(t)
		l := claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(15))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW"))

		got, err := s.Get(ctx, []string{"TKT-OLD", "TKT-NEW"}, at(1))
		require.NoError(t, err)
		require.NotContains(t, got, "TKT-OLD")
		l.ID = "TKT-NEW"
		require.Equal(t, l, got["TKT-NEW"])

		renewed, err := s.Renew(ctx, "h1", alice.Identity, at(2), at(20), nil)
		require.NoError(t, err)
		require.Equal(t, "TKT-NEW", renewed.ID)
	})

	t.Run("rekey replaces a lease on the new id", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-OLD", alice, "h1", at(0), at(15))
		claim(ctx, t, s, "TKT-NEW", bob, "h2", at(0), at(15))
		require.NoError(t, s.Rekey(ctx, "TKT-OLD", "TKT-NEW"))

		got, err := s.Get(ctx, []string{"TKT-OLD", "TKT-NEW"}, at(1))
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, alice, got["TKT-NEW"].Holder)
	})

	t.Run("rekey without a lease, or onto itself, changes nothing", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-NEW", bob, "h2", at(0), at(15))
		require.NoError(t, s.Rekey(ctx, "TKT-NONE", "TKT-NEW"))
		require.NoError(t, s.Rekey(ctx, "TKT-NEW", "TKT-NEW"))

		got, err := s.Get(ctx, []string{"TKT-NEW"}, at(1))
		require.NoError(t, err)
		require.Equal(t, bob, got["TKT-NEW"].Holder, "a rekey from an id without a lease must not drop the target's")
	})

	t.Run("drop removes the lease", func(t *testing.T) {
		s := f(t)
		claim(ctx, t, s, "TKT-1", alice, "h1", at(0), at(15))
		require.NoError(t, s.Drop(ctx, "TKT-1"))
		require.NoError(t, s.Drop(ctx, "TKT-1"), "dropping a missing lease is not an error")

		got, err := s.Get(ctx, []string{"TKT-1"}, at(1))
		require.NoError(t, err)
		require.Empty(t, got)
		_, err = s.Renew(ctx, "h1", alice.Identity, at(1), at(20), nil)
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

		require.NoError(t, s.Release(ctx, "h1", alice.Identity, at(11)))
		t3 := claim(ctx, t, s, "TKT-1", alice, "h3", at(11), at(60)).Token
		require.Greater(t, t3, t2, "a claim after a release")

		t4 := claim(ctx, t, s, "TKT-3", alice, "h4", at(-100), at(60)).Token
		require.Greater(t, t4, t3, "a claim on a clock that went back")

		require.NoError(t, s.Release(ctx, "h3", alice.Identity, at(-100)))
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
		_, err := s.Claim(ctx, "TKT-X", alice, "hx", at(1), at(60), "")
		require.ErrorIs(t, err, lease.ErrLimit)

		claim(ctx, t, s, "TKT-B", bob, "hb", at(1), at(60))

		// Once one of alice's leases expires, she may claim again.
		claim(ctx, t, s, "TKT-X", alice, "hx", at(15), at(60))
	})
}

// RunVersionTests covers the version watchers poll.
func RunVersionTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("every change moves the version and a no-op does not", func(t *testing.T) {
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
		_, err := s.Renew(ctx, "h1", alice.Identity, at(1), at(20), nil)
		require.NoError(t, err)
		changed("renew")
		require.NoError(t, s.Rekey(ctx, "TKT-1", "TKT-2"))
		changed("rekey")
		require.NoError(t, s.Release(ctx, "h1", alice.Identity, at(2)))
		changed("release")
		claim(ctx, t, s, "TKT-3", alice, "h3", at(3), at(20))
		changed("claim")
		require.NoError(t, s.Drop(ctx, "TKT-3"))
		changed("drop")

		require.NoError(t, s.Release(ctx, "unknown", alice.Identity, at(4)))
		unchanged("a release of nothing")
		require.NoError(t, s.Drop(ctx, "TKT-NONE"))
		unchanged("a drop of nothing")
		_, err = s.Get(ctx, []string{"TKT-1"}, at(4))
		require.NoError(t, err)
		unchanged("a read")
	})
}

// RunConcurrencyTests covers concurrent claims on one entity.
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
				_, err := s.Claim(ctx, "TKT-HOT", h, fmt.Sprintf("hash-%d", i), at(0), at(15), "")
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
}
