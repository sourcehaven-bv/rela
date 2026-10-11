package lease_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/kvlease"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// fakeClock is a settable clock.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// fakeEntity is one entity the fake gate knows.
type fakeEntity struct {
	typ   string
	faces []entity.Face
	// readers and writers name the users who may read and update it.
	readers, writers []string
	// faceWriters, when set, replaces writers per face.
	faceWriters map[entity.Face][]string
}

// ceilingKey carries ids hidden from one request, whatever its principal: a
// session ceiling, which the lease identity does not see.
type ceilingKey struct{}

func withCeiling(ctx context.Context, hidden ...string) context.Context {
	return context.WithValue(ctx, ceilingKey{}, hidden)
}

// fakeGate answers like the real gate: a miss is store.ErrNotFound whether
// the entity is absent or hidden, and a bare id of a faced type is ambiguous.
type fakeGate struct {
	mu       sync.Mutex
	entities map[string]*fakeEntity
	calls    atomic.Int64
	// batches counts Readable calls.
	batches atomic.Int64
	// onFamily, when set, runs once before the next Family answer.
	onFamily func(id string)
}

func (g *fakeGate) put(id string, e *fakeEntity) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entities[id] = e
}

func (g *fakeGate) remove(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entities, id)
}

func (g *fakeGate) readable(ctx context.Context, id string) (*fakeEntity, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entities[id]
	if !ok || !slices.Contains(e.readers, principal.From(ctx).User) {
		return nil, false
	}
	if hidden, _ := ctx.Value(ceilingKey{}).([]string); slices.Contains(hidden, id) {
		return nil, false
	}
	return e, true
}

func (g *fakeGate) Readable(ctx context.Context, ids []string) (map[string]bool, error) {
	g.calls.Add(1)
	g.batches.Add(1)
	out := map[string]bool{}
	for _, id := range ids {
		if _, ok := g.readable(ctx, id); ok {
			out[id] = true
		}
	}
	return out, nil
}

func (g *fakeGate) WriteTarget(ctx context.Context, addr string) (entity.Ref, error) {
	g.calls.Add(1)
	a, err := entity.ParseAddress(addr)
	if err != nil {
		return entity.Ref{}, store.ErrNotFound
	}
	e, ok := g.readable(ctx, a.ID())
	if !ok {
		return entity.Ref{}, store.ErrNotFound
	}
	if named, isNamed := a.Named(); isNamed {
		if !slices.Contains(e.faces, named.Face) {
			return entity.Ref{}, store.ErrNotFound
		}
		return named, nil
	}
	if !slices.Contains(e.faces, entity.ImplicitFace) {
		return entity.Ref{}, &visibility.AmbiguousAddressError{ID: a.ID(), Faces: e.faces}
	}
	return entity.Ref{ID: a.ID(), Face: entity.ImplicitFace}, nil
}

func (g *fakeGate) Family(ctx context.Context, id string) (lease.Family, bool, error) {
	g.calls.Add(1)
	g.mu.Lock()
	hook := g.onFamily
	g.onFamily = nil
	g.mu.Unlock()
	if hook != nil {
		hook(id)
	}
	e, ok := g.readable(ctx, id)
	if !ok {
		return lease.Family{}, false, nil
	}
	return lease.Family{Type: e.typ, Faces: e.faces}, true, nil
}

// fakeAuth allows an update when the user is one of the entity's writers.
type fakeAuth struct{ g *fakeGate }

func (a fakeAuth) AuthorizeWrite(ctx context.Context, req acl.WriteRequest) acl.Decision {
	sub, ok := req.Subject.(acl.EntitySubject)
	if !ok || req.Op != acl.OpUpdate {
		return acl.Decision{Reason: "unexpected request"}
	}
	a.g.mu.Lock()
	defer a.g.mu.Unlock()
	var writers []string
	if e, ok := a.g.entities[sub.ID()]; ok {
		writers = e.writers
		if e.faceWriters != nil {
			writers = e.faceWriters[sub.Face()]
		}
	}
	if slices.Contains(writers, principal.From(ctx).User) {
		return acl.Decision{Allow: true}
	}
	return acl.Decision{RuleKind: "role-grant", RuleID: "-", Reason: "no update grant"}
}

// countingStore counts every store call.
type countingStore struct {
	lease.Store
	calls atomic.Int64
}

func (c *countingStore) Claim(
	ctx context.Context, id string, h lease.Holder, hash string, now, until time.Time, status string,
) (lease.Lease, error) {
	c.calls.Add(1)
	return c.Store.Claim(ctx, id, h, hash, now, until, status)
}

func (c *countingStore) Renew(
	ctx context.Context, hash, identity string, now, until time.Time, status *string,
) (lease.Lease, error) {
	c.calls.Add(1)
	return c.Store.Renew(ctx, hash, identity, now, until, status)
}

func (c *countingStore) Release(ctx context.Context, hash, identity string, now time.Time) error {
	c.calls.Add(1)
	return c.Store.Release(ctx, hash, identity, now)
}

func (c *countingStore) Lookup(ctx context.Context, hash, identity string, now time.Time) (lease.Lease, error) {
	c.calls.Add(1)
	return c.Store.Lookup(ctx, hash, identity, now)
}

type env struct {
	svc   *lease.Service
	gate  *fakeGate
	clock *fakeClock
	store *countingStore
	kv    state.KV
}

var (
	alice = principal.Principal{User: "alice", Tool: "mcp"}
	bob   = principal.Principal{User: "bob", Tool: "web"}
	carol = principal.Principal{User: "carol", Tool: "web"}
)

func as(p principal.Principal) context.Context { return principal.With(context.Background(), p) }

func newKV(t *testing.T) state.KV {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

// newEnv builds a service over a real kvlease store. TKT-1 is readable and
// writable by alice and bob, readable only by carol; TKT-F is a faced type.
func newEnv(t *testing.T) *env {
	t.Helper()
	kv := newKV(t)
	st, err := kvlease.New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	return newEnvOver(t, st, kv)
}

func newEnvOver(t *testing.T, st lease.Store, kv state.KV) *env {
	t.Helper()
	g := &fakeGate{entities: map[string]*fakeEntity{}}
	g.put("TKT-1", &fakeEntity{
		typ: "ticket", faces: []entity.Face{entity.ImplicitFace},
		readers: []string{"alice", "bob", "carol"}, writers: []string{"alice", "bob"},
	})
	g.put("TKT-F", &fakeEntity{
		typ: "doc", faces: []entity.Face{"draft", "published"},
		readers: []string{"alice", "bob"}, writers: []string{"alice", "bob"},
	})
	g.put("TKT-H", &fakeEntity{
		typ: "ticket", faces: []entity.Face{entity.ImplicitFace},
		readers: []string{"alice"}, writers: []string{"alice"},
	})
	cs := &countingStore{Store: st}
	clock := &fakeClock{now: t0}
	svc, err := lease.NewService(lease.Deps{
		Store: cs, Gate: g, Authorizer: fakeAuth{g}, Clock: clock, Hub: lease.NewHub(),
	})
	require.NoError(t, err)
	return &env{svc: svc, gate: g, clock: clock, store: cs, kv: kv}
}

func (e *env) claim(t *testing.T, p principal.Principal, addr string) lease.Claimed {
	t.Helper()
	c, err := e.svc.Claim(as(p), lease.ClaimRequest{Addr: addr, Label: "agent-1", Status: "on it"})
	require.NoError(t, err)
	return c
}

func (e *env) held(t *testing.T, id string) (lease.Lease, bool) {
	t.Helper()
	got, err := e.svc.GetReadable(context.Background(), []string{id})
	require.NoError(t, err)
	l, ok := got[id]
	return l, ok
}

func TestNewService_RejectsNilCollaborators(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	full := lease.Deps{
		Store: e.store, Gate: e.gate, Authorizer: fakeAuth{e.gate}, Clock: e.clock, Hub: lease.NewHub(),
	}
	cases := []struct {
		name string
		mod  func(d *lease.Deps)
	}{
		{"store", func(d *lease.Deps) { d.Store = nil }},
		{"gate", func(d *lease.Deps) { d.Gate = nil }},
		{"authorizer", func(d *lease.Deps) { d.Authorizer = nil }},
		{"clock", func(d *lease.Deps) { d.Clock = nil }},
		{"hub", func(d *lease.Deps) { d.Hub = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := full
			tc.mod(&d)
			_, err := lease.NewService(d)
			require.Error(t, err)
		})
	}
}

func TestClaim_AccessChecks(t *testing.T) {
	t.Parallel()

	t.Run("hidden and nonexistent give the same not-found", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		_, hidden := e.svc.Claim(as(bob), lease.ClaimRequest{Addr: "TKT-H"})
		_, missing := e.svc.Claim(as(bob), lease.ClaimRequest{Addr: "TKT-NOPE"})
		require.ErrorIs(t, hidden, store.ErrNotFound)
		require.ErrorIs(t, missing, store.ErrNotFound)
		require.Equal(t, missing.Error(), hidden.Error())
	})

	t.Run("a hidden entity held by someone else is still not-found", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.claim(t, alice, "TKT-H")
		_, err := e.svc.Claim(as(bob), lease.ClaimRequest{Addr: "TKT-H"})
		require.ErrorIs(t, err, store.ErrNotFound)
		require.NotContains(t, err.Error(), "alice")
	})

	t.Run("bare id of a faced type is ambiguous, naming readable faces", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		_, err := e.svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-F"})
		var amb *visibility.AmbiguousAddressError
		require.ErrorAs(t, err, &amb)
		require.Equal(t, []entity.Face{"draft", "published"}, amb.Faces)

		c := e.claim(t, alice, "TKT-F@draft")
		require.Equal(t, "TKT-F", c.Lease.ID, "the lease covers the whole family")
	})

	t.Run("readable but not updatable is forbidden without holder details", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.claim(t, alice, "TKT-1")
		_, err := e.svc.Claim(as(carol), lease.ClaimRequest{Addr: "TKT-1"})
		require.ErrorIs(t, err, acl.ErrForbidden)
		var held *lease.HeldError
		require.NotErrorAs(t, err, &held)
		require.NotContains(t, err.Error(), "alice")
		require.NotContains(t, err.Error(), "on it")
	})

	t.Run("held by another names the holder", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		first := e.claim(t, alice, "TKT-1")
		_, err := e.svc.Claim(as(bob), lease.ClaimRequest{Addr: "TKT-1"})
		var held *lease.HeldError
		require.ErrorAs(t, err, &held)
		cur := held.Current
		require.Equal(t, "alice", cur.Holder.User)
		require.Equal(t, "mcp", cur.Holder.Tool)
		require.Equal(t, "agent-1", cur.Holder.Label)
		require.Equal(t, "on it", cur.Status)
		require.True(t, cur.Since.Equal(first.Lease.Since))
		require.Empty(t, cur.LeaseHash)
		require.Empty(t, cur.Holder.Identity)
	})

	t.Run("the holder claiming again gets HeldError naming itself", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.claim(t, alice, "TKT-1")
		_, err := e.svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-1"})
		var held *lease.HeldError
		require.ErrorAs(t, err, &held)
		require.Equal(t, "alice", held.Current.Holder.User)
	})
}

func TestRenew_LosesTheLeaseWithAccess(t *testing.T) {
	t.Parallel()

	t.Run("plain renew extends", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		c := e.claim(t, alice, "TKT-1")
		e.clock.Set(t0.Add(time.Minute))
		l, err := e.svc.Renew(as(alice), c.LeaseID, time.Hour, nil)
		require.NoError(t, err)
		require.True(t, l.Expires.Equal(t0.Add(time.Minute+time.Hour)))
		require.Equal(t, c.Lease.Token, l.Token)
		require.Equal(t, "on it", l.Status)
	})

	t.Run("someone else's lease id is lost", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		c := e.claim(t, alice, "TKT-1")
		_, err := e.svc.Renew(as(bob), c.LeaseID, 0, nil)
		require.ErrorIs(t, err, lease.ErrLost)
		_, ok := e.held(t, "TKT-1")
		require.True(t, ok, "a foreign renew leaves the lease alone")
	})

	cases := []struct {
		name   string
		revoke func(t *testing.T, e *env)
	}{
		{"read revoked", func(_ *testing.T, e *env) {
			e.gate.put("TKT-1", &fakeEntity{typ: "ticket", faces: []entity.Face{""}, readers: []string{"bob"}})
		}},
		{"update revoked", func(_ *testing.T, e *env) {
			e.gate.put("TKT-1", &fakeEntity{typ: "ticket", faces: []entity.Face{""}, readers: []string{"alice"}})
		}},
		{"entity gone without a hook", func(_ *testing.T, e *env) { e.gate.remove("TKT-1") }},
		{"entity deleted", func(t *testing.T, e *env) {
			t.Helper()
			e.gate.remove("TKT-1")
			require.NoError(t, e.svc.EntityDeleted(context.Background(), "TKT-1"))
		}},
		{"entity deleted and recreated", func(t *testing.T, e *env) {
			t.Helper()
			// The gate still knows TKT-1: it was created again.
			require.NoError(t, e.svc.EntityDeleted(context.Background(), "TKT-1"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			c := e.claim(t, alice, "TKT-1")
			tc.revoke(t, e)
			_, err := e.svc.Renew(as(alice), c.LeaseID, 0, nil)
			require.ErrorIs(t, err, lease.ErrLost)
			_, ok := e.held(t, "TKT-1")
			require.False(t, ok, "the entity shows no holder")
		})
	}
}

func TestRelease_SameAnswerForEveryLeaseID(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	mine := e.claim(t, alice, "TKT-1")
	foreign := e.claim(t, bob, "TKT-F@draft")
	unknown := strings.Repeat("ab", 16)

	require.NoError(t, e.svc.Release(as(alice), foreign.LeaseID))
	require.NoError(t, e.svc.Release(as(alice), unknown))
	_, ok := e.held(t, "TKT-F")
	require.True(t, ok, "a foreign release removes nothing")
	require.NoError(t, e.svc.Release(as(alice), mine.LeaseID))
	_, ok = e.held(t, "TKT-1")
	require.False(t, ok)
	require.NoError(t, e.svc.Release(as(alice), mine.LeaseID), "releasing twice is the same answer")
}

func TestLeaseID_NeverStoredOrLogged(t *testing.T) { //nolint:paralleltest // replaces the default logger
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	e := newEnv(t)
	const status = "rewriting the parser"
	c, err := e.svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-1", Status: status})
	require.NoError(t, err)
	require.Regexp(t, `^[0-9a-f]{32}$`, c.LeaseID)
	require.Empty(t, c.Lease.LeaseHash)

	_, err = e.svc.Claim(as(bob), lease.ClaimRequest{Addr: "TKT-1", Status: status})
	require.Error(t, err)
	newStatus := status + " v2"
	_, err = e.svc.Renew(as(alice), c.LeaseID, 0, &newStatus)
	require.NoError(t, err)
	_, err = e.svc.Mine(as(alice))
	require.NoError(t, err)
	e.gate.remove("TKT-1")
	_, err = e.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.ErrorIs(t, err, lease.ErrLost)
	require.NoError(t, e.svc.Release(as(alice), c.LeaseID))

	sum := sha256.Sum256([]byte(c.LeaseID))
	hash := hex.EncodeToString(sum[:])
	logged := buf.String()
	require.Contains(t, logged, "alice", "the conflict is logged with the holder")
	for _, secret := range []string{c.LeaseID, hash, status} {
		require.NotContains(t, logged, secret)
	}

	// A second claim leaves a document behind to inspect.
	c2 := e.claim(t, bob, "TKT-F@draft")
	doc, err := e.kv.Get(context.Background(), kvlease.StateKey)
	require.NoError(t, err)
	sum = sha256.Sum256([]byte(c2.LeaseID))
	require.NotContains(t, string(doc), c2.LeaseID)
	require.Contains(t, string(doc), hex.EncodeToString(sum[:]), "the store keeps only the hash")
}

func TestValidation_BeforeAnyRead(t *testing.T) {
	t.Parallel()
	valid := strings.Repeat("0", 32)
	str := func(s string) *string { return &s }
	cases := []struct {
		name string
		call func(s *lease.Service) error
	}{
		{"claim ttl too short", claimWith(lease.ClaimRequest{Addr: "TKT-1", TTL: 29 * time.Second})},
		{"claim ttl too long", claimWith(lease.ClaimRequest{Addr: "TKT-1", TTL: time.Hour + time.Second})},
		{"claim negative ttl", claimWith(lease.ClaimRequest{Addr: "TKT-1", TTL: -time.Minute})},
		{"claim empty address", claimWith(lease.ClaimRequest{})},
		{"claim long status", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: strings.Repeat("é", 141)})},
		{"claim status newline", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: "a\nb"})},
		{"claim status carriage return", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: "a\rb"})},
		{"claim status control", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: "a\x07b"})},
		{"claim status bidi override", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: "a\u202eb"})},
		{"claim status bidi isolate", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: "a\u2066b"})},
		{"claim status invalid utf-8", claimWith(lease.ClaimRequest{Addr: "TKT-1", Status: "a\xffb"})},
		{"claim label too long", claimWith(lease.ClaimRequest{Addr: "TKT-1", Label: strings.Repeat("a", 33)})},
		{"claim label space", claimWith(lease.ClaimRequest{Addr: "TKT-1", Label: "a b"})},
		{"claim label slash", claimWith(lease.ClaimRequest{Addr: "TKT-1", Label: "a/b"})},
		{"renew ttl too short", func(s *lease.Service) error {
			_, err := s.Renew(as(alice), valid, time.Second, nil)
			return err
		}},
		{"renew bad status", func(s *lease.Service) error {
			_, err := s.Renew(as(alice), valid, 0, str("x\u200fy"))
			return err
		}},
		{"renew malformed id", func(s *lease.Service) error {
			_, err := s.Renew(as(alice), "not-a-lease", 0, nil)
			return err
		}},
		{"release malformed id", func(s *lease.Service) error { return s.Release(as(alice), "ABC") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			require.ErrorIs(t, tc.call(e.svc), lease.ErrInvalid)
			require.Zero(t, e.store.calls.Load(), "no store call")
			require.Zero(t, e.gate.calls.Load(), "no gate call")
		})
	}
}

func claimWith(req lease.ClaimRequest) func(s *lease.Service) error {
	return func(s *lease.Service) error {
		_, err := s.Claim(as(alice), req)
		return err
	}
}

func TestClaim_TTL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ttl  time.Duration
		want time.Duration
	}{
		{"default", 0, 15 * time.Minute},
		{"minimum", 30 * time.Second, 30 * time.Second},
		{"maximum", time.Hour, time.Hour},
		{"between the bounds", 20 * time.Minute, 20 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			c, err := e.svc.Claim(as(alice), lease.ClaimRequest{
				Addr: "TKT-1", TTL: tc.ttl, Label: strings.Repeat("a", 32), Status: strings.Repeat("é", 140),
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, c.Lease.Expires.Sub(c.Lease.Since))
		})
	}
}

func TestHooks_MoveDropAndWake(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := []struct {
		name  string
		hook  func(s *lease.Service) error
		check func(t *testing.T, e *env)
	}{
		{"rename moves the lease", func(s *lease.Service) error { return s.EntityRenamed(ctx, "TKT-1", "TKT-2") },
			func(t *testing.T, e *env) {
				t.Helper()
				_, old := e.held(t, "TKT-1")
				l, moved := e.held(t, "TKT-2")
				require.False(t, old)
				require.True(t, moved)
				require.Equal(t, "alice", l.Holder.User)
			}},
		{"delete drops the lease", func(s *lease.Service) error { return s.EntityDeleted(ctx, "TKT-1") },
			func(t *testing.T, e *env) {
				t.Helper()
				_, ok := e.held(t, "TKT-1")
				require.False(t, ok)
			}},
		{"face delete keeps the lease", func(s *lease.Service) error {
			return s.EntityFaceDeleted(ctx, "TKT-1", "draft")
		}, func(t *testing.T, e *env) {
			t.Helper()
			_, ok := e.held(t, "TKT-1")
			require.True(t, ok)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.claim(t, alice, "TKT-1")
			wake, cancel := e.svc.Subscribe()
			defer cancel()
			require.NoError(t, tc.hook(e.svc))
			requireWoken(t, wake)
			tc.check(t, e)
		})
	}
}

func TestSubscribe_FanOutCoalesceCancel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w1, cancel1 := e.svc.Subscribe()
	w2, cancel2 := e.svc.Subscribe()
	defer cancel2()

	c := e.claim(t, alice, "TKT-1")
	require.NoError(t, e.svc.Release(as(alice), c.LeaseID))
	e.claim(t, alice, "TKT-1")
	for _, w := range []<-chan struct{}{w1, w2} {
		requireWoken(t, w)
		requireQuiet(t, w, "three changes coalesce into one wake")
	}

	cancel1()
	cancel1()
	require.NoError(t, e.svc.EntityDeleted(context.Background(), "TKT-1"))
	requireQuiet(t, w1, "a cancelled subscriber is not woken")
	requireWoken(t, w2)
}

func TestRenew_WakesOnlyOnStatusChange(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := e.claim(t, alice, "TKT-1")
	wake, cancel := e.svc.Subscribe()
	defer cancel()
	same := "on it"
	_, err := e.svc.Renew(as(alice), c.LeaseID, 0, &same)
	require.NoError(t, err)
	requireQuiet(t, wake, "a plain renew changes nothing shown")
	other := "testing"
	_, err = e.svc.Renew(as(alice), c.LeaseID, 0, &other)
	require.NoError(t, err)
	requireWoken(t, wake)
}

func TestWatch_WakesOnRemoteWriteAndExpiry(t *testing.T) {
	t.Parallel()
	kv := newKV(t)
	st, err := kvlease.New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	local, remote := newEnvOver(t, st, kv), newEnvOver(t, st, kv)
	watcher, err := lease.NewService(lease.Deps{
		Store: st, Gate: local.gate, Authorizer: fakeAuth{local.gate}, Clock: local.clock,
		Hub: lease.NewHub(), PollInterval: 5 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	wake, unsub := watcher.Subscribe()
	defer unsub()
	go func() { done <- watcher.Watch(ctx) }()

	// Another process (a second service over the same store) claims: only the
	// version poll can tell the watcher. Watch may take its baseline after a
	// claim, so the claim is repeated until a wake arrives.
	var c lease.Claimed
	require.Eventually(t, func() bool {
		select {
		case <-wake:
			return true
		default:
		}
		if c.LeaseID != "" {
			if relErr := remote.svc.Release(as(alice), c.LeaseID); relErr != nil {
				return false
			}
		}
		next, claimErr := remote.svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-1"})
		if claimErr == nil {
			c = next
		}
		return false
	}, 10*time.Second, 20*time.Millisecond)
	require.NotEmpty(t, c.LeaseID)

	// Move the watcher's clock to just before the expiry and restart Watch so
	// it re-arms: the timer, not the version, must wake it.
	cancel()
	require.NoError(t, <-done)
	drain(wake)
	local.clock.Set(c.Lease.Expires.Add(-20 * time.Millisecond))
	_, ok := local.held(t, "TKT-1")
	require.True(t, ok, "the lease is live when the watcher arms")
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	go func() { done <- watcher.Watch(ctx) }()
	requireWoken(t, wake)
	cancel()
	require.NoError(t, <-done)

	// The wake was for a real lapse: at the expiry the watcher fired for,
	// the store has no live lease on the entity.
	got, err := st.Get(context.Background(), []string{"TKT-1"}, c.Lease.Expires)
	require.NoError(t, err)
	require.Empty(t, got, "the lease the watcher woke for has expired")
}

// startWatch runs svc.Watch until the test ends.
func startWatch(t *testing.T, svc *lease.Service) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Watch(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
}

// newWatcher returns a service over st with its own hub and clock, as
// another process would have, polling every 5ms.
func newWatcher(t *testing.T, st lease.Store, g *fakeGate, clock *fakeClock) *lease.Service {
	t.Helper()
	svc, err := lease.NewService(lease.Deps{
		Store: st, Gate: g, Authorizer: fakeAuth{g}, Clock: clock, Hub: lease.NewHub(),
		PollInterval: 5 * time.Millisecond,
	})
	require.NoError(t, err)
	return svc
}

// requireWatching makes changes through remote until the watcher's
// subscriber is woken, so the watcher has taken its baseline.
func requireWatching(t *testing.T, remote *env, wake <-chan struct{}) {
	t.Helper()
	var probe lease.Claimed
	require.Eventually(t, func() bool {
		select {
		case <-wake:
			return true
		default:
		}
		if probe.LeaseID != "" {
			require.NoError(t, remote.svc.Release(as(bob), probe.LeaseID))
		}
		probe = remote.claim(t, bob, "TKT-F@draft")
		return false
	}, 10*time.Second, 20*time.Millisecond)
	require.NoError(t, remote.svc.Release(as(bob), probe.LeaseID))
}

// A plain renew in another process moves no version, so the watcher does
// not wake; a status change does.
func TestWatch_PlainRenewDoesNotWake(t *testing.T) {
	t.Parallel()
	kv := newKV(t)
	st, err := kvlease.New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	local, remote := newEnvOver(t, st, kv), newEnvOver(t, st, kv)
	c := remote.claim(t, alice, "TKT-1")
	watcher := newWatcher(t, st, local.gate, local.clock)
	wake, unsub := watcher.Subscribe()
	defer unsub()
	startWatch(t, watcher)
	requireWatching(t, remote, wake)
	time.Sleep(50 * time.Millisecond) // let the probe's release be polled
	drain(wake)

	remote.clock.Set(t0.Add(time.Minute))
	_, err = remote.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	requireQuiet(t, wake, "a plain renew must not wake the watcher")

	status := "reviewing"
	_, err = remote.svc.Renew(as(alice), c.LeaseID, 0, &status)
	require.NoError(t, err)
	requireWoken(t, wake)
}

// The expiry timer fires for a lease that another process extended since:
// the watcher re-arms without waking.
func TestWatch_ExtendedLeaseRearmsWithoutWaking(t *testing.T) {
	t.Parallel()
	kv := newKV(t)
	st, err := kvlease.New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	local, remote := newEnvOver(t, st, kv), newEnvOver(t, st, kv)
	c, err := remote.svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-1", TTL: lease.MinTTL})
	require.NoError(t, err)
	local.clock.Set(c.Lease.Expires.Add(-300 * time.Millisecond))
	watcher := newWatcher(t, st, local.gate, local.clock)
	wake, unsub := watcher.Subscribe()
	defer unsub()
	startWatch(t, watcher)
	// The probe's release re-arms the timer for TKT-1 before the renew.
	requireWatching(t, remote, wake)
	time.Sleep(50 * time.Millisecond)
	drain(wake)

	remote.clock.Set(t0.Add(time.Second))
	_, err = remote.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.NoError(t, err)
	time.Sleep(600 * time.Millisecond)
	requireQuiet(t, wake, "an extended lease did not lapse")
}

// expiryFails fails its first NextExpiry calls.
type expiryFails struct {
	lease.Store
	left atomic.Int64
}

func (f *expiryFails) NextExpiry(ctx context.Context, now time.Time) (time.Time, bool, error) {
	if f.left.Add(-1) >= 0 {
		return time.Time{}, false, errors.New("next expiry unavailable")
	}
	return f.Store.NextExpiry(ctx, now)
}

// A failed NextExpiry leaves the timer unarmed; the next tick arms it even
// though no version moved, so the expiry still wakes.
func TestWatch_RearmsAfterFailedNextExpiry(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := e.claim(t, alice, "TKT-1")
	flaky := &expiryFails{Store: e.store}
	flaky.left.Store(1)
	clock := &fakeClock{now: c.Lease.Expires.Add(-20 * time.Millisecond)}
	watcher := newWatcher(t, flaky, e.gate, clock)
	wake, unsub := watcher.Subscribe()
	defer unsub()
	startWatch(t, watcher)
	requireWoken(t, wake)
}

// Mine leaves out a lease on an entity the caller's ceiling hides, though
// the identity is the same, and gates all leases in one batch.
func TestMine_GatedByCallerCeiling(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.claim(t, alice, "TKT-1")
	e.claim(t, alice, "TKT-H")

	all, err := e.svc.Mine(as(alice))
	require.NoError(t, err)
	require.Len(t, all, 2)

	before := e.gate.batches.Load()
	got, err := e.svc.Mine(withCeiling(as(alice), "TKT-H"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "TKT-1", got[0].ID)
	require.Empty(t, got[0].LeaseHash)
	require.Empty(t, got[0].Holder.Identity)
	require.Equal(t, int64(1), e.gate.batches.Load()-before, "one batch gate call")
}

func TestZeroPrincipal_RefusedBeforeAnyRead(t *testing.T) {
	t.Parallel()
	valid := strings.Repeat("0", 32)
	calls := []struct {
		name string
		call func(ctx context.Context, s *lease.Service) error
	}{
		{"claim", func(ctx context.Context, s *lease.Service) error {
			_, err := s.Claim(ctx, lease.ClaimRequest{Addr: "TKT-1"})
			return err
		}},
		{"renew", func(ctx context.Context, s *lease.Service) error {
			_, err := s.Renew(ctx, valid, 0, nil)
			return err
		}},
		{"release", func(ctx context.Context, s *lease.Service) error { return s.Release(ctx, valid) }},
	}
	callers := []struct {
		name string
		ctx  func() context.Context
	}{
		{"none stamped", context.Background},
		{"zero", func() context.Context { return as(principal.Principal{}) }},
		{"unknown", func() context.Context {
			return as(principal.Principal{User: principal.Unknown, Tool: principal.Unknown})
		}},
	}
	for _, c := range calls {
		for _, who := range callers {
			t.Run(c.name+"/"+who.name, func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				require.ErrorIs(t, c.call(who.ctx(), e.svc), lease.ErrNoPrincipal)
				require.Zero(t, e.store.calls.Load(), "no store call")
				require.Zero(t, e.gate.calls.Load(), "no gate call")
			})
		}
	}
}

// cancelAware fails a write on a cancelled context, as a database backend
// does.
type cancelAware struct{ lease.Store }

func (c cancelAware) Drop(ctx context.Context, id string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.Store.Drop(ctx, id, now)
}

func (c cancelAware) Rekey(ctx context.Context, from, to string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.Store.Rekey(ctx, from, to, now)
}

// The hooks run after the entity changed, so a requester that went away
// must not leave the lease behind.
func TestHooks_IgnoreCancellation(t *testing.T) {
	t.Parallel()
	cancelled := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	newCancelEnv := func(t *testing.T) *env {
		t.Helper()
		kv := newKV(t)
		st, err := kvlease.New(kv, filelock.ProcessPrivate{})
		require.NoError(t, err)
		return newEnvOver(t, cancelAware{st}, kv)
	}

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		e := newCancelEnv(t)
		e.claim(t, alice, "TKT-1")
		require.NoError(t, e.svc.EntityDeleted(cancelled(), "TKT-1"))
		_, ok := e.held(t, "TKT-1")
		require.False(t, ok, "the delete drops the lease")
	})
	t.Run("rename", func(t *testing.T) {
		t.Parallel()
		e := newCancelEnv(t)
		e.claim(t, alice, "TKT-1")
		require.NoError(t, e.svc.EntityRenamed(cancelled(), "TKT-1", "TKT-2"))
		_, moved := e.held(t, "TKT-2")
		require.True(t, moved, "the rename moves the lease")
	})
}

// A rename between the renew's lookup and its access check makes the check
// miss on the old id; the renew checks the new id instead of releasing.
func TestRenew_RechecksAfterConcurrentRename(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := e.claim(t, alice, "TKT-1")
	e.gate.onFamily = func(string) {
		e.gate.put("TKT-2", &fakeEntity{
			typ: "ticket", faces: []entity.Face{entity.ImplicitFace},
			readers: []string{"alice"}, writers: []string{"alice"},
		})
		e.gate.remove("TKT-1")
		require.NoError(t, e.svc.EntityRenamed(context.Background(), "TKT-1", "TKT-2"))
	}
	l, err := e.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.NoError(t, err)
	require.Equal(t, "TKT-2", l.ID)
	_, ok := e.held(t, "TKT-2")
	require.True(t, ok)

	e.gate.remove("TKT-2")
	_, err = e.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.ErrorIs(t, err, lease.ErrLost, "without a rename the miss releases the lease")
}

// A claim checks the face it names, a renew any readable face: losing
// update on the claimed face while gaining it on another keeps the lease.
func TestRenew_AnyUpdatableFaceKeepsTheLease(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	doc := func(writers map[entity.Face][]string) *fakeEntity {
		return &fakeEntity{
			typ: "doc", faces: []entity.Face{"draft", "published"},
			readers: []string{"alice"}, faceWriters: writers,
		}
	}
	e.gate.put("TKT-F", doc(map[entity.Face][]string{"draft": {"alice"}}))
	c := e.claim(t, alice, "TKT-F@draft")

	e.gate.put("TKT-F", doc(map[entity.Face][]string{"published": {"alice"}}))
	_, err := e.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.NoError(t, err, "update on another readable face keeps the lease")

	e.gate.put("TKT-F", doc(map[entity.Face][]string{}))
	_, err = e.svc.Renew(as(alice), c.LeaseID, 0, nil)
	require.ErrorIs(t, err, lease.ErrLost, "no updatable face loses the lease")
}

// TestClaim_TypeChecked pins ClaimRequest.Type: the family is read first,
// and a type other than the stored one is not found before the write target
// is resolved or update access is asked about.
func TestClaim_TypeChecked(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		p       principal.Principal
		addr    string
		typ     string
		wantErr error
		// wantGateCalls is the number of gate reads, when the claim fails.
		wantGateCalls int64
	}{
		{name: "matching type claims", p: alice, addr: "TKT-1", typ: "ticket"},
		{name: "no type claims", p: alice, addr: "TKT-1"},
		{name: "other type is not found", p: alice, addr: "TKT-1", typ: "doc",
			wantErr: store.ErrNotFound, wantGateCalls: 1},
		{name: "other type beats forbidden", p: carol, addr: "TKT-1", typ: "doc",
			wantErr: store.ErrNotFound, wantGateCalls: 1},
		{name: "other type beats ambiguous", p: alice, addr: "TKT-F", typ: "ticket",
			wantErr: store.ErrNotFound, wantGateCalls: 1},
		{name: "hidden is not found whatever the type", p: bob, addr: "TKT-H", typ: "ticket",
			wantErr: store.ErrNotFound, wantGateCalls: 1},
		{name: "unparseable address is not found before any read", p: alice, addr: "@@",
			wantErr: store.ErrNotFound, wantGateCalls: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			c, err := e.svc.Claim(as(tc.p), lease.ClaimRequest{Addr: tc.addr, Type: tc.typ})
			if tc.wantErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.addr, c.Lease.ID)
				return
			}
			require.Same(t, tc.wantErr, err)
			require.Equal(t, tc.wantGateCalls, e.gate.calls.Load())
			require.Zero(t, e.store.calls.Load())
		})
	}
}

// vanishingGate resolves a write target but then finds no family, as when
// the entity is deleted or hidden between the two reads.
type vanishingGate struct{ *fakeGate }

func (vanishingGate) Family(context.Context, string) (lease.Family, bool, error) {
	return lease.Family{}, false, nil
}

func TestClaim_EntityGoneBetweenReadsIsNotFound(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	svc, err := lease.NewService(lease.Deps{
		Store: e.store, Gate: vanishingGate{e.gate}, Authorizer: fakeAuth{e.gate}, Clock: e.clock, Hub: lease.NewHub(),
	})
	require.NoError(t, err)
	_, raced := svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-1"})
	_, missing := svc.Claim(as(alice), lease.ClaimRequest{Addr: "TKT-NOPE"})
	require.Same(t, missing, raced, "the race path returns the gate's not-found value")
	require.Zero(t, e.store.calls.Load())
}

func TestRelease_OneStoreCallThenWake(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := e.claim(t, alice, "TKT-1")
	wake, cancel := e.svc.Subscribe()
	defer cancel()
	before := e.store.calls.Load()
	require.NoError(t, e.svc.Release(as(alice), c.LeaseID))
	require.Equal(t, int64(1), e.store.calls.Load()-before, "a release is one store call, no lookup")
	requireWoken(t, wake)
}

func requireWoken(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("subscriber was not woken")
	}
}

func requireQuiet(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	default:
	}
}

func drain(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
