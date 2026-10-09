package piles_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/kvpiles"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newService(t *testing.T, opts piles.Options) *piles.Service {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	require.NoError(t, err)
	st, err := kvpiles.New(state.NewFSKV(rfs), kvpiles.ProcessPrivate{})
	require.NoError(t, err)
	if opts.Clock == nil {
		opts.Clock = func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }
	}
	svc, err := piles.NewService(st, opts)
	require.NoError(t, err)
	return svc
}

func as(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI})
}

func asPerson(id, login string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: id, RawUser: login, Tool: principal.ToolCLI})
}

func asScheduler() context.Context {
	return principal.With(context.Background(),
		principal.Principal{User: principal.UserScheduler, Tool: principal.ToolScheduler})
}

func r(id string) entity.Ref { return entity.Ref{ID: id, Face: entity.Face("main")} }

func TestNewService_RejectsNilStore(t *testing.T) {
	_, err := piles.NewService(nil, piles.Options{})
	require.Error(t, err)
}

func TestOwner(t *testing.T) {
	plain := newService(t, piles.Options{})
	mapped := newService(t, piles.Options{PersonType: "person"})
	tests := []struct {
		name    string
		svc     *piles.Service
		ctx     func() context.Context
		want    string
		wantErr error
	}{
		{"no principal", plain, context.Background, "", piles.ErrNoOwner},
		{"unknown principal", plain, func() context.Context { return as(principal.Unknown) }, "", piles.ErrNoOwner},
		{"system identity", plain, asScheduler, "", piles.ErrNoOwner},
		{"login name", plain, func() context.Context { return as("alice") }, "alice", nil},
		{"unmapped login under person mapping", mapped, func() context.Context { return as("alice") }, "", piles.ErrNoOwner},
		{"mapped person", mapped, func() context.Context { return asPerson("P-1", "alice") }, "P-1", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.svc.Owner(tc.ctx())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestService_Lifecycle(t *testing.T) {
	svc := newService(t, piles.Options{})
	ctx := as("alice")

	p, err := svc.Create(ctx, piles.CreateRequest{Name: "  Inbox ", Refs: []entity.Ref{r("A-1"), r("A-1"), r("A-2")}})
	require.NoError(t, err)
	require.Equal(t, "Inbox", p.Name)
	require.Equal(t, piles.DefaultIcon, p.Icon)
	require.Len(t, p.Items, 2, "duplicate refs collapse")
	require.True(t, piles.ValidID(p.ID))

	added, err := svc.Add(ctx, p.ID, []entity.Ref{r("A-2"), r("A-3")})
	require.NoError(t, err)
	require.Equal(t, 1, added)

	name, icon := "Today", "star"
	p, err = svc.Update(ctx, p.ID, &name, &icon)
	require.NoError(t, err)
	require.Equal(t, "Today", p.Name)
	require.Equal(t, "star", p.Icon)

	got, err := svc.ByName(ctx, "today")
	require.NoError(t, err)
	require.Equal(t, p.ID, got.ID)

	require.NoError(t, svc.Remove(ctx, p.ID, []entity.Ref{r("A-1"), r("Z-9")}))
	got, err = svc.Get(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 2)

	_, err = svc.Get(as("bob"), p.ID)
	require.ErrorIs(t, err, piles.ErrNotFound, "another user's pile does not exist for bob")

	list, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, svc.Delete(ctx, p.ID))
	_, err = svc.Get(ctx, p.ID)
	require.ErrorIs(t, err, piles.ErrNotFound)
}

func TestService_InvalidInput(t *testing.T) {
	svc := newService(t, piles.Options{})
	ctx := as("alice")
	for _, req := range []piles.CreateRequest{
		{Name: ""},
		{Name: strings.Repeat("x", piles.MaxNameRunes+1)},
		{Name: "a\tb"},
		{Name: "\xff"},
		{Name: "ok", Icon: "rocket"},
		{Name: "ok", Refs: []entity.Ref{{}}},
	} {
		_, err := svc.Create(ctx, req)
		require.ErrorIs(t, err, piles.ErrInvalid, "%+v", req)
	}
	for _, id := range []string{"", "PIL-x", "../etc"} {
		_, err := svc.Get(ctx, id)
		require.ErrorIs(t, err, piles.ErrNotFound)
		_, err = svc.Add(ctx, id, []entity.Ref{r("A-1")})
		require.ErrorIs(t, err, piles.ErrNotFound)
		require.ErrorIs(t, svc.Remove(ctx, id, nil), piles.ErrNotFound)
		require.ErrorIs(t, svc.Delete(ctx, id), piles.ErrNotFound)
	}
}

func TestService_PushToSelf(t *testing.T) {
	svc := newService(t, piles.Options{})
	ctx := as("alice")

	_, err := svc.Push(ctx, piles.PushRequest{Pile: "Inbox", Refs: []entity.Ref{r("A-1")}})
	require.ErrorIs(t, err, piles.ErrNotFound, "no pile and no create")

	n, err := svc.Push(ctx, piles.PushRequest{Pile: "Inbox", Create: true, Refs: []entity.Ref{r("A-1")}})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	n, err = svc.Push(ctx, piles.PushRequest{Owner: "alice", Pile: "inbox", Refs: []entity.Ref{r("A-1"), r("A-2")}})
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestService_PushToOthers(t *testing.T) {
	people := map[string]bool{"P-2": true}
	exists := func(_ context.Context, id string) (bool, error) { return people[id], nil }
	svc := newService(t, piles.Options{OwnerExists: exists, PersonType: "person"})
	bob := asPerson("P-2", "bob")

	_, err := svc.Push(context.Background(), piles.PushRequest{Owner: "P-2", Pile: "Inbox", Create: true})
	require.ErrorIs(t, err, piles.ErrNoOwner, "an anonymous caller may not push to anyone")

	_, err = svc.Push(as(principal.Unknown), piles.PushRequest{Owner: "P-2", Pile: "Inbox", Create: true})
	require.ErrorIs(t, err, piles.ErrNoOwner)

	_, err = svc.Push(asScheduler(), piles.PushRequest{Owner: "P-404", Pile: "Inbox"})
	require.ErrorIs(t, err, piles.ErrUnknownOwner)
	require.NotContains(t, err.Error(), "P-404", "an owner error names the rule, not the target")

	_, err = svc.Push(asScheduler(), piles.PushRequest{Owner: principal.UserScheduler, Pile: "x"})
	require.ErrorIs(t, err, piles.ErrUnknownOwner)

	n, err := svc.Push(asScheduler(),
		piles.PushRequest{Owner: "P-2", Pile: "Inbox", Refs: []entity.Ref{r("A-1")}})
	require.NoError(t, err)
	require.Zero(t, n, "a push to a missing pile of another user reports nothing")
	_, err = svc.ByName(bob, "Inbox")
	require.ErrorIs(t, err, piles.ErrNotFound)

	_, err = svc.Push(asScheduler(),
		piles.PushRequest{Owner: "P-2", Pile: "Inbox", Create: true, Refs: []entity.Ref{r("A-1")}})
	require.NoError(t, err)
	got, err := svc.ByName(bob, "Inbox")
	require.NoError(t, err)
	require.Len(t, got.Items, 1)

	_, err = newService(t, piles.Options{}).Push(asScheduler(),
		piles.PushRequest{Owner: "P-2", Pile: "Inbox"})
	require.ErrorIs(t, err, piles.ErrUnknownOwner, "without person mapping no other owner exists")
}

func TestService_EntityHooks(t *testing.T) {
	t.Run("person mapping moves and drops owners", func(t *testing.T) {
		svc := newService(t, piles.Options{PersonType: "person"})
		_, err := svc.Create(asPerson("P-1", "alice"), piles.CreateRequest{Name: "Inbox", Refs: []entity.Ref{r("A-1")}})
		require.NoError(t, err)

		require.NoError(t, svc.EntityRenamed(context.Background(), "A-1", "A-9"))
		require.NoError(t, svc.EntityRenamed(context.Background(), "P-1", "P-7"))
		got, err := svc.ByName(asPerson("P-7", "alice"), "Inbox")
		require.NoError(t, err)
		require.Equal(t, "A-9", got.Items[0].Ref.ID)

		require.NoError(t, svc.EntityFaceDeleted(context.Background(), "A-9", entity.Face("main")))
		got, err = svc.ByName(asPerson("P-7", "alice"), "Inbox")
		require.NoError(t, err)
		require.Empty(t, got.Items)

		require.NoError(t, svc.EntityDeleted(context.Background(), "P-7"))
		list, err := svc.List(asPerson("P-7", "alice"))
		require.NoError(t, err)
		require.Empty(t, list)
	})
	t.Run("login owners are never matched against entity ids", func(t *testing.T) {
		svc := newService(t, piles.Options{})
		_, err := svc.Create(as("alice"), piles.CreateRequest{Name: "Inbox"})
		require.NoError(t, err)
		require.NoError(t, svc.EntityRenamed(context.Background(), "alice", "bob"))
		require.NoError(t, svc.EntityDeleted(context.Background(), "alice"))
		list, err := svc.List(as("alice"))
		require.NoError(t, err)
		require.Len(t, list, 1)
	})
}

func foreignService(t *testing.T) *piles.Service {
	t.Helper()
	exists := func(_ context.Context, id string) (bool, error) { return id == "P-2", nil }
	return newService(t, piles.Options{OwnerExists: exists, PersonType: "person"})
}

func manyRefs(prefix string, n int) []entity.Ref {
	out := make([]entity.Ref, n)
	for i := range out {
		out[i] = r(fmt.Sprintf("%s-%d", prefix, i))
	}
	return out
}

// RR-SEXBQP: a push into another user's pile must never evict what the owner
// collected.
func TestService_ForeignPushNeverEvicts(t *testing.T) {
	svc := foreignService(t)
	bob := asPerson("P-2", "bob")
	full, err := svc.Create(bob, piles.CreateRequest{Name: "Inbox", Refs: manyRefs("OWN", piles.MaxItems)})
	require.NoError(t, err)
	require.Len(t, full.Items, piles.MaxItems)

	n, err := svc.Push(asScheduler(), piles.PushRequest{Owner: "P-2", Pile: "Inbox", Refs: manyRefs("NEW", 3)})
	require.NoError(t, err)
	require.Zero(t, n)
	got, err := svc.Get(bob, full.ID)
	require.NoError(t, err)
	require.Equal(t, full.Refs(), got.Refs(), "every owner item survives a foreign push")

	// The owner's own push still evicts.
	n, err = svc.Push(bob, piles.PushRequest{Pile: "Inbox", Refs: manyRefs("NEW", 3)})
	require.NoError(t, err)
	require.Equal(t, 3, n)
	got, err = svc.Get(bob, full.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, piles.MaxItems)
	require.Equal(t, "NEW-0", got.Items[0].Ref.ID)
}

// RR-SEXBQP: pushes by others may create piles only up to MaxForeignPiles,
// leaving the owner the rest of MaxPiles.
func TestService_ForeignCreatesStopAtMaxForeignPiles(t *testing.T) {
	svc := foreignService(t)
	bob := asPerson("P-2", "bob")
	for i := range piles.MaxForeignPiles + 3 {
		n, err := svc.Push(asScheduler(), piles.PushRequest{
			Owner: "P-2", Pile: fmt.Sprintf("foreign-%d", i), Create: true, Refs: []entity.Ref{r("A-1")},
		})
		require.NoError(t, err, "past the foreign cap a push is a silent no-op")
		require.Zero(t, n)
	}
	list, err := svc.List(bob)
	require.NoError(t, err)
	require.Len(t, list, piles.MaxForeignPiles)

	for i := piles.MaxForeignPiles; i < piles.MaxPiles; i++ {
		_, err = svc.Create(bob, piles.CreateRequest{Name: fmt.Sprintf("own-%d", i)})
		require.NoError(t, err, "the owner can still fill the rest of MaxPiles")
	}
	_, err = svc.Create(bob, piles.CreateRequest{Name: "one too many"})
	require.ErrorIs(t, err, piles.ErrLimit)
}

func TestService_UnknownIconIsNotEchoed(t *testing.T) {
	svc := newService(t, piles.Options{})
	_, err := svc.Create(as("alice"), piles.CreateRequest{Name: "x", Icon: "<script>"})
	require.ErrorIs(t, err, piles.ErrInvalid)
	require.NotContains(t, err.Error(), "<script>")
}

func TestService_UpdatePartial(t *testing.T) {
	svc := newService(t, piles.Options{})
	ctx := as("alice")
	p, err := svc.Create(ctx, piles.CreateRequest{Name: "Inbox", Icon: "star"})
	require.NoError(t, err)

	name := "Today"
	got, err := svc.Update(ctx, p.ID, &name, nil)
	require.NoError(t, err)
	require.Equal(t, "Today", got.Name)
	require.Equal(t, "star", got.Icon, "a nil icon is left unchanged")

	icon := "flag"
	_, err = svc.Update(as("bob"), p.ID, &name, &icon)
	require.ErrorIs(t, err, piles.ErrNotFound, "a full update of another user's pile is not found")
	_, err = svc.Update(ctx, "PIL-x", &name, &icon)
	require.ErrorIs(t, err, piles.ErrNotFound)
}

// clashingStore refuses the first clashes creates with ErrIDTaken.
type clashingStore struct {
	piles.Store
	clashes int
}

func (c *clashingStore) CreatePile(
	ctx context.Context, p piles.Pile, refs []entity.Ref, maxPiles, maxItems int,
) error {
	if c.clashes > 0 {
		c.clashes--
		return piles.ErrIDTaken
	}
	return c.Store.CreatePile(ctx, p, refs, maxPiles, maxItems)
}

func TestService_CreateRetriesIDClash(t *testing.T) {
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	require.NoError(t, err)
	backend, err := kvpiles.New(state.NewFSKV(rfs), kvpiles.ProcessPrivate{})
	require.NoError(t, err)

	st := &clashingStore{Store: backend, clashes: 2}
	svc, err := piles.NewService(st, piles.Options{})
	require.NoError(t, err)
	_, err = svc.Create(as("alice"), piles.CreateRequest{Name: "Inbox"})
	require.NoError(t, err, "a clash is retried with a fresh id")

	st.clashes = 10
	_, err = svc.Create(as("alice"), piles.CreateRequest{Name: "Other"})
	require.ErrorIs(t, err, piles.ErrIDTaken, "retries are bounded")
}
