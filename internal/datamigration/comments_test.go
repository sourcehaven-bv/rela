package datamigration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Migration writes go below the entitymanager, so they must keep comment
// threads at the address of the content themselves (BUG-6OZBP9). These tests
// assert WHERE each thread ends up, not just that the step succeeded: a
// thread left at the old address is invisible until a row is recreated there,
// so a step that forgets it still reports success.

// threadFixture is a comment service over memcomments plus a raw handle on
// its store, so assertions read storage rather than the service under test.
type threadFixture struct {
	svc *comments.Service
	st  comments.Store
}

func newThreadFixture(t *testing.T) threadFixture {
	t.Helper()
	st := memcomments.New()
	// Each comment a second after the last, so a thread lists in the order
	// the test wrote it rather than by random comment id.
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	svc, err := comments.NewService(st, func() time.Time {
		at = at.Add(time.Second)
		return at
	})
	if err != nil {
		t.Fatal(err)
	}
	return threadFixture{svc: svc, st: st}
}

func (f threadFixture) add(t *testing.T, typ, id string, face entity.Face, body string) {
	t.Helper()
	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice@example.com", Tool: "data-entry"})
	if _, err := f.svc.Add(ctx, comments.Target{Type: typ, ID: id, Face: face}, comments.AddRequest{
		Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "title"},
		Body:   body,
	}); err != nil {
		t.Fatalf("seed comment on %s: %v", entity.FormatStateRef(id, face), err)
	}
}

// bodies returns the comment bodies stored at one address.
func (f threadFixture) bodies(t *testing.T, id string, face entity.Face) []string {
	t.Helper()
	list, err := f.st.List(context.Background(), comments.Target{ID: id, Face: face})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.Body)
	}
	return out
}

func (f threadFixture) wantBodies(t *testing.T, id string, face entity.Face, want ...string) {
	t.Helper()
	got := f.bodies(t, id, face)
	if len(got) != len(want) {
		t.Fatalf("%s: comments = %q, want %q", entity.FormatStateRef(id, face), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: comments = %q, want %q", entity.FormatStateRef(id, face), got, want)
		}
	}
}

func TestRenameFace_MovesCommentThreads(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-1", Type: "task", Face: "nl", Properties: map[string]any{"title": "een"},
	}); err != nil {
		t.Fatal(err)
	}
	threads := newThreadFixture(t)
	threads.add(t, "task", "TSK-1", "nl", "on nl")
	threads.add(t, "task", "TSK-1", "", "on the bare row")

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory(), Comments: threads.svc})
	f := mustParse(t, testName("face"), mustFileYAML(t,
		facedMeta("en", "nl"), facedMeta("en", "nl-be"),
		"  - rename_face: {entity: task, from: nl, to: nl-be}\n"))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	threads.wantBodies(t, "TSK-1", "nl-be", "on nl")
	threads.wantBodies(t, "TSK-1", "nl")
	threads.wantBodies(t, "TSK-1", "", "on the bare row")
}

func TestMigrateFace_MovesCommentThreads(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	threads := newThreadFixture(t)
	// Written before the type declared faces, so stored at the bare id.
	threads.add(t, "task", "TSK-1", "", "on open task")
	threads.add(t, "task", "TSK-3", "", "on done task")

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory(), Comments: threads.svc})
	f := mustParse(t, testName("faces"), mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	threads.wantBodies(t, "TSK-1", "draft", "on open task")
	threads.wantBodies(t, "TSK-1", "")
	threads.wantBodies(t, "TSK-3", "published", "on done task")
	threads.wantBodies(t, "TSK-3", "")
}

func TestMigrateFace_DryRunLeavesCommentThreads(t *testing.T) {
	st := seedStore(t)
	threads := newThreadFixture(t)
	threads.add(t, "task", "TSK-1", "", "on open task")

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory(), Comments: threads.svc})
	f := mustParse(t, testName("faces"), mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(t.Context(), []*File{f}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	threads.wantBodies(t, "TSK-1", "", "on open task")
	threads.wantBodies(t, "TSK-1", "draft")
}

// failingThreads refuses every call, standing in for a comment store that is
// down.
type failingThreads struct{}

func (failingThreads) FaceMoved(context.Context, string, string, entity.Face, entity.Face) error {
	return errThreadsDown
}

func (failingThreads) EntityDeleted(context.Context, string) error { return errThreadsDown }

var errThreadsDown = errors.New("comment store unavailable")

// A thread that cannot be moved must stop the row move. If the row moved
// anyway, a re-run would no longer list it and the thread would be stranded
// for good.
func TestMigrateFace_ThreadFailureStopsTheRowMove(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory(), Comments: failingThreads{}})
	f := mustParse(t, testName("faces"), mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(ctx, []*File{f}, true); err == nil {
		t.Fatal("Run succeeded although the comment threads could not be moved")
	}
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1"}); err != nil {
		t.Errorf("TSK-1 left the zero coordinate although its thread did not move: %v", err)
	}
}

func TestAdopt_MovesCommentThreads(t *testing.T) {
	st := seedStore(t)
	threads := newThreadFixture(t)
	threads.add(t, "task", "TSK-2", "", "on wip task")

	deps := adoptDeps(t, st, "draft", "published")
	deps.Comments = threads.svc
	if _, err := Adopt(t.Context(), deps, AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft", "wip": "draft", "done": "published"},
		Apply:    true,
	}); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	threads.wantBodies(t, "TSK-2", "draft", "on wip task")
	threads.wantBodies(t, "TSK-2", "")
}

func TestDropEntities_DropsCommentThreads(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	for _, face := range []entity.Face{"draft", "published"} {
		if err := st.CreateEntity(ctx, &entity.Entity{
			ID: "PER-F", Type: "person", Face: face, Properties: map[string]any{"name": "f"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	threads := newThreadFixture(t)
	threads.add(t, "person", "PER-F", "draft", "on draft")
	threads.add(t, "person", "PER-F", "published", "on published")
	threads.add(t, "task", "TSK-1", "", "on a surviving task")

	from := metaV1()
	to := metaV1()
	delete(to.Entities, "person")
	f := mustParse(t, testName("drop"), mustFileYAML(t, from, to, "  - drop_entities: {type: person}\n"))
	r := newTestRunner(t, Deps{Store: st, Meta: to, Comments: threads.svc})
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("apply: %v", err)
	}

	threads.wantBodies(t, "PER-F", "draft")
	threads.wantBodies(t, "PER-F", "published")
	threads.wantBodies(t, "TSK-1", "", "on a surviving task")
}

func TestDropEntities_DryRunLeavesCommentThreads(t *testing.T) {
	st := seedStore(t)
	threads := newThreadFixture(t)
	threads.add(t, "person", "PER-1", "", "on a person")

	from := metaV1()
	to := metaV1()
	delete(to.Entities, "person")
	f := mustParse(t, testName("drop"), mustFileYAML(t, from, to, "  - drop_entities: {type: person}\n"))
	r := newTestRunner(t, Deps{Store: st, Meta: to, Comments: threads.svc})
	if _, err := r.Run(t.Context(), []*File{f}, false); err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	threads.wantBodies(t, "PER-1", "", "on a person")
}

// The GC sweep reaches drop_entities through its own executor, so it needs
// its own wiring of the comment service.
func TestGC_DropsCommentThreadsOfCollectedEntities(t *testing.T) {
	st, kv, gate, m2 := driftSetup(t)
	threads := newThreadFixture(t)
	threads.add(t, "person", "PER-1", "", "on a dropped person")
	threads.add(t, "task", "TSK-1", "", "on a surviving task")

	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m2 },
		Verdicts: gate, Comments: threads.svc,
	})
	// The first tick records the drift; the second, past the grace period,
	// collects it.
	if _, err := g.Tick(t.Context(), true); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	g.now = func() time.Time { return time.Now().Add(DefaultGrace + time.Hour) }
	if _, err := g.Tick(t.Context(), true); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if _, err := st.GetEntity(t.Context(), entity.Ref{ID: "PER-1"}); err == nil {
		t.Fatal("precondition: PER-1 survived the GC, so this test asserts nothing")
	}

	threads.wantBodies(t, "PER-1", "")
	threads.wantBodies(t, "TSK-1", "", "on a surviving task")
}

// A move refused because a DIFFERENT row holds the destination must leave both
// threads where they are. Moving first would merge the stranded row's remarks
// into the other row's thread, in front of that face's readers.
func TestAdopt_CollisionLeavesCommentThreads(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-2", Type: "task", Face: "draft", Properties: map[string]any{"title": "someone else's draft"},
	}); err != nil {
		t.Fatal(err)
	}
	threads := newThreadFixture(t)
	threads.add(t, "task", "TSK-2", "", "on the stranded row")
	threads.add(t, "task", "TSK-2", "draft", "on the other draft")

	deps := adoptDeps(t, st, "draft", "published")
	deps.Comments = threads.svc
	if _, err := Adopt(ctx, deps, AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft", "wip": "draft", "done": "published"},
		Apply:    true,
	}); err == nil {
		t.Fatal("Adopt succeeded although TSK-2's destination holds different content")
	}

	threads.wantBodies(t, "TSK-2", "", "on the stranded row")
	threads.wantBodies(t, "TSK-2", "draft", "on the other draft")
}

// txFailsOnce fails its first transaction, standing in for a row move that
// breaks after the comment threads already moved.
type txFailsOnce struct {
	store.Store
	failed bool
}

var errTxDown = errors.New("transaction failed")

func (s *txFailsOnce) Tx(ctx context.Context, fn func(store.Store) error) error {
	if !s.failed {
		s.failed = true
		return errTxDown
	}
	return s.Store.Tx(ctx, fn)
}

// The order moveThreads relies on: a thread that moved ahead of a failed row
// move waits at the destination face, and the re-run brings the row to it.
func TestMigrateFace_RerunConvergesAfterTheRowMoveFails(t *testing.T) {
	st := &txFailsOnce{Store: seedStore(t)}
	ctx := t.Context()
	threads := newThreadFixture(t)
	threads.add(t, "task", "TSK-1", "", "on open task")

	f := mustParse(t, testName("faces"), mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory(), Comments: threads.svc})
	if _, err := r.Run(ctx, []*File{f}, true); !errors.Is(err, errTxDown) {
		t.Fatalf("first Run: err = %v, want %v", err, errTxDown)
	}
	threads.wantBodies(t, "TSK-1", "draft", "on open task")
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1"}); err != nil {
		t.Fatalf("precondition: TSK-1 left the zero coordinate although its move failed: %v", err)
	}

	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1", Face: "draft"}); err != nil {
		t.Fatalf("TSK-1 is not at its draft face after the re-run: %v", err)
	}
	threads.wantBodies(t, "TSK-1", "draft", "on open task")
	threads.wantBodies(t, "TSK-1", "")
}
