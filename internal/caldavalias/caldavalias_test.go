package caldavalias

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

// memKV is an in-memory state.KV. Deliberately not a mock with expectations:
// these tests care about what SURVIVES a round trip, so a real store that can
// be re-read is the useful stand-in.
type memKV struct {
	mu   sync.Mutex
	data map[string][]byte
	// putErr, when set, fails the next Put — for the persistence-failure path.
	putErr error
	puts   int
}

func newMemKV() *memKV { return &memKV{data: map[string][]byte{}} }

func (m *memKV) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		// Must satisfy os.IsNotExist, per the state.KV contract.
		return nil, &os.PathError{Op: "open", Path: key, Err: os.ErrNotExist}
	}
	return v, nil
}

func (m *memKV) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.putErr != nil {
		return m.putErr
	}
	m.puts++
	m.data[key] = data
	return nil
}

func (m *memKV) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func newTestService(t *testing.T) (*Service, *memKV) {
	t.Helper()
	kv := newMemKV()
	svc, err := New(t.Context(), kv)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc, kv
}

// appleUUIDAlias mirrors the real inbound case: Apple mints a bare UUID as both
// the UID and the resource filename, which is why the alias exists at all.
func appleUUIDAlias() Alias {
	return Alias{
		Collection: "tasks",
		Href:       "D8AAE77A-89CB-46D2-BDA4-F319D2014D6B.ics",
		UID:        "D8AAE77A-89CB-46D2-BDA4-F319D2014D6B",
		EntityID:   "TSK-a3f8",
	}
}

func TestPutAndLookup(t *testing.T) {
	svc, _ := newTestService(t)
	want := appleUUIDAlias()
	if err := svc.Put(t.Context(), want); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok := svc.Lookup("", want.Collection, want.Href)
	if !ok {
		t.Fatal("Lookup found nothing")
	}
	if got != want {
		t.Errorf("Lookup = %+v, want %+v", got, want)
	}

	if _, ok := svc.Lookup("", want.Collection, "other.ics"); ok {
		t.Error("Lookup returned an alias for an unknown href")
	}
	// Collection is part of the key: the same href in another collection is a
	// different resource.
	if _, ok := svc.Lookup("", "bugs", want.Href); ok {
		t.Error("Lookup crossed a collection boundary")
	}
}

// TestSurvivesRestart is the point of persisting at all: a client-created
// to-do must still map to its entity after the process restarts, or the next
// sync re-creates it.
func TestSurvivesRestart(t *testing.T) {
	svc, kv := newTestService(t)
	want := appleUUIDAlias()
	if err := svc.Put(t.Context(), want); err != nil {
		t.Fatalf("Put: %v", err)
	}

	reloaded, err := New(t.Context(), kv)
	if err != nil {
		t.Fatalf("New after restart: %v", err)
	}
	got, ok := reloaded.Lookup("", want.Collection, want.Href)
	if !ok {
		t.Fatal("alias did not survive a restart")
	}
	if got != want {
		t.Errorf("reloaded = %+v, want %+v", got, want)
	}
}

func TestLookupByEntity(t *testing.T) {
	svc, _ := newTestService(t)
	a := appleUUIDAlias()
	if err := svc.Put(t.Context(), a); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok := svc.LookupByEntity("", "tasks", a.EntityID)
	if !ok || got.Href != a.Href {
		t.Errorf("LookupByEntity = %+v, %v; want href %q", got, ok, a.Href)
	}
	if _, ok := svc.LookupByEntity("", "bugs", a.EntityID); ok {
		t.Error("LookupByEntity crossed a collection boundary")
	}
	if _, ok := svc.LookupByEntity("", "tasks", "TSK-nope"); ok {
		t.Error("LookupByEntity matched an unknown entity")
	}
}

// TestEntityRenamed is the load-bearing case. If a rename does not rewrite the
// alias, the client sees a delete plus a create and the user's to-do silently
// duplicates.
func TestEntityRenamed(t *testing.T) {
	svc, kv := newTestService(t)
	a := appleUUIDAlias()
	if err := svc.Put(t.Context(), a); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := svc.EntityRenamed(t.Context(), "TSK-a3f8", "TSK-renamed"); err != nil {
		t.Fatalf("EntityRenamed: %v", err)
	}

	// Same resource, now pointing at the new id — NOT a delete plus a create.
	got, ok := svc.Lookup("", a.Collection, a.Href)
	if !ok {
		t.Fatal("the resource lost its alias across a rename")
	}
	if got.EntityID != "TSK-renamed" {
		t.Errorf("EntityID = %q, want the renamed id", got.EntityID)
	}
	if got.Href != a.Href || got.UID != a.UID {
		t.Error("the client-facing identity (href/uid) must not change on a rename")
	}

	// And it must be durable, not just in memory.
	reloaded, err := New(t.Context(), kv)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, _ := reloaded.Lookup("", a.Collection, a.Href); got.EntityID != "TSK-renamed" {
		t.Errorf("rename did not persist: %+v", got)
	}
}

func TestEntityRenamed_AcrossCollections(t *testing.T) {
	svc, _ := newTestService(t)
	// The same entity surfaced in two collections — legal, since each declares
	// its own filter over the same type.
	for _, c := range []string{"tasks", "urgent"} {
		a := appleUUIDAlias()
		a.Collection = c
		if err := svc.Put(t.Context(), a); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	if err := svc.EntityRenamed(t.Context(), "TSK-a3f8", "TSK-new"); err != nil {
		t.Fatalf("EntityRenamed: %v", err)
	}
	for _, c := range []string{"tasks", "urgent"} {
		got, ok := svc.Lookup("", c, appleUUIDAlias().Href)
		if !ok || got.EntityID != "TSK-new" {
			t.Errorf("collection %q was not rewritten: %+v", c, got)
		}
	}
}

// TestEntityIDsAreCaseInsensitive: entity id identity is case-insensitive since
// pgstore migration 0007, so a case-only rename must still find the alias.
func TestEntityIDsAreCaseInsensitive(t *testing.T) {
	svc, _ := newTestService(t)
	if err := svc.Put(t.Context(), appleUUIDAlias()); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, ok := svc.LookupByEntity("", "tasks", "tsk-A3F8"); !ok {
		t.Error("LookupByEntity should fold case")
	}
	if err := svc.EntityRenamed(t.Context(), "tsk-A3F8", "TSK-new"); err != nil {
		t.Fatalf("EntityRenamed: %v", err)
	}
	got, _ := svc.Lookup("", "tasks", appleUUIDAlias().Href)
	if got.EntityID != "TSK-new" {
		t.Errorf("a case-differing rename did not match: %+v", got)
	}
}

func TestEntityDeleted(t *testing.T) {
	svc, _ := newTestService(t)
	keep := Alias{Collection: "tasks", Href: "keep.ics", UID: "u2", EntityID: "TSK-other"}
	if err := svc.Put(t.Context(), appleUUIDAlias()); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := svc.Put(t.Context(), keep); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := svc.EntityDeleted(t.Context(), "TSK-a3f8"); err != nil {
		t.Fatalf("EntityDeleted: %v", err)
	}
	// The alias must SURVIVE. It is the durable record that this server once
	// served the resource, and CalDAV reads "alias exists, entity does not" as
	// proof of a deliberate deletion. Removing it here would destroy that
	// evidence, and the next PUT from an unsynced client would read as a create
	// and resurrect the entity — the opposite of what deleting it would achieve.
	if _, ok := svc.Lookup("", "tasks", appleUUIDAlias().Href); !ok {
		t.Error("the alias was dropped; it is the tombstone a stale PUT is refused against")
	}
	if _, ok := svc.Lookup("", keep.Collection, keep.Href); !ok {
		t.Error("EntityDeleted removed an unrelated alias")
	}
}

func TestDelete(t *testing.T) {
	svc, _ := newTestService(t)
	a := appleUUIDAlias()
	if err := svc.Put(t.Context(), a); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := svc.Delete(t.Context(), "", a.Collection, a.Href); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := svc.Lookup("", a.Collection, a.Href); ok {
		t.Error("alias survived Delete")
	}
	// Deleting an absent alias is a no-op, matching the state.KV convention.
	if err := svc.Delete(t.Context(), "", a.Collection, a.Href); err != nil {
		t.Errorf("deleting an absent alias should be a no-op, got %v", err)
	}
}

// TestNoOpsDoNotWrite guards against rewriting the whole table on every
// unrelated rename or delete in the graph — most renames touch no alias at all.
func TestNoOpsDoNotWrite(t *testing.T) {
	svc, kv := newTestService(t)
	if err := svc.Put(t.Context(), appleUUIDAlias()); err != nil {
		t.Fatalf("Put: %v", err)
	}
	before := kv.puts

	if err := svc.EntityRenamed(t.Context(), "TSK-unrelated", "TSK-x"); err != nil {
		t.Fatalf("EntityRenamed: %v", err)
	}
	if err := svc.EntityDeleted(t.Context(), "TSK-unrelated"); err != nil {
		t.Fatalf("EntityDeleted: %v", err)
	}
	if kv.puts != before {
		t.Errorf("a no-op rewrote the table: %d writes, want %d", kv.puts, before)
	}
}

// TestCorruptStoreIsHardError pins the policy: a corrupt table REFUSES rather
// than starting empty. An empty table re-creates every to-do as a new entity,
// silently doubling the user's list — the failure internal/cli/sync also
// refuses to risk.
func TestCorruptStoreIsHardError(t *testing.T) {
	kv := newMemKV()
	if err := kv.Put(t.Context(), stateKey, []byte("{not json")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc, err := New(t.Context(), kv)
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("want ErrCorrupt, got %v", err)
	}
	if svc != nil {
		t.Error("a corrupt store must not yield a usable service")
	}
}

func TestMissingStoreIsFirstRun(t *testing.T) {
	svc, err := New(t.Context(), newMemKV())
	if err != nil {
		t.Fatalf("a missing table is a normal first run, got %v", err)
	}
	if _, ok := svc.Lookup("", "tasks", "x.ics"); ok {
		t.Error("a fresh service should hold nothing")
	}
}

func TestPutRejectsIncompleteAlias(t *testing.T) {
	svc, _ := newTestService(t)
	for _, tc := range []struct {
		name  string
		alias Alias
	}{
		{"no collection", Alias{Href: "h.ics", EntityID: "TSK-1"}},
		{"no href", Alias{Collection: "tasks", EntityID: "TSK-1"}},
		{"no entity", Alias{Collection: "tasks", Href: "h.ics"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.Put(t.Context(), tc.alias); err == nil {
				t.Error("want an error for an incomplete alias")
			}
		})
	}
}

// TestPersistFailureSurfaces: unlike version capture, which is best-effort, a
// failed alias write is returned. Losing an alias corrupts a user's list rather
// than losing an audit record.
func TestPersistFailureSurfaces(t *testing.T) {
	svc, kv := newTestService(t)
	kv.putErr = errors.New("disk on fire")

	if err := svc.Put(t.Context(), appleUUIDAlias()); err == nil {
		t.Error("Put should surface a persistence failure")
	}
	if err := svc.EntityRenamed(t.Context(), "TSK-a3f8", "TSK-x"); err != nil {
		// Nothing was stored (the Put failed), so the rename is a no-op and
		// must not manufacture an error.
		t.Errorf("a no-op rename should not fail: %v", err)
	}
}

// TestStoredFormIsStable keeps the on-disk bytes diffable: an unordered rewrite
// would churn the file on every save and make debugging harder.
func TestStoredFormIsStable(t *testing.T) {
	svc, kv := newTestService(t)
	// Distinct entities per href: Put evicts a second href for the SAME entity
	// (one entity, one resource), so reusing one id here would leave a single
	// alias and test nothing about ordering.
	for i, href := range []string{"c.ics", "a.ics", "b.ics"} {
		a := appleUUIDAlias()
		a.Href = href
		a.EntityID = fmt.Sprintf("TSK-order%d", i)
		if err := svc.Put(t.Context(), a); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	raw, err := kv.Get(t.Context(), stateKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	var stored []Alias
	if unmarshalErr := json.Unmarshal(raw, &stored); unmarshalErr != nil {
		t.Fatalf("stored form is not valid JSON: %v", unmarshalErr)
	}
	var hrefs []string
	for _, a := range stored {
		hrefs = append(hrefs, a.Href)
	}
	if want := []string{"a.ics", "b.ics", "c.ics"}; strings.Join(hrefs, ",") != strings.Join(want, ",") {
		t.Errorf("stored order = %v, want sorted %v", hrefs, want)
	}
}

// TestConcurrentMutations exercises the mutex under -race: state.KV offers no
// read-modify-write primitive, so the in-process lock is what makes a merge
// safe.
func TestConcurrentMutations(t *testing.T) {
	svc, _ := newTestService(t)
	var wg sync.WaitGroup
	for i := range 25 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			a := appleUUIDAlias()
			a.Href = string(rune('a'+i%26)) + ".ics"
			_ = svc.Put(t.Context(), a)
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.Lookup("", "tasks", "a.ics")
		}()
		go func() {
			defer wg.Done()
			_ = svc.EntityRenamed(t.Context(), "TSK-a3f8", "TSK-a3f8")
		}()
	}
	wg.Wait()
}

// TestFailedWriteRollsBack pins the commit discipline: a mutation that cannot
// be persisted must not survive in memory either. Otherwise the service and
// disk disagree — a later rename would rewrite an alias that was never stored,
// and a restart would silently lose it.
func TestFailedWriteRollsBack(t *testing.T) {
	svc, kv := newTestService(t)
	stored := appleUUIDAlias()
	if err := svc.Put(t.Context(), stored); err != nil {
		t.Fatalf("Put: %v", err)
	}

	kv.putErr = errors.New("disk full")

	t.Run("failed Put leaves no trace", func(t *testing.T) {
		nextAlias := Alias{Collection: "tasks", Href: "new.ics", UID: "u", EntityID: "TSK-new"}
		if err := svc.Put(t.Context(), nextAlias); err == nil {
			t.Fatal("want a persistence error")
		}
		if _, ok := svc.Lookup("", nextAlias.Collection, nextAlias.Href); ok {
			t.Error("an unpersisted alias is visible in memory")
		}
	})

	t.Run("failed rename leaves the old id", func(t *testing.T) {
		if err := svc.EntityRenamed(t.Context(), stored.EntityID, "TSK-renamed"); err == nil {
			t.Fatal("want a persistence error")
		}
		got, _ := svc.Lookup("", stored.Collection, stored.Href)
		if got.EntityID != stored.EntityID {
			t.Errorf("EntityID = %q, want the unchanged %q — the rename was not persisted",
				got.EntityID, stored.EntityID)
		}
	})

	t.Run("failed delete keeps the alias", func(t *testing.T) {
		if err := svc.Delete(t.Context(), "", stored.Collection, stored.Href); err == nil {
			t.Fatal("want a persistence error")
		}
		if _, ok := svc.Lookup("", stored.Collection, stored.Href); !ok {
			t.Error("an unpersisted delete removed the alias from memory")
		}
	})

	// Once writes succeed again, the service is still consistent with disk.
	kv.putErr = nil
	reloaded, err := New(t.Context(), kv)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, ok := reloaded.Lookup("", stored.Collection, stored.Href)
	if !ok || got != stored {
		t.Errorf("on-disk state diverged: %+v, want %+v", got, stored)
	}
}

// TestOneEntityOneResource pins the invariant that stops a to-do flickering in
// and out of a client's list.
//
// Go randomizes map iteration, so when an entity had two aliases LookupByEntity
// returned a different href per call and objectFor served whichever it got. A
// CalDAV client reads a changed href as delete-plus-create, so the to-do
// vanishes and reappears on random sync cycles — and because the ctag hashes
// content rather than hrefs it does not change across the flip, so a polling
// client never learns to resync.
func TestOneEntityOneResource(t *testing.T) {
	svc, _ := newTestService(t)
	first := Alias{Collection: "tasks", Href: "a.ics", UID: "a", EntityID: "TSK-dup"}
	second := Alias{Collection: "tasks", Href: "b.ics", UID: "b", EntityID: "TSK-dup"}

	for _, a := range []Alias{first, second} {
		if err := svc.Put(t.Context(), a); err != nil {
			t.Fatalf("Put %s: %v", a.Href, err)
		}
	}

	// The newest write wins: that is the href the client just used.
	if _, ok := svc.Lookup("", "tasks", first.Href); ok {
		t.Error("the superseded href survived; two hrefs for one to-do is a state " +
			"no client can render, and the loser dangles")
	}
	if _, ok := svc.Lookup("", "tasks", second.Href); !ok {
		t.Fatal("the newest href was not recorded")
	}

	// And the answer must not depend on map iteration order.
	for i := range 50 {
		got, ok := svc.LookupByEntity("", "tasks", "TSK-dup")
		if !ok {
			t.Fatalf("call %d: no alias found", i)
		}
		if got.Href != second.Href {
			t.Fatalf("call %d: href = %q, want %q — the served href flips between polls",
				i, got.Href, second.Href)
		}
	}

	// A different entity in the same collection is untouched.
	other := Alias{Collection: "tasks", Href: "c.ics", UID: "c", EntityID: "TSK-other"}
	if err := svc.Put(t.Context(), other); err != nil {
		t.Fatalf("Put other: %v", err)
	}
	if _, ok := svc.Lookup("", "tasks", second.Href); !ok {
		t.Error("recording an unrelated entity evicted a live alias")
	}
}

// TestPrincipalScoping pins that the alias table is keyed per principal.
//
// An href is a CLIENT's own bookkeeping — Apple mints a bare UUID — so two
// identities have no reason to share that namespace. Without the principal in
// the key they do, and two consequences follow: one principal's href surfaces
// in another's listing, and Put's same-entity eviction deletes the other's
// alias, which a CalDAV client reads as delete-plus-create (the to-do vanishes
// and reappears).
//
// This also makes the table able to represent a principal-bound `where:` filter
// (an `@me` clause), where one config key resolves to a DIFFERENT member set
// per principal — i.e. genuinely different collections sharing a name.
func TestPrincipalScoping(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()

	// Same collection, same href, two identities, two different entities.
	for _, a := range []Alias{
		{Principal: "alice", Collection: "tasks", Href: "shared.ics", UID: "u1", EntityID: "TSK-A"},
		{Principal: "mallory", Collection: "tasks", Href: "shared.ics", UID: "u2", EntityID: "TSK-M"},
	} {
		if err := svc.Put(ctx, a); err != nil {
			t.Fatalf("Put(%s): %v", a.Principal, err)
		}
	}

	for _, tc := range []struct{ principal, wantEntity string }{
		{"alice", "TSK-A"},
		{"mallory", "TSK-M"},
	} {
		got, ok := svc.Lookup(tc.principal, "tasks", "shared.ics")
		if !ok {
			t.Fatalf("%s lost their alias — the later Put overwrote it", tc.principal)
		}
		if got.EntityID != tc.wantEntity {
			t.Errorf("%s resolved to %s, want %s — one principal's href resolved to "+
				"another's entity", tc.principal, got.EntityID, tc.wantEntity)
		}
	}

	// A third identity that never synced sees nothing.
	if _, ok := svc.Lookup("eve", "tasks", "shared.ics"); ok {
		t.Error("an unrelated principal resolved someone else's href")
	}
}

// TestPrincipalScopedEviction pins that Put's same-entity eviction does not
// reach across principals. Two clients holding different hrefs for one entity
// is the NORMAL state of a shared collection, not the ambiguity the eviction
// guards against.
func TestPrincipalScopedEviction(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()

	alice := Alias{Principal: "alice", Collection: "tasks", Href: "a.ics", UID: "ua", EntityID: "TSK-1"}
	if err := svc.Put(ctx, alice); err != nil {
		t.Fatalf("Put(alice): %v", err)
	}
	// Mallory syncs the SAME entity under her own href.
	mallory := Alias{Principal: "mallory", Collection: "tasks", Href: "m.ics", UID: "um", EntityID: "TSK-1"}
	if err := svc.Put(ctx, mallory); err != nil {
		t.Fatalf("Put(mallory): %v", err)
	}

	if _, ok := svc.Lookup("alice", "tasks", "a.ics"); !ok {
		t.Error("Mallory's Put evicted Alice's alias — her client would see the " +
			"to-do vanish and reappear under a new href")
	}
	if got, ok := svc.LookupByEntity("alice", "tasks", "TSK-1"); !ok || got.Href != "a.ics" {
		t.Errorf("Alice's entity lookup returned %+v, want her own href a.ics", got)
	}
	if got, ok := svc.LookupByEntity("mallory", "tasks", "TSK-1"); !ok || got.Href != "m.ics" {
		t.Errorf("Mallory's entity lookup returned %+v, want her own href m.ics", got)
	}
}
