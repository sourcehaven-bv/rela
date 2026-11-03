package piles_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/kvpiles"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var (
	favorites = piles.Declared{Key: "favorites", Name: "Favorites", Icon: "star"}
	inbox     = piles.Declared{
		Key: "inbox", Name: "Notifications", Icon: "bell", Bump: true, ReadState: true, AutoRead: true,
		ClearReadAfter: time.Hour, ExpireAfter: 7 * 24 * time.Hour,
		Reasons: map[string]string{"done": "completed"},
	}
)

// clock is a settable test clock.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func stubStore(t *testing.T) piles.Store {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	require.NoError(t, err)
	st, err := kvpiles.New(state.NewFSKV(rfs), filelock.ProcessPrivate{})
	require.NoError(t, err)
	return st
}

func declaredService(t *testing.T, c *clock) *piles.Service {
	t.Helper()
	exists := func(_ context.Context, id string) (bool, error) { return id == "P-1" || id == "P-2", nil }
	return newService(t, piles.Options{
		OwnerExists: exists, PersonType: "person", Clock: c.Now,
		Declared: []piles.Declared{favorites, inbox},
	})
}

func TestNewService_ValidatesDeclared(t *testing.T) {
	for name, d := range map[string]piles.Declared{
		"bad key":              {Key: "Bad Key", Name: "X", Icon: "star"},
		"no name":              {Key: "x", Icon: "star"},
		"unknown icon":         {Key: "x", Name: "X", Icon: "rocket"},
		"auto read, no state":  {Key: "x", Name: "X", Icon: "star", AutoRead: true},
		"clear read, no state": {Key: "x", Name: "X", Icon: "star", ClearReadAfter: time.Hour},
		"negative expiry":      {Key: "x", Name: "X", Icon: "star", ExpireAfter: -time.Hour},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := piles.NewService(stubStore(t), piles.Options{Declared: []piles.Declared{d}})
			require.ErrorIs(t, err, piles.ErrInvalid)
		})
	}
	_, err := piles.NewService(stubStore(t), piles.Options{Declared: []piles.Declared{favorites, favorites}})
	require.ErrorIs(t, err, piles.ErrInvalid, "a key appears once")
}

func TestService_DeclaredPilesExistAndAreLocked(t *testing.T) {
	c := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	svc := declaredService(t, c)
	alice := asPerson("P-1", "alice")

	list, err := svc.List(alice)
	require.NoError(t, err)
	require.Len(t, list, 2, "every declared pile is listed without being made")
	require.Equal(t, "favorites", list[0].Key)
	require.Equal(t, "Favorites", list[0].Name)
	require.Equal(t, "bell", list[1].Icon)

	again, err := svc.List(alice)
	require.NoError(t, err)
	require.Equal(t, list[0].ID, again[0].ID, "a second list creates nothing")

	fav, err := svc.ByKey(alice, "favorites")
	require.NoError(t, err)
	require.Equal(t, list[0].ID, fav.ID)
	_, err = svc.ByKey(alice, "nope")
	require.ErrorIs(t, err, piles.ErrNotFound)

	name := "Mine"
	_, err = svc.Update(alice, fav.ID, &name, nil)
	require.ErrorIs(t, err, piles.ErrLocked)
	require.ErrorIs(t, svc.Delete(alice, fav.ID), piles.ErrLocked)

	_, err = svc.Create(alice, piles.CreateRequest{Name: "Favorites"})
	require.NoError(t, err, "a user pile may share a declared pile's name")

	_, err = svc.Add(alice, fav.ID, []entity.Ref{r("A-1")})
	require.NoError(t, err)
	got, err := svc.Get(alice, fav.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, "P-1", got.Items[0].Actor)

	d, ok := svc.Declaration(got)
	require.True(t, ok)
	require.Equal(t, "favorites", d.Key)
}

func TestService_UndeclaredKeyIsAnOrdinaryPile(t *testing.T) {
	c := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	st := stubStore(t)
	old, err := piles.NewService(st, piles.Options{Clock: c.Now, Declared: []piles.Declared{favorites}})
	require.NoError(t, err)
	fav, err := old.ByKey(as("alice"), "favorites")
	require.NoError(t, err)

	svc, err := piles.NewService(st, piles.Options{Clock: c.Now})
	require.NoError(t, err)
	_, ok := svc.Declaration(fav)
	require.False(t, ok)
	require.NoError(t, svc.Delete(as("alice"), fav.ID), "a pile the schema dropped is the user's to delete")
}

func TestService_NotifyPush(t *testing.T) {
	c := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	svc := declaredService(t, c)
	alice, bob := asPerson("P-1", "alice"), asPerson("P-2", "bob")
	push := func(ctx context.Context, owner string, id string) {
		t.Helper()
		_, err := svc.Push(ctx, piles.PushRequest{Owner: owner, Key: "inbox", Reason: "done", SkipSelf: true,
			Refs: []entity.Ref{r(id)}})
		require.NoError(t, err)
	}

	push(alice, "P-1", "T-1")
	got, err := svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Empty(t, got.Items, "SkipSelf drops a notification about your own write")

	push(bob, "P-1", "T-1")
	push(bob, "P-1", "T-2")
	got, err = svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	require.Equal(t, "T-2", got.Items[0].Ref.ID)
	require.Equal(t, "P-2", got.Items[0].Actor)
	require.Equal(t, "done", got.Items[0].Reason)

	_, err = svc.Push(bob, piles.PushRequest{Owner: "P-1", Key: "inbox", Reason: "nope", Refs: []entity.Ref{r("T-3")}})
	require.ErrorIs(t, err, piles.ErrInvalid, "a reason must be declared")
	_, err = svc.Push(bob, piles.PushRequest{Owner: "P-1", Key: "missing", Refs: []entity.Ref{r("T-3")}})
	require.ErrorIs(t, err, piles.ErrInvalid)
	_, err = svc.Push(bob, piles.PushRequest{Owner: "P-1", Pile: "Inbox", Reason: "done", Create: true})
	require.ErrorIs(t, err, piles.ErrInvalid, "only a declared pile records a reason")

	_, err = svc.Push(alice, piles.PushRequest{Pile: "inbox", Refs: []entity.Ref{r("T-4")}})
	require.NoError(t, err)
	got, err = svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Len(t, got.Items, 3, "a pile name equal to a declared key reaches the owner's declared pile")

	// Read state.
	require.NoError(t, svc.MarkRead(alice, got.ID, []entity.Ref{r("T-2")}, false))
	require.NoError(t, svc.MarkOpened(alice, []entity.Ref{r("T-4")}))
	got, err = svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	read := map[string]bool{}
	for _, it := range got.Items {
		read[it.Ref.ID] = it.Read()
	}
	require.Equal(t, map[string]bool{"T-1": false, "T-2": true, "T-4": true}, read)

	// Bump: a repeat notification comes back unread on top.
	push(bob, "P-1", "T-2")
	got, err = svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Equal(t, "T-2", got.Items[0].Ref.ID)
	require.False(t, got.Items[0].Read())

	// Expiry: read items go after ClearReadAfter, all after ExpireAfter.
	c.now = c.now.Add(2 * time.Hour)
	got, err = svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Len(t, got.Items, 2, "T-4 was read more than an hour ago")
	c.now = c.now.Add(8 * 24 * time.Hour)
	got, err = svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Empty(t, got.Items)

	fav, err := svc.ByKey(alice, "favorites")
	require.NoError(t, err)
	require.ErrorIs(t, svc.MarkRead(alice, fav.ID, nil, true), piles.ErrInvalid, "favorites keep no read state")
	require.NoError(t, svc.MarkRead(alice, got.ID, nil, true))
}

func TestService_DeclaredPushIgnoresForeignLimits(t *testing.T) {
	c := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	svc := declaredService(t, c)
	alice, bob := asPerson("P-1", "alice"), asPerson("P-2", "bob")
	for i := range piles.MaxPiles {
		_, err := svc.Create(alice, piles.CreateRequest{Name: fmt.Sprintf("Pile %d", i)})
		require.NoError(t, err)
	}
	n, err := svc.Push(bob, piles.PushRequest{Owner: "P-1", Key: "inbox", Refs: manyRefs("N", piles.MaxItems+5)})
	require.NoError(t, err)
	require.Zero(t, n, "a foreign push reports nothing")
	got, err := svc.ByKey(alice, "inbox")
	require.NoError(t, err)
	require.Len(t, got.Items, piles.MaxItems, "a full quota does not stop a declared pile, which evicts")
}

// Another user reaches a declared pile only as a notification: by key, onto a
// pile with read state. A push by name, or onto favorites, is dropped, so it
// cannot fill someone's pile and evict what they keep.
func TestService_ForeignPushReachesOnlyNotificationPiles(t *testing.T) {
	svc := declaredService(t, &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)})
	alice, bob := asPerson("P-1", "alice"), asPerson("P-2", "bob")
	for name, req := range map[string]piles.PushRequest{
		"favorites by key":  {Owner: "P-1", Key: "favorites", Refs: manyRefs("F", 3)},
		"favorites by name": {Owner: "P-1", Pile: "favorites", Refs: manyRefs("F", 3), Create: true},
		"inbox by name":     {Owner: "P-1", Pile: "inbox", Refs: manyRefs("I", 3), Create: true},
	} {
		_, err := svc.Push(bob, req)
		require.NoError(t, err, name)
	}
	for _, key := range []string{"favorites", "inbox"} {
		got, err := svc.ByKey(alice, key)
		require.NoError(t, err)
		require.Empty(t, got.Items, key)
	}
}

// A declared key works wherever a pile name is taken, as a push does, so a
// script or an MCP call can name "inbox" without knowing the pile's id.
func TestService_ByNameFindsDeclaredKey(t *testing.T) {
	svc := declaredService(t, &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)})
	alice := asPerson("P-1", "alice")

	p, err := svc.ByName(alice, "inbox")
	require.NoError(t, err)
	require.Equal(t, "inbox", p.Key)
	require.Equal(t, "Notifications", p.Name)

	_, err = svc.ByName(alice, "Notifications")
	require.ErrorIs(t, err, piles.ErrNotFound, "a declared pile is found by its key, not its name")
}
