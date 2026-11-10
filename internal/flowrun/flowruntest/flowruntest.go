// Package flowruntest is the conformance harness every [flowrun.Store]
// implementation must pass.
//
// It pins the rules a Go map and a SQL table are most likely to get
// differently: the compare-and-set on rev, the singleton and per-subject
// checks, the due order and its cursor, time precision, a rekey racing
// creates, and the definition snapshot against a concurrent prune.
//
// Every assertion is deterministic except the concurrency cases, which
// assert only outcomes that hold under every interleaving.
package flowruntest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// Harness is one fresh, empty store for one subtest.
type Harness struct {
	Store flowrun.Store
	// Second opens another store over the same storage, standing in for a
	// second process. Nil: accepted; the concurrency cases then race
	// goroutines on Store alone.
	Second func(t *testing.T) flowrun.Store
}

// Factory returns a fresh harness. Cleanup is the factory's
// responsibility, normally via t.Cleanup.
type Factory func(t *testing.T) Harness

// RunAll runs every conformance case against stores from open.
func RunAll(t *testing.T, open Factory) {
	t.Helper()
	cases := []struct {
		name string
		fn   func(*testing.T, Harness)
	}{
		{"CreateGet", testCreateGet},
		{"CreateRefusals", testCreateRefusals},
		{"Singleton", testSingleton},
		{"SubjectCap", testSubjectCap},
		{"Update", testUpdate},
		{"UpdateLogAndClaim", testUpdateLogAndClaim},
		{"Delete", testDelete},
		{"ListDue", testListDue},
		{"ListDueCursor", testListDueCursor},
		{"List", testList},
		{"RekeySubject", testRekeySubject},
		{"RekeyRefusals", testRekeyRefusals},
		{"Definitions", testDefinitions},
		{"InvalidArguments", testInvalidArguments},
		{"ConcurrentUpdate", testConcurrentUpdate},
		{"ConcurrentSingleton", testConcurrentSingleton},
		{"ConcurrentRekey", testConcurrentRekey},
		{"PruneRacesCreate", testPruneRacesCreate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t, open(t))
		})
	}
}

// base is an arbitrary fixed instant with sub-microsecond digits, so every
// case also pins the precision the store keeps.
var base = time.Date(2026, 11, 7, 9, 0, 0, 123456789, time.FixedZone("CET", 3600))

// at returns base plus n minutes.
func at(n int) time.Time { return base.Add(time.Duration(n) * time.Minute) }

const (
	v1 = "0123456789abcdef"
	v2 = "fedcba9876543210"
)

var def1 = []byte("states:\n  open: {}\n")

// newRun returns a valid non-singleton run of flow onboard on subject.
func newRun(id string, subject entity.Ref) flowrun.NewRun {
	return flowrun.NewRun{
		ID: flowrun.RunID(id), Flow: "onboard", Version: v1, Definition: def1,
		Subject: subject, At: at(0),
	}
}

// ref names a subject of a faceless type, at the implicit face.
func ref(id string) entity.Ref { return entity.Ref{ID: id, Face: entity.ImplicitFace} }

func create(t *testing.T, s flowrun.Store, nr flowrun.NewRun) flowrun.Run {
	t.Helper()
	r, err := s.Create(context.Background(), nr)
	require.NoError(t, err)
	return r
}

// fullInstance is an instance with every field set.
func fullInstance() flow.Instance {
	return flow.Instance{
		State: "review", Generation: 3, EnteredAt: at(5), Cause: 7, Previous: "draft", NextDueAt: at(65),
		Restarts: map[string]time.Time{"escalate": at(6)},
		Reminders: map[string]flow.ReminderRecord{
			"nudge": {Since: at(5), Last: at(35), Count: 2},
		},
	}
}

func ids(runs []flowrun.Run) []string {
	out := make([]string, 0, len(runs))
	for _, r := range runs {
		out = append(out, string(r.ID))
	}
	return out
}

func testCreateGet(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	nr := newRun("FR-1", entity.Ref{ID: "EMP-1", Face: "draft"})
	nr.Singleton = true
	nr.Instance = fullInstance()
	got := create(t, h.Store, nr)

	want := flowrun.Run{
		ID: "FR-1", Flow: "onboard", Version: v1, Subject: entity.Ref{ID: "EMP-1", Face: "draft"},
		Singleton: true, CreatedAt: flow.NormTime(at(0)), UpdatedAt: flow.NormTime(at(0)), Rev: 1,
		Instance: flowrun.NormInstance(fullInstance()),
	}
	require.Equal(t, want, got)
	require.Equal(t, time.UTC, got.CreatedAt.Location())

	read, err := h.Store.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, want, read)

	// An unset instance reads back unset, with nil maps.
	create(t, h.Store, newRun("FR-2", ref("EMP-2")))
	read, err = h.Store.Get(ctx, "FR-2")
	require.NoError(t, err)
	require.True(t, read.Instance.IsZero())
	require.Nil(t, read.Instance.Restarts)
	require.Nil(t, read.Instance.Reminders)
	require.False(t, read.Singleton)
	require.True(t, read.Subject.Face.IsImplicit())

	_, err = h.Store.Get(ctx, "FR-3")
	require.ErrorIs(t, err, flowrun.ErrNotFound)
}

func testCreateRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	create(t, h.Store, newRun("FR-1", ref("EMP-1")))

	_, err := h.Store.Create(ctx, newRun("FR-1", ref("EMP-2")))
	require.ErrorIs(t, err, flowrun.ErrExists)

	other := newRun("FR-2", ref("EMP-2"))
	other.Definition = []byte("states:\n  closed: {}\n")
	_, err = h.Store.Create(ctx, other)
	require.ErrorIs(t, err, flowrun.ErrDefinitionConflict)
	_, err = h.Store.Get(ctx, "FR-2")
	require.ErrorIs(t, err, flowrun.ErrNotFound, "a refused create stores nothing")

	// Another version, or the same version of another flow, is a new
	// definition.
	other.Version = v2
	create(t, h.Store, other)
	third := newRun("FR-3", ref("EMP-2"))
	third.Flow = "offboard"
	third.Definition = []byte("other")
	create(t, h.Store, third)
}

func testSingleton(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	single := func(id string, subject entity.Ref) flowrun.NewRun {
		nr := newRun(id, subject)
		nr.Singleton = true
		return nr
	}
	// Non-singleton runs never block, and do not block a singleton.
	create(t, h.Store, newRun("FR-1", ref("EMP-1")))
	create(t, h.Store, newRun("FR-2", ref("EMP-1")))
	create(t, h.Store, single("FR-3", ref("EMP-1")))

	_, err := h.Store.Create(ctx, single("FR-4", ref("EMP-1")))
	require.ErrorIs(t, err, flowrun.ErrSubjectBusy)
	// Every face of the subject id is one subject.
	_, err = h.Store.Create(ctx, single("FR-4", entity.Ref{ID: "EMP-1", Face: "draft"}))
	require.ErrorIs(t, err, flowrun.ErrSubjectBusy)
	// An id stored already is refused first.
	_, err = h.Store.Create(ctx, single("FR-3", ref("EMP-1")))
	require.ErrorIs(t, err, flowrun.ErrExists)

	// Another flow, or another subject, is free.
	nr := single("FR-5", ref("EMP-1"))
	nr.Flow = "offboard"
	create(t, h.Store, nr)
	create(t, h.Store, single("FR-6", ref("EMP-2")))

	// Deleting the singleton frees the subject.
	require.NoError(t, h.Store.Delete(ctx, "FR-3", 1))
	create(t, h.Store, single("FR-4", entity.Ref{ID: "EMP-1", Face: "draft"}))
}

func testSubjectCap(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	for i := range flowrun.MaxRunsPerSubject {
		subject := ref("EMP-1")
		if i%2 == 1 {
			subject.Face = "draft"
		}
		create(t, h.Store, newRun(fmt.Sprintf("FR-%d", i), subject))
	}
	_, err := h.Store.Create(ctx, newRun("FR-X", ref("EMP-1")))
	require.ErrorIs(t, err, flowrun.ErrSubjectFull)

	// Another flow on the subject, or the flow on another subject, is free.
	nr := newRun("FR-Y", ref("EMP-1"))
	nr.Flow = "offboard"
	create(t, h.Store, nr)
	create(t, h.Store, newRun("FR-Z", ref("EMP-2")))

	require.NoError(t, h.Store.Delete(ctx, "FR-0", 1))
	create(t, h.Store, newRun("FR-X", ref("EMP-1")))
}

func testUpdate(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	created := create(t, h.Store, newRun("FR-1", entity.Ref{ID: "EMP-1", Face: "draft"}))

	got, err := h.Store.Update(ctx, "FR-1", 1, flowrun.State{Instance: fullInstance()}, at(10))
	require.NoError(t, err)
	want := created
	want.Rev = 2
	want.UpdatedAt = flow.NormTime(at(10))
	want.Instance = flowrun.NormInstance(fullInstance())
	require.Equal(t, want, got)
	read, err := h.Store.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, want, read)

	_, err = h.Store.Update(ctx, "FR-1", 1, flowrun.State{Instance: flow.Instance{State: "done", Generation: 4}}, at(11))
	require.ErrorIs(t, err, flowrun.ErrConflict)
	read, err = h.Store.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, want, read, "a lost update changes nothing")

	// An update may clear the schedule.
	cleared := flow.Instance{State: "done", Generation: 4, EnteredAt: at(12)}
	got, err = h.Store.Update(ctx, "FR-1", 2, flowrun.State{Instance: cleared}, at(12))
	require.NoError(t, err)
	require.Equal(t, int64(3), got.Rev)
	require.Equal(t, flowrun.NormInstance(cleared), got.Instance)

	_, err = h.Store.Update(ctx, "FR-2", 1, flowrun.State{Instance: cleared}, at(12))
	require.ErrorIs(t, err, flowrun.ErrNotFound)

	// The pending-outputs mark is stored with the instance and cleared by
	// an update without one (RR-U30RNUZ).
	mark := &flowrun.Pending{State: "done", Generation: 4, Done: 1}
	got, err = h.Store.Update(ctx, "FR-1", 3, flowrun.State{Instance: cleared, Pending: mark}, at(13))
	require.NoError(t, err)
	require.Equal(t, mark, got.Pending)
	read, err = h.Store.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, mark, read.Pending)
	got, err = h.Store.Update(ctx, "FR-1", 4, flowrun.State{Instance: cleared}, at(14))
	require.NoError(t, err)
	require.Nil(t, got.Pending)
	read, err = h.Store.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Nil(t, read.Pending)
}

// testUpdateLogAndClaim pins that the timeline and the step claim are
// stored with the instance, and that an update without them clears them.
func testUpdateLogAndClaim(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	cleared := flow.Instance{State: "done", Generation: 4, EnteredAt: at(12)}
	nr := newRun("FR-P", ref("EMP-9"))
	nr.Pending = &flowrun.Pending{State: "open", Generation: 1}
	nr.Log = []flowrun.Step{{At: flow.NormTime(at(0)), State: "open", Via: flowrun.ViaStart, By: "alice"}}
	created := create(t, h.Store, nr)
	require.Equal(t, nr.Pending, created.Pending)
	require.Equal(t, nr.Log, created.Log)
	read, err := h.Store.Get(ctx, "FR-P")
	require.NoError(t, err)
	require.Equal(t, nr.Pending, read.Pending)
	require.Equal(t, nr.Log, read.Log)

	log := flowrun.AppendLog(nr.Log, flowrun.Step{At: flow.NormTime(at(1)), State: "done", From: "open",
		Via: flowrun.ViaChange})
	claim := &flowrun.Claim{Generation: 1, By: "bob", At: flow.NormTime(at(1))}
	got, err := h.Store.Update(ctx, "FR-P", 1, flowrun.State{Instance: cleared, Log: log, Claim: claim}, at(2))
	require.NoError(t, err)
	require.Equal(t, log, got.Log)
	require.Equal(t, claim, got.Claim)
	read, err = h.Store.Get(ctx, "FR-P")
	require.NoError(t, err)
	require.Equal(t, log, read.Log)
	require.Equal(t, claim, read.Claim)
	got, err = h.Store.Update(ctx, "FR-P", 2, flowrun.State{Instance: cleared}, at(3))
	require.NoError(t, err)
	require.Nil(t, got.Log)
	require.Nil(t, got.Claim)
}

func testDelete(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	create(t, h.Store, newRun("FR-1", ref("EMP-1")))
	_, err := h.Store.Update(ctx, "FR-1", 1, flowrun.State{Instance: fullInstance()}, at(1))
	require.NoError(t, err)

	require.ErrorIs(t, h.Store.Delete(ctx, "FR-1", 1), flowrun.ErrConflict)
	_, err = h.Store.Get(ctx, "FR-1")
	require.NoError(t, err, "a refused delete keeps the run")

	require.NoError(t, h.Store.Delete(ctx, "FR-1", 2))
	_, err = h.Store.Get(ctx, "FR-1")
	require.ErrorIs(t, err, flowrun.ErrNotFound)
	require.ErrorIs(t, h.Store.Delete(ctx, "FR-1", 2), flowrun.ErrNotFound)

	due, err := h.Store.ListDue(ctx, at(1000), flowrun.DueCursor{}, flowrun.MaxPage)
	require.NoError(t, err)
	require.Empty(t, due, "a deleted run is never due")
}

// seedDue creates runs with the given due offsets in minutes; a negative
// offset leaves the run without a due time.
func seedDue(t *testing.T, s flowrun.Store, dues map[string]int) {
	t.Helper()
	for id, d := range dues {
		nr := newRun(id, ref("EMP-"+id))
		nr.Instance = flow.Instance{State: "open", Generation: 1, EnteredAt: at(0)}
		if d >= 0 {
			nr.Instance.NextDueAt = at(d)
		}
		create(t, s, nr)
	}
}

func testListDue(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	seedDue(t, h.Store, map[string]int{"B": 10, "A": 10, "C": 5, "D": 20, "E": -1})

	due, err := h.Store.ListDue(ctx, at(10), flowrun.DueCursor{}, flowrun.MaxPage)
	require.NoError(t, err)
	require.Equal(t, []string{"C", "A", "B"}, ids(due), "due order, ties by id, until inclusive")
	require.Equal(t, flow.NormTime(at(5)), due[0].Instance.NextDueAt)

	due, err = h.Store.ListDue(ctx, at(10), flowrun.DueCursor{}, 2)
	require.NoError(t, err)
	require.Equal(t, []string{"C", "A"}, ids(due))

	due, err = h.Store.ListDue(ctx, at(4), flowrun.DueCursor{}, flowrun.MaxPage)
	require.NoError(t, err)
	require.NotNil(t, due)
	require.Empty(t, due)

	// An update that clears the due time takes the run off the list.
	_, err = h.Store.Update(ctx, "C", 1, flowrun.State{Instance: flow.Instance{State: "open", Generation: 1, EnteredAt: at(0)}}, at(6))
	require.NoError(t, err)
	due, err = h.Store.ListDue(ctx, at(100), flowrun.DueCursor{}, flowrun.MaxPage)
	require.NoError(t, err)
	require.Equal(t, []string{"A", "B", "D"}, ids(due))
}

// testListDueCursor: runs that stay due at the head do not starve the
// ones behind them when the caller passes the cursor on.
func testListDueCursor(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	seedDue(t, h.Store, map[string]int{"A": 1, "B": 1, "C": 1, "D": 2, "E": 3})

	var seen []string
	cursor := flowrun.DueCursor{}
	for range 10 {
		page, err := h.Store.ListDue(ctx, at(5), cursor, 2)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		seen = append(seen, ids(page)...)
		last := page[len(page)-1]
		cursor = flowrun.DueCursor{At: last.Instance.NextDueAt, ID: last.ID}
	}
	require.Equal(t, []string{"A", "B", "C", "D", "E"}, seen)

	// A cursor in another zone and with sub-microsecond digits reads as
	// its normalized instant.
	page, err := h.Store.ListDue(ctx, at(5), flowrun.DueCursor{At: at(1).In(time.UTC), ID: "B"}, flowrun.MaxPage)
	require.NoError(t, err)
	require.Equal(t, []string{"C", "D", "E"}, ids(page))
}

func testList(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	create(t, h.Store, newRun("FR-3", ref("EMP-1")))
	create(t, h.Store, newRun("FR-1", entity.Ref{ID: "EMP-1", Face: "draft"}))
	create(t, h.Store, newRun("FR-2", ref("EMP-2")))
	other := newRun("FR-4", ref("EMP-1"))
	other.Flow = "offboard"
	create(t, h.Store, other)

	list := func(q flowrun.Query) []string {
		t.Helper()
		if q.Limit == 0 {
			q.Limit = flowrun.MaxPage
		}
		runs, err := h.Store.List(ctx, q)
		require.NoError(t, err)
		require.NotNil(t, runs)
		return ids(runs)
	}
	require.Equal(t, []string{"FR-1", "FR-2", "FR-3", "FR-4"}, list(flowrun.Query{}))
	require.Equal(t, []string{"FR-1", "FR-2", "FR-3"}, list(flowrun.Query{Flow: "onboard"}))
	require.Equal(t, []string{"FR-1", "FR-3", "FR-4"}, list(flowrun.Query{SubjectID: "EMP-1"}))
	require.Equal(t, []string{"FR-4"}, list(flowrun.Query{Flow: "offboard", SubjectID: "EMP-1"}))
	require.Equal(t, []string{"FR-3", "FR-4"}, list(flowrun.Query{After: "FR-2"}))
	require.Equal(t, []string{"FR-1", "FR-2"}, list(flowrun.Query{Limit: 2}))
	require.Empty(t, list(flowrun.Query{After: "FR-4"}))
	require.Empty(t, list(flowrun.Query{SubjectID: "EMP-9"}))
}

func testRekeySubject(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	nr := newRun("FR-1", entity.Ref{ID: "EMP-1", Face: "draft"})
	nr.Singleton = true
	create(t, h.Store, nr)
	create(t, h.Store, newRun("FR-2", ref("EMP-1")))
	create(t, h.Store, newRun("FR-3", ref("EMP-2")))
	_, err := h.Store.Update(ctx, "FR-2", 1, flowrun.State{Instance: fullInstance()}, at(1))
	require.NoError(t, err)

	n, err := h.Store.RekeySubject(ctx, "EMP-1", "EMP-10")
	require.NoError(t, err)
	require.Equal(t, 2, n)

	moved, err := h.Store.List(ctx, flowrun.Query{SubjectID: "EMP-10", Limit: flowrun.MaxPage})
	require.NoError(t, err)
	require.Equal(t, []string{"FR-1", "FR-2"}, ids(moved))
	require.Equal(t, entity.Ref{ID: "EMP-10", Face: "draft"}, moved[0].Subject, "the face is kept")
	require.Equal(t, int64(2), moved[1].Rev, "the rev is kept")
	left, err := h.Store.List(ctx, flowrun.Query{SubjectID: "EMP-1", Limit: flowrun.MaxPage})
	require.NoError(t, err)
	require.Empty(t, left)

	// The moved singleton still blocks on its new subject.
	nr = newRun("FR-4", ref("EMP-10"))
	nr.Singleton = true
	_, err = h.Store.Create(ctx, nr)
	require.ErrorIs(t, err, flowrun.ErrSubjectBusy)

	// A repeat moves nothing.
	n, err = h.Store.RekeySubject(ctx, "EMP-1", "EMP-10")
	require.NoError(t, err)
	require.Zero(t, n)
}

func testRekeyRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	for _, id := range []string{"FR-1", "FR-2"} {
		nr := newRun(id, ref("EMP-"+id))
		nr.Singleton = true
		create(t, h.Store, nr)
	}
	create(t, h.Store, newRun("FR-3", ref("EMP-FR-1")))

	_, err := h.Store.RekeySubject(ctx, "EMP-FR-1", "EMP-FR-2")
	require.ErrorIs(t, err, flowrun.ErrSubjectBusy)
	left, err := h.Store.List(ctx, flowrun.Query{SubjectID: "EMP-FR-1", Limit: flowrun.MaxPage})
	require.NoError(t, err)
	require.Equal(t, []string{"FR-1", "FR-3"}, ids(left), "a refused rekey moves nothing")

	// The per-subject cap holds on the target.
	for i := range flowrun.MaxRunsPerSubject - 1 {
		create(t, h.Store, newRun(fmt.Sprintf("FR-C%d", i), ref("EMP-FULL")))
	}
	_, err = h.Store.RekeySubject(ctx, "EMP-FR-1", "EMP-FULL")
	require.ErrorIs(t, err, flowrun.ErrSubjectFull)
	left, err = h.Store.List(ctx, flowrun.Query{SubjectID: "EMP-FR-1", Limit: flowrun.MaxPage})
	require.NoError(t, err)
	require.Len(t, left, 2)
}

func testDefinitions(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	_, err := h.Store.Definition(ctx, "onboard", v1)
	require.ErrorIs(t, err, flowrun.ErrNotFound)

	create(t, h.Store, newRun("FR-1", ref("EMP-1"))) // v1, saved at(0)
	late := newRun("FR-2", ref("EMP-2"))
	late.Version, late.Definition, late.At = v2, []byte("late"), at(30)
	create(t, h.Store, late)

	got, err := h.Store.Definition(ctx, "onboard", v1)
	require.NoError(t, err)
	require.Equal(t, def1, got)

	// Both are in use: nothing goes.
	n, err := h.Store.PruneDefinitions(ctx, at(60))
	require.NoError(t, err)
	require.Zero(t, n)

	require.NoError(t, h.Store.Delete(ctx, "FR-1", 1))
	require.NoError(t, h.Store.Delete(ctx, "FR-2", 1))
	// v2 was saved at(30), not before at(30): it stays.
	n, err = h.Store.PruneDefinitions(ctx, at(30))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	_, err = h.Store.Definition(ctx, "onboard", v1)
	require.ErrorIs(t, err, flowrun.ErrNotFound)
	_, err = h.Store.Definition(ctx, "onboard", v2)
	require.NoError(t, err)

	// A new run on a pruned version stores it again, with any bytes.
	again := newRun("FR-3", ref("EMP-1"))
	again.Definition = []byte("again")
	create(t, h.Store, again)
	got, err = h.Store.Definition(ctx, "onboard", v1)
	require.NoError(t, err)
	require.Equal(t, []byte("again"), got)
}

func testInvalidArguments(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	create(t, h.Store, newRun("FR-1", ref("EMP-1")))
	mod := func(f func(*flowrun.NewRun)) flowrun.NewRun {
		nr := newRun("FR-9", ref("EMP-9"))
		f(&nr)
		return nr
	}
	creates := map[string]flowrun.NewRun{
		"empty id":         mod(func(nr *flowrun.NewRun) { nr.ID = "" }),
		"traversal id":     mod(func(nr *flowrun.NewRun) { nr.ID = "../x" }),
		"long id":          mod(func(nr *flowrun.NewRun) { nr.ID = flowrun.RunID(strings.Repeat("A", 256)) }),
		"flow name":        mod(func(nr *flowrun.NewRun) { nr.Flow = "Onboard" }),
		"short version":    mod(func(nr *flowrun.NewRun) { nr.Version = "abc" }),
		"upper version":    mod(func(nr *flowrun.NewRun) { nr.Version = "0123456789ABCDEF" }),
		"no definition":    mod(func(nr *flowrun.NewRun) { nr.Definition = nil }),
		"big definition":   mod(func(nr *flowrun.NewRun) { nr.Definition = make([]byte, flowrun.MaxDefinitionBytes+1) }),
		"subject id":       mod(func(nr *flowrun.NewRun) { nr.Subject.ID = "a b" }),
		"subject face":     mod(func(nr *flowrun.NewRun) { nr.Subject.Face = "Draft" }),
		"zero time":        mod(func(nr *flowrun.NewRun) { nr.At = time.Time{} }),
		"state name":       mod(func(nr *flowrun.NewRun) { nr.Instance.State = "Open State" }),
		"previous name":    mod(func(nr *flowrun.NewRun) { nr.Instance.Previous = "-x" }),
		"negative gen":     mod(func(nr *flowrun.NewRun) { nr.Instance.Generation = -1 }),
		"negative cause":   mod(func(nr *flowrun.NewRun) { nr.Instance.Cause = -1 }),
		"timer name":       mod(func(nr *flowrun.NewRun) { nr.Instance.Restarts = map[string]time.Time{"X": at(1)} }),
		"too many anchors": mod(func(nr *flowrun.NewRun) { nr.Instance.Restarts = names(flow.MaxTimers + 1) }),
		"reminder count": mod(func(nr *flowrun.NewRun) {
			nr.Instance.Reminders = map[string]flow.ReminderRecord{"nudge": {Count: -1}}
		}),
		"too many reminders": mod(func(nr *flowrun.NewRun) {
			nr.Instance.Reminders = map[string]flow.ReminderRecord{}
			for k := range names(flow.MaxReminders + 1) {
				nr.Instance.Reminders[k] = flow.ReminderRecord{}
			}
		}),
	}
	for name, nr := range creates {
		_, err := h.Store.Create(ctx, nr)
		require.ErrorIs(t, err, flowrun.ErrInvalid, name)
	}
	_, err := h.Store.Get(ctx, "FR-9")
	require.ErrorIs(t, err, flowrun.ErrNotFound, "no refused create stored a run")

	errs := map[string]error{
		"get id":            second(h.Store.Get(ctx, "")),
		"update rev":        second(h.Store.Update(ctx, "FR-1", 0, flowrun.State{Instance: flow.Instance{}}, at(1))),
		"update time":       second(h.Store.Update(ctx, "FR-1", 1, flowrun.State{Instance: flow.Instance{}}, time.Time{})),
		"update instance":   second(h.Store.Update(ctx, "FR-1", 1, flowrun.State{Instance: flow.Instance{State: "X"}}, at(1))),
		"update id":         second(h.Store.Update(ctx, "a/b", 1, flowrun.State{Instance: flow.Instance{}}, at(1))),
		"delete rev":        h.Store.Delete(ctx, "FR-1", 0),
		"delete id":         h.Store.Delete(ctx, "", 1),
		"due until":         second(h.Store.ListDue(ctx, time.Time{}, flowrun.DueCursor{}, 1)),
		"due limit 0":       second(h.Store.ListDue(ctx, at(1), flowrun.DueCursor{}, 0)),
		"due limit max":     second(h.Store.ListDue(ctx, at(1), flowrun.DueCursor{}, flowrun.MaxPage+1)),
		"due cursor":        second(h.Store.ListDue(ctx, at(1), flowrun.DueCursor{ID: "a b"}, 1)),
		"list limit":        second(h.Store.List(ctx, flowrun.Query{})),
		"list flow":         second(h.Store.List(ctx, flowrun.Query{Flow: "X", Limit: 1})),
		"list subject":      second(h.Store.List(ctx, flowrun.Query{SubjectID: "a b", Limit: 1})),
		"list after":        second(h.Store.List(ctx, flowrun.Query{After: "a b", Limit: 1})),
		"rekey old":         second(h.Store.RekeySubject(ctx, "", "EMP-2")),
		"rekey new":         second(h.Store.RekeySubject(ctx, "EMP-1", "../x")),
		"definition flow":   second(h.Store.Definition(ctx, "X", v1)),
		"definition vers":   second(h.Store.Definition(ctx, "onboard", "x")),
		"prune zero before": second(h.Store.PruneDefinitions(ctx, time.Time{})),
	}
	for name, err := range errs {
		require.ErrorIs(t, err, flowrun.ErrInvalid, name)
	}
	r, err := h.Store.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, int64(1), r.Rev, "no refused update wrote")
}

func names(n int) map[string]time.Time {
	out := make(map[string]time.Time, n)
	for i := range n {
		out[fmt.Sprintf("t%d", i)] = at(i)
	}
	return out
}

func second[T any](_ T, err error) error { return err }

// stores returns two stores over one storage: Store and, with Second, a
// store of another process.
func stores(t *testing.T, h Harness) []flowrun.Store {
	t.Helper()
	if h.Second == nil {
		return []flowrun.Store{h.Store, h.Store}
	}
	return []flowrun.Store{h.Store, h.Second(t)}
}

const racers = 8

func testConcurrentUpdate(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	create(t, h.Store, newRun("FR-1", ref("EMP-1")))
	ss := stores(t, h)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, conflicts := 0, 0
	for i := range racers {
		wg.Go(func() {
			inst := flow.Instance{State: "open", Generation: int64(i + 1)}
			_, err := ss[i%len(ss)].Update(ctx, "FR-1", 1, flowrun.State{Instance: inst}, at(i))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, flowrun.ErrConflict):
				conflicts++
			default:
				t.Errorf("update: %v", err)
			}
		})
	}
	wg.Wait()
	require.Equal(t, 1, wins)
	require.Equal(t, racers-1, conflicts)

	r, err := ss[len(ss)-1].Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, int64(2), r.Rev, "the other store sees the write")
}

func testConcurrentSingleton(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	ss := stores(t, h)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, busy := 0, 0
	for i := range racers {
		wg.Go(func() {
			nr := newRun(fmt.Sprintf("FR-%d", i), ref("EMP-1"))
			nr.Singleton = true
			_, err := ss[i%len(ss)].Create(ctx, nr)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, flowrun.ErrSubjectBusy):
				busy++
			default:
				t.Errorf("create: %v", err)
			}
		})
	}
	wg.Wait()
	require.Equal(t, 1, wins)
	require.Equal(t, racers-1, busy)
}

// testConcurrentRekey: a rekey racing singleton creates of another flow on
// the old and the new subject id never leaves two singleton runs of one
// flow on one subject id, and refuses only with ErrSubjectBusy.
func testConcurrentRekey(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	ss := stores(t, h)
	ok := func(op string, err error) {
		if err != nil && !errors.Is(err, flowrun.ErrSubjectBusy) {
			t.Errorf("%s: %v", op, err)
		}
	}
	for round := range racers {
		oldID, newID := fmt.Sprintf("EMP-O%d", round), fmt.Sprintf("EMP-N%d", round)
		create(t, h.Store, newRun(fmt.Sprintf("FR-%d-seed", round), ref(oldID)))
		var wg sync.WaitGroup
		wg.Go(func() {
			_, err := ss[round%len(ss)].RekeySubject(ctx, oldID, newID)
			ok("rekey", err)
		})
		for i, subject := range []string{oldID, newID} {
			wg.Go(func() {
				nr := newRun(fmt.Sprintf("FR-%d-%d", round, i), ref(subject))
				nr.Flow, nr.Singleton = "audit", true
				_, err := ss[(round+i+1)%len(ss)].Create(ctx, nr)
				ok("create", err)
			})
		}
		wg.Wait()

		for _, subject := range []string{oldID, newID} {
			runs, err := h.Store.List(ctx, flowrun.Query{Flow: "audit", SubjectID: subject, Limit: flowrun.MaxPage})
			require.NoError(t, err)
			require.LessOrEqual(t, len(runs), 1, "round %d: singleton runs of audit on %s", round, subject)
		}
	}
}

// testPruneRacesCreate: a prune running beside creates never leaves a
// stored run without its definition.
func testPruneRacesCreate(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	ss := stores(t, h)
	// A used and an unused definition before the race.
	create(t, h.Store, newRun("FR-0", ref("EMP-0")))
	seed := newRun("FR-S", ref("EMP-S"))
	seed.Version = v2
	create(t, h.Store, seed)
	require.NoError(t, h.Store.Delete(ctx, "FR-S", 1))

	var wg sync.WaitGroup
	for i := range racers {
		wg.Go(func() {
			nr := newRun(fmt.Sprintf("FR-%d", i+1), ref(fmt.Sprintf("EMP-%d", i+1)))
			if i%2 == 1 {
				nr.Version = v2
			}
			if _, err := ss[i%len(ss)].Create(ctx, nr); err != nil {
				t.Errorf("create: %v", err)
			}
		})
		wg.Go(func() {
			if _, err := ss[(i+1)%len(ss)].PruneDefinitions(ctx, at(1000)); err != nil {
				t.Errorf("prune: %v", err)
			}
		})
	}
	wg.Wait()

	runs, err := h.Store.List(ctx, flowrun.Query{Limit: flowrun.MaxPage})
	require.NoError(t, err)
	require.Len(t, runs, racers+1)
	for _, r := range runs {
		_, err := h.Store.Definition(ctx, r.Flow, r.Version)
		require.NoError(t, err, "run %s lost its definition %s", r.ID, r.Version)
	}
}
