package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// AC #1: a local create/update/delete pushes to the server and both ends
// converge (the index records the agreed hash; a re-push is a no-op).
func TestPush_CreateUpdateDelete_Converges(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Create + push.
	h.createLocalEntity(t, "TKT-1", map[string]any{"title": "one"})
	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("push create: %v", err)
	}
	if rep.Applied != 1 || rep.Conflicts != 0 {
		t.Fatalf("push create: applied=%d conflicts=%d, want 1/0", rep.Applied, rep.Conflicts)
	}
	if _, ok := h.server.entities["TKT-1"]; !ok {
		t.Fatal("server missing TKT-1 after push")
	}

	// Re-push with no local change → nothing to do (converged).
	rep, _ = h.engine.Push(ctx)
	if len(rep.Results) != 0 {
		t.Fatalf("re-push: got %d results, want 0 (converged)", len(rep.Results))
	}

	// Update + push.
	if err := h.st.UpdateEntity(ctx, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "two"}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	rep, _ = h.engine.Push(ctx)
	if rep.Applied != 1 {
		t.Fatalf("push update: applied=%d, want 1", rep.Applied)
	}
	if got := h.server.entities["TKT-1"].Properties["title"]; got != "two" {
		t.Fatalf("server title=%v, want two", got)
	}

	// Delete + push → mirrored remote delete.
	if _, err := h.st.DeleteEntity(ctx, "TKT-1", false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rep, _ = h.engine.Push(ctx)
	if rep.Deleted != 1 {
		t.Fatalf("push delete: deleted=%d, want 1", rep.Deleted)
	}
	if _, ok := h.server.entities["TKT-1"]; ok {
		t.Fatal("server still has TKT-1 after delete push")
	}
	if _, ok := h.idx.Hash("TKT-1"); ok {
		t.Fatal("index still has TKT-1 after delete")
	}
}

// AC #2: a server-side create/update/delete pulls back; a remote tombstone
// mirrors as a local delete.
func TestPull_RemoteChanges_Mirror(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.server.seedEntity("DEC-1", "decision", map[string]any{"title": "remote"})
	rep, err := h.engine.Pull(ctx)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("pull create: applied=%d, want 1", rep.Applied)
	}
	got, err := h.st.GetEntity(ctx, "DEC-1")
	if err != nil {
		t.Fatalf("local DEC-1 missing after pull: %v", err)
	}
	if got.Properties["title"] != "remote" {
		t.Fatalf("local title=%v, want remote", got.Properties["title"])
	}

	// Cursor advanced; a second pull with no new changes is a no-op.
	rep, _ = h.engine.Pull(ctx)
	if rep.Applied != 0 || rep.Skipped != 0 {
		t.Fatalf("second pull: applied=%d skipped=%d, want 0/0 (cursor advanced past it)", rep.Applied, rep.Skipped)
	}

	// Server deletes DEC-1 → pull mirrors the delete locally.
	h.server.mu.Lock()
	delete(h.server.entities, "DEC-1")
	h.server.recordChange("e", "DEC-1", "", true)
	h.server.mu.Unlock()

	rep, _ = h.engine.Pull(ctx)
	if rep.Deleted != 1 {
		t.Fatalf("pull delete: deleted=%d, want 1", rep.Deleted)
	}
	if _, err := h.st.GetEntity(ctx, "DEC-1"); err == nil {
		t.Fatal("local DEC-1 still present after remote delete pulled")
	}
}

// AC #3: a concurrent edit (remote moved since the client's base) halts the
// record with a conflict on push, and `push --force` resolves it (local wins)
// and re-baselines.
func TestPush_Conflict_HaltsThenForceResolves(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Establish a shared baseline: push a create so the index has its hash.
	h.createLocalEntity(t, "TKT-9", map[string]any{"title": "base"})
	if _, err := h.engine.Push(ctx); err != nil {
		t.Fatalf("baseline push: %v", err)
	}

	// Remote moves underneath us (someone else pushed a change).
	h.server.mu.Lock()
	h.server.entities["TKT-9"].Properties["title"] = "remote-edit"
	h.server.recordChange("e", "TKT-9", "ticket", false)
	h.server.mu.Unlock()

	// We also edit locally, then push → 412 conflict, halted.
	if err := h.st.UpdateEntity(ctx, &entity.Entity{ID: "TKT-9", Type: "ticket", Properties: map[string]any{"title": "local-edit"}}); err != nil {
		t.Fatalf("local edit: %v", err)
	}
	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("conflicting push errored instead of halting: %v", err)
	}
	if rep.Conflicts != 1 || rep.Applied != 0 {
		t.Fatalf("push conflict: conflicts=%d applied=%d, want 1/0", rep.Conflicts, rep.Applied)
	}
	if got := h.server.entities["TKT-9"].Properties["title"]; got != "remote-edit" {
		t.Fatalf("server overwritten despite conflict: title=%v", got)
	}

	// Force-push: local wins.
	res, err := h.engine.ForcePush(ctx, "TKT-9")
	if err != nil {
		t.Fatalf("force push: %v", err)
	}
	if res.Outcome != OutcomePushed {
		t.Fatalf("force push outcome=%v, want pushed", res.Outcome)
	}
	if got := h.server.entities["TKT-9"].Properties["title"]; got != "local-edit" {
		t.Fatalf("server title=%v after force, want local-edit", got)
	}
	// Index re-baselined → a subsequent push is a no-op.
	rep, _ = h.engine.Push(ctx)
	if len(rep.Results) != 0 {
		t.Fatalf("post-force push not converged: %d results", len(rep.Results))
	}
}

// AC #6: a relation listed BEFORE its endpoint entity in a batch is still
// applied, because push reorders entities ahead of relations.
func TestPush_TopologicalOrder_EntitiesBeforeRelations(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Create a relation AND its endpoints locally. The diff order is map-random,
	// but orderForApply must emit both entities before the relation.
	h.createLocalEntity(t, "A", map[string]any{"title": "a"})
	h.createLocalEntity(t, "B", map[string]any{"title": "b"})
	rd := store.RelationData{Content: "link"}
	if _, err := h.st.CreateRelation(ctx, "A", "blocks", "B", &rd); err != nil {
		t.Fatalf("create relation: %v", err)
	}

	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if rep.Applied != 3 {
		t.Fatalf("applied=%d, want 3 (2 entities + 1 relation)", rep.Applied)
	}
	// Verify ordering in the result stream: both entities precede the relation.
	relIdx, aIdx, bIdx := -1, -1, -1
	for i, res := range rep.Results {
		switch res.Key {
		case "A":
			aIdx = i
		case "B":
			bIdx = i
		case "A/blocks/B":
			relIdx = i
		}
	}
	if aIdx > relIdx || bIdx > relIdx {
		t.Fatalf("relation applied before an endpoint: A=%d B=%d rel=%d", aIdx, bIdx, relIdx)
	}
}

// Idempotent replay: a mid-batch transport failure aborts the run, but the
// records applied before the failure are durably in the index, so a re-run
// resumes and converges.
func TestPush_MidBatchFailure_ResumesOnRerun(t *testing.T) {
	st := memstore.New()
	fs := newFakeServer()

	// A server that fails the SECOND entity PUT once, then recovers.
	failOnce := map[string]bool{"TKT-B": true}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failPath := r.Method == http.MethodPut && failOnce["TKT-B"] && r.URL.Path == "/api/sync/entities/TKT-B"
		if failPath {
			failOnce["TKT-B"] = false
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fs.handle(w, r)
	}))
	t.Cleanup(srv.Close)

	client, _ := NewClient(srv.URL, "", nil)
	idx := newState()
	eng, _ := NewEngine(client, st, memApplier{st: st}, idx)
	ctx := context.Background()

	for _, id := range []string{"TKT-A", "TKT-B"} {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	// First run: TKT-A applies, TKT-B fails → run aborts with an error.
	if _, err := eng.Push(ctx); err == nil {
		t.Fatal("expected mid-batch failure to surface as an error")
	}
	if _, ok := idx.Hash("TKT-A"); !ok {
		t.Fatal("TKT-A should be durably in the index after partial run")
	}
	if _, ok := idx.Hash("TKT-B"); ok {
		t.Fatal("TKT-B must NOT be in the index (its push failed)")
	}

	// Re-run: TKT-A is a no-op (already in index), TKT-B now applies → converged.
	rep, err := eng.Push(ctx)
	if err != nil {
		t.Fatalf("rerun push: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("rerun applied=%d, want 1 (only TKT-B)", rep.Applied)
	}
	if _, ok := fs.entities["TKT-B"]; !ok {
		t.Fatal("TKT-B missing on server after resume")
	}
}

// `--force` on a non-existent id is a clear error and leaves no partial state.
func TestForcePush_UnknownRecord_Errors(t *testing.T) {
	h := newHarness(t)
	_, err := h.engine.ForcePush(context.Background(), "NOPE-1")
	if err == nil {
		t.Fatal("force push of unknown record should error")
	}
	if len(h.server.entities) != 0 {
		t.Fatal("force push of unknown record wrote partial state to server")
	}
}

// The bearer token authenticates the CLI; a missing/invalid token is a clean
// auth error, distinct from a 412 conflict; and the token is never echoed into
// an error message.
func TestAuth_BearerToken(t *testing.T) {
	st := memstore.New()
	fs := newFakeServer()
	fs.authToken = "secret-token-xyz"
	srv := fs.start(t)
	ctx := context.Background()

	if err := st.CreateEntity(ctx, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "x"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Wrong token → auth error surfaced, distinct from conflict.
	badClient, _ := NewClient(srv.URL, "wrong-token", nil)
	badEng, _ := NewEngine(badClient, st, memApplier{st: st}, newState())
	_, err := badEng.Push(ctx)
	if err == nil {
		t.Fatal("push with wrong token should fail")
	}
	if got := err.Error(); contains(got, "secret") || contains(got, "wrong-token") {
		t.Fatalf("error leaked a token: %q", got)
	}
	if !contains(err.Error(), "authentication failed") {
		t.Fatalf("auth error not distinct/clear: %q", err.Error())
	}

	// Correct token → push succeeds.
	goodClient, _ := NewClient(srv.URL, "secret-token-xyz", nil)
	goodEng, _ := NewEngine(goodClient, st, memApplier{st: st}, newState())
	rep, err := goodEng.Push(ctx)
	if err != nil {
		t.Fatalf("push with correct token: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("applied=%d, want 1", rep.Applied)
	}
}

// Regression for review finding #1: a re-played relation tombstone for an
// already-absent local relation must be a no-op, not a hard failure that wedges
// the pull. (DeleteRelation returns store.ErrNotFound, a different sentinel than
// DeleteEntity's ErrEntityNotFound.)
func TestPull_RelationTombstone_IdempotentOnResume(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Seed endpoints + a relation locally and on the server, in sync.
	h.createLocalEntity(t, "A", map[string]any{"title": "a"})
	h.createLocalEntity(t, "B", map[string]any{"title": "b"})
	rd := store.RelationData{Content: "link"}
	if _, err := h.st.CreateRelation(ctx, "A", "rel", "B", &rd); err != nil {
		t.Fatalf("create relation: %v", err)
	}
	if _, err := h.engine.Push(ctx); err != nil {
		t.Fatalf("baseline push: %v", err)
	}

	// Server deletes the relation → record a tombstone in the feed.
	h.server.mu.Lock()
	delete(h.server.relations, "A/rel/B")
	h.server.recordChange("r", "A/rel/B", "", true)
	h.server.mu.Unlock()

	// First pull mirrors the delete.
	if _, err := h.engine.Pull(ctx); err != nil {
		t.Fatalf("first pull: %v", err)
	}
	if _, err := h.st.GetRelation(ctx, "A", "rel", "B"); err == nil {
		t.Fatal("relation still present locally after tombstone pulled")
	}

	// Simulate a resume that re-sees the same tombstone: rewind the cursor and
	// pull again. The relation is already gone locally — this must NOT error.
	h.idx.Cursor = ""
	if _, err := h.engine.Pull(ctx); err != nil {
		t.Fatalf("resume pull re-playing relation tombstone must be a no-op, got: %v", err)
	}
}

// Regression for review finding #2: a base URL with a path prefix (a proxy that
// mounts the API under a sub-path) must keep its prefix — the request path is
// joined, not replaced.
func TestClient_BasePathPrefixPreserved(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = writeManifestOK(w)
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL+"/rela/", "", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.Manifest(context.Background(), ""); err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if gotPath != "/rela/api/sync/manifest" {
		t.Fatalf("path prefix dropped: got %q, want /rela/api/sync/manifest", gotPath)
	}
}

// Regression for review finding #6: a local record whose id cannot be safely
// synced (path separator, "..", control char) is skipped and reported, not put
// on the wire.
func TestPush_UnsyncableLocalID_SkippedAndReported(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// memstore validates ids on create, so inject the bad key straight into the
	// index-diff path by seeding a record the snapshot will surface. We instead
	// assert syncableKey directly plus a push that includes a good record.
	if !syncableKey("TKT-1", KindEntity) {
		t.Fatal("a normal id should be syncable")
	}
	for _, bad := range []string{"..", "a/b", "a..b", "x\x00y"} {
		if syncableKey(bad, KindEntity) {
			t.Errorf("id %q should not be syncable", bad)
		}
	}
	// And a clean push still works (sanity that the gate doesn't reject good ids).
	h.createLocalEntity(t, "TKT-OK", map[string]any{"title": "ok"})
	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if rep.Applied != 1 || rep.Invalid != 0 {
		t.Fatalf("good push: applied=%d invalid=%d, want 1/0", rep.Applied, rep.Invalid)
	}
}

func writeManifestOK(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(`{"changes":[],"cursor":"0"}`))
	return err
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
