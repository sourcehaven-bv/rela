package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// AC #1: a local create/update/delete pushes through /api/v1 and both ends
// converge. Under the fancy-browser model the PRIMARY mints the id on create,
// so the replica creates locally under a temp id, adopts the minted id, and
// renames its local doc (TKT-8P1TM7). The index records the agreed baseline; a
// re-push is a no-op.
func TestPush_CreateUpdateDelete_Converges(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Create locally under a temp id + push → primary mints the real id.
	h.createLocalEntity(t, "tmp-1", map[string]any{"title": "one"})
	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("push create: %v", err)
	}
	if rep.Applied != 1 || rep.Conflicts != 0 {
		t.Fatalf("push create: applied=%d conflicts=%d, want 1/0", rep.Applied, rep.Conflicts)
	}
	// The primary minted exactly one ticket; find its id.
	mintedID := h.onlyServerEntityID(t)
	if _, ok := h.server.entities[mintedID]; !ok {
		t.Fatalf("server missing minted entity %q after push", mintedID)
	}
	// The replica renamed its local doc to the minted id (temp id gone).
	if _, err := h.st.GetEntity(ctx, "tmp-1"); err == nil {
		t.Fatal("local temp id tmp-1 should be renamed away after adoption")
	}
	if _, err := h.st.GetEntity(ctx, mintedID); err != nil {
		t.Fatalf("local doc not renamed to minted id %q: %v", mintedID, err)
	}

	// Re-push with no local change → nothing to do (converged).
	rep, _ = h.engine.Push(ctx)
	if len(rep.Results) != 0 {
		t.Fatalf("re-push: got %d results, want 0 (converged)", len(rep.Results))
	}

	// Update + push (now an id-stable PATCH under the minted id).
	if err := h.st.UpdateEntity(ctx, &entity.Entity{ID: mintedID, Type: "ticket", Properties: map[string]any{"title": "two"}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	rep, _ = h.engine.Push(ctx)
	if rep.Applied != 1 {
		t.Fatalf("push update: applied=%d, want 1", rep.Applied)
	}
	if got := h.server.entities[mintedID].Properties["title"]; got != "two" {
		t.Fatalf("server title=%v, want two", got)
	}

	// Delete + push → mirrored remote delete.
	if _, err := h.st.DeleteEntity(ctx, mintedID, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rep, _ = h.engine.Push(ctx)
	if rep.Deleted != 1 {
		t.Fatalf("push delete: deleted=%d, want 1", rep.Deleted)
	}
	if _, ok := h.server.entities[mintedID]; ok {
		t.Fatalf("server still has %q after delete push", mintedID)
	}
	if _, ok := h.idx.Baseline(mintedID); ok {
		t.Fatalf("index still has %q after delete", mintedID)
	}
}

// TKT-8P1TM7 security crux, end-to-end: a redacted pull must NOT erase a hidden
// field the replica already holds. The primary hides `salary`; a prior wider
// pull left `salary` in the local store; a later redacted pull of a changed
// `title` must upsert title and PRESERVE the local salary (it is named in
// `_redacted`, so it is hidden-not-deleted).
func TestPull_RedactedField_PreservesLocalHiddenValue(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Seed the primary with both fields, then mark salary redacted for reads.
	h.server.seedEntity("TKT-R", "ticket", map[string]any{"title": "v1", "salary": 100})

	// Replica already holds salary locally (as if from an earlier wider-access
	// pull) — write it straight into the local store.
	if err := h.st.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-R", Type: "ticket", Properties: map[string]any{"title": "v1", "salary": 100},
	}); err != nil {
		t.Fatalf("seed local: %v", err)
	}
	// Baseline the index so the replica is "in sync" before the redacted change.
	h.idx.Set("TKT-R", "", canonicalOf(t, h, "TKT-R"), "ticket")

	// Now redaction turns on and the primary edits the visible title.
	h.server.hideField("ticket", "salary")
	h.server.mu.Lock()
	h.server.entities["TKT-R"].Properties["title"] = "v2"
	h.server.recordChange("e", "TKT-R", "ticket", false)
	h.server.mu.Unlock()

	if _, err := h.engine.Pull(ctx); err != nil {
		t.Fatalf("pull: %v", err)
	}

	got, err := h.st.GetEntity(ctx, "TKT-R")
	if err != nil {
		t.Fatalf("local TKT-R missing after pull: %v", err)
	}
	if got.Properties["title"] != "v2" {
		t.Fatalf("visible title not updated: %v", got.Properties["title"])
	}
	if got.Properties["salary"] != 100 {
		t.Fatalf("HIDDEN salary erased/changed by a redacted pull: %v (want 100 preserved)", got.Properties["salary"])
	}
}

// canonicalOf returns the canonical hash of a local entity, for seeding a
// baseline Local token in a test.
func canonicalOf(t *testing.T, h *harness, id string) string {
	t.Helper()
	e, err := h.st.GetEntity(context.Background(), id)
	if err != nil {
		t.Fatalf("hash local %s: %v", id, err)
	}
	return canonical.HashEntity(*e)
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

	// Establish a shared baseline: push a create so the primary mints an id and
	// the index baselines it. Subsequent edits are id-stable PATCHes on that id.
	h.createLocalEntity(t, "tmp-9", map[string]any{"title": "base"})
	if _, err := h.engine.Push(ctx); err != nil {
		t.Fatalf("baseline push: %v", err)
	}
	id := h.onlyServerEntityID(t)

	// Remote moves underneath us (someone else pushed a change).
	h.server.mu.Lock()
	h.server.entities[id].Properties["title"] = "remote-edit"
	h.server.recordChange("e", id, "ticket", false)
	h.server.mu.Unlock()

	// We also edit locally, then push → 412 conflict, halted.
	if err := h.st.UpdateEntity(ctx, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": "local-edit"}}); err != nil {
		t.Fatalf("local edit: %v", err)
	}
	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("conflicting push errored instead of halting: %v", err)
	}
	if rep.Conflicts != 1 || rep.Applied != 0 {
		t.Fatalf("push conflict: conflicts=%d applied=%d, want 1/0", rep.Conflicts, rep.Applied)
	}
	if got := h.server.entities[id].Properties["title"]; got != "remote-edit" {
		t.Fatalf("server overwritten despite conflict: title=%v", got)
	}

	// Force-push: local wins.
	res, err := h.engine.ForcePush(ctx, id)
	if err != nil {
		t.Fatalf("force push: %v", err)
	}
	if res.Outcome != OutcomePushed {
		t.Fatalf("force push outcome=%v, want pushed", res.Outcome)
	}
	if got := h.server.entities[id].Properties["title"]; got != "local-edit" {
		t.Fatalf("server title=%v after force, want local-edit", got)
	}
	// Index re-baselined → a subsequent push is a no-op.
	rep, _ = h.engine.Push(ctx)
	if len(rep.Results) != 0 {
		t.Fatalf("post-force push not converged: %d results", len(rep.Results))
	}
}

// BUG-ZWTDH9: a create-intent push that races a concurrent first-create of the
// same id gets a 409 from the server. That 409 must HALT ONLY that record (a
// conflict outcome), NOT abort the whole run — subsequent records still push.
// This mirrors the 412 halt-one-record contract; before the fix a 409 fell
// through to statusError and aborted the entire topo-ordered run.
func TestPush_CreateConflict409_HaltsOneRecordAndRunContinues(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Two brand-new local creates. The server 409s the FIRST create it receives
	// (as if a peer created it concurrently on the multi-writer backend); the
	// other create must still be applied in the same run. Under the minted-id
	// model both creates carry temp ids; the 409'd one keeps its temp id (not
	// adopted, not indexed) so a re-run replays it.
	h.createLocalEntity(t, "tmp-a", map[string]any{"title": "a"})
	h.createLocalEntity(t, "tmp-b", map[string]any{"title": "b"})
	h.server.mu.Lock()
	h.server.conflictOnceKey = "*create*" // 409 the first create, once
	h.server.mu.Unlock()

	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("409 must halt one record, not abort the run: %v", err)
	}
	if rep.Conflicts != 1 {
		t.Fatalf("conflicts=%d, want 1 (one create halted on 409)", rep.Conflicts)
	}
	if rep.Applied != 1 {
		t.Fatalf("applied=%d, want 1 (the other create must proceed past the halted one)", rep.Applied)
	}

	// Exactly one entity landed on the server (the non-conflicted create), and
	// exactly one temp id remains local (the halted create, not yet adopted).
	if len(h.server.entities) != 1 {
		t.Fatalf("server entities=%d, want 1 (only the non-conflicted create)", len(h.server.entities))
	}
	tempsRemaining := 0
	for _, id := range []string{"tmp-a", "tmp-b"} {
		if _, gerr := h.st.GetEntity(ctx, id); gerr == nil {
			tempsRemaining++
		}
	}
	if tempsRemaining != 1 {
		t.Fatalf("temp ids remaining=%d, want 1 (the halted create keeps its temp id)", tempsRemaining)
	}

	// The halted record carries the create-specific message (not the 412
	// base-changed wording).
	var halted *PushRecordResult
	for i := range rep.Results {
		if rep.Results[i].Outcome == OutcomeConflict {
			halted = &rep.Results[i]
		}
	}
	if halted == nil {
		t.Fatal("no conflict report entry for the halted create")
	}
	if !contains(halted.Detail, "created concurrently by a peer") {
		t.Fatalf("409 detail=%q, want the concurrent-create wording", halted.Detail)
	}

	// Re-run: the server no longer 409s, so the halted create now applies.
	rep2, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("re-run after 409 resolved: %v", err)
	}
	if rep2.Applied != 1 || rep2.Conflicts != 0 {
		t.Fatalf("re-run applied=%d conflicts=%d, want 1/0 (halted create now applies)", rep2.Applied, rep2.Conflicts)
	}
	if len(h.server.entities) != 2 {
		t.Fatalf("server entities=%d after re-run, want 2 (both creates landed)", len(h.server.entities))
	}
}

// AC #6: a relation and its two locally-created endpoints push in one run —
// entities before relations (orderForApply). Under the minted-id model the
// endpoints adopt primary-minted ids and RenameEntity remaps the relation's
// endpoints, so the pushed relation connects the two minted ids (RR-SYNCR2).
func TestPush_TopologicalOrder_EntitiesBeforeRelations(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Create a relation AND its endpoints locally under temp ids.
	h.createLocalEntity(t, "tmp-a", map[string]any{"title": "a"})
	h.createLocalEntity(t, "tmp-b", map[string]any{"title": "b"})
	rd := store.RelationData{Content: "link"}
	if _, err := h.st.CreateRelation(ctx, "tmp-a", "blocks", "tmp-b", &rd); err != nil {
		t.Fatalf("create relation: %v", err)
	}

	rep, err := h.engine.Push(ctx)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if rep.Applied != 3 {
		t.Fatalf("applied=%d, want 3 (2 entities + 1 relation)", rep.Applied)
	}
	// Two entities minted; exactly one relation on the server, connecting them.
	if len(h.server.entities) != 2 {
		t.Fatalf("server entities=%d, want 2", len(h.server.entities))
	}
	if len(h.server.relations) != 1 {
		t.Fatalf("server relations=%d, want 1", len(h.server.relations))
	}
	for key, rel := range h.server.relations {
		if _, ok := h.server.entities[rel.From]; !ok {
			t.Fatalf("relation %q FROM %q is not a minted server entity", key, rel.From)
		}
		if _, ok := h.server.entities[rel.To]; !ok {
			t.Fatalf("relation %q TO %q is not a minted server entity", key, rel.To)
		}
	}
}

// Idempotent replay: a mid-batch transport failure aborts the run, but the
// records applied before the failure are durably in the index, so a re-run
// resumes and converges.
func TestPush_MidBatchFailure_ResumesOnRerun(t *testing.T) {
	st := memstore.New()
	fs := newFakeServer()

	// A server that fails the SECOND create (POST) once with a 502, then recovers.
	createCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/") && r.URL.Path != "/api/v1/_schema" {
			createCount++
			if createCount == 2 {
				w.WriteHeader(http.StatusBadGateway) // transient failure, once
				return
			}
		}
		fs.handle(w, r)
	}))
	t.Cleanup(srv.Close)

	client, _ := NewClient(srv.URL, "", nil)
	idx := newState()
	eng, _ := NewEngine(client, st, memApplier{st: st}, idx)
	ctx := context.Background()

	for _, id := range []string{"tmp-a", "tmp-b"} {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	// First run: one create applies, the second fails → run aborts with an error.
	if _, err := eng.Push(ctx); err == nil {
		t.Fatal("expected mid-batch failure to surface as an error")
	}
	// Exactly one entity landed durably (on the server and in the index).
	if len(fs.entities) != 1 {
		t.Fatalf("server entities=%d after partial run, want 1", len(fs.entities))
	}
	indexed := 0
	for range idx.Records {
		indexed++
	}
	if indexed != 1 {
		t.Fatalf("indexed records=%d after partial run, want 1 (the applied create)", indexed)
	}

	// Re-run: the already-applied create is a no-op; the failed one now applies.
	rep, err := eng.Push(ctx)
	if err != nil {
		t.Fatalf("rerun push: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("rerun applied=%d, want 1 (only the previously-failed create)", rep.Applied)
	}
	if len(fs.entities) != 2 {
		t.Fatalf("server entities=%d after resume, want 2", len(fs.entities))
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
