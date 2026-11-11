package remotesync

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

func pushMapping(t *testing.T, tg *target, edit func(p *Remote)) *Mapping {
	t.Helper()
	return testMapping(t, tg.meta, func(p *Remote) {
		p.Push = true
		if edit != nil {
			edit(p)
		}
	})
}

func runSync(t *testing.T, tg *target, m *Mapping, src *pushSource, o Options) Report {
	t.Helper()
	rep, err := Run(context.Background(), m, tg.pushDeps(src), o)
	if err != nil {
		t.Fatalf("Run: %v (report %+v)", err, rep)
	}
	if rep.Pushed == nil {
		t.Fatalf("a remote with push reports no push: %+v", rep)
	}
	return rep
}

func version(t *testing.T, tg *target, id string) store.EntityVersion {
	t.Helper()
	for _, typ := range []string{"requirement", "component"} {
		for e, err := range tg.st.ListEntities(context.Background(), store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
			if err != nil {
				t.Fatal(err)
			}
			if e.ID == id {
				return store.VersionOf(e)
			}
		}
	}
	t.Fatalf("no local row %s", id)
	return ""
}

func patch(t *testing.T, tg *target, id string, props map[string]any) {
	t.Helper()
	if _, err := tg.mgr.PatchEntity(context.Background(), id, entity.Patch{Properties: props}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PushNeedsPusher(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	if _, err := Run(context.Background(), m, tg.deps(seedSource()), Options{}); err == nil {
		t.Fatal("a remote with push ran without a pusher")
	}
}

// TestPush_SendsChangesAndStopsTheEcho pins the update push and the echo
// stop: a pushed value becomes the merge base, so the pull that sees it
// come back writes nothing, and the run after that touches nothing.
func TestPush_SendsChangesAndStopsTheEcho(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	if rep := runSync(t, tg, m, src, Options{}); rep.Entities.Created != 4 || *rep.Pushed != (PushCounts{}) {
		t.Fatalf("first run: %+v %+v", rep, rep.Pushed)
	}
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Login v2", "note": "local only"})

	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Updated != 1 || rep.Pushed.Failed != 0 {
		t.Fatalf("push run: %+v", rep.Pushed)
	}
	got := src.ents["R1"]
	if got.Props["title"] != "Login v2" || got.Props["state"] != "open" || got.Content != "Body" {
		t.Fatalf("remote R1 = %+v", got)
	}
	if _, ok := got.Props["note"]; ok {
		t.Fatal("an unmapped local field was pushed")
	}

	v := version(t, tg, r1.ID)
	src.sends = nil
	rep = runSync(t, tg, m, src, Options{})
	if src.writes() != 0 || rep.Entities.Updated != 0 || version(t, tg, r1.ID) != v {
		t.Fatalf("echo run wrote: %+v, sends %v", rep, src.sends)
	}
	src.sends, src.gets = nil, nil
	rep = runSync(t, tg, m, src, Options{})
	if len(src.sends) != 0 || src.fetches() != 0 || rep.Entities != (Counts{}) || version(t, tg, r1.ID) != v {
		t.Fatalf("idle run did work: %+v, sends %v, gets %v", rep, src.sends, src.gets)
	}
}

func TestPush_ClearsAField(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	if _, err := tg.mgr.PatchEntity(ctx, r1.ID, entity.Patch{MetaUnset: []string{"status"}}); err != nil {
		t.Fatal(err)
	}
	if rep := runSync(t, tg, m, src, Options{}); rep.Pushed.Updated != 1 {
		t.Fatalf("push: %+v", rep.Pushed)
	}
	if _, ok := src.ents["R1"].Props["state"]; ok {
		t.Fatalf("state not cleared on the remote: %+v", src.ents["R1"].Props)
	}
}

// TestPush_CreatesAndMapsNewRows pins the create: the remote picks the id,
// the ledger maps it, the local ref names it, and no run creates it twice.
func TestPush_CreatesAndMapsNewRows(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement", Content: "New body",
		Properties: map[string]any{"title": "Export", "status": "open"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Created != 1 {
		t.Fatalf("create: %+v %+v", rep.Pushed, rep.Issues)
	}
	n1 := src.ents["N1"]
	if n1 == nil || n1.Type != "req" || n1.Props["title"] != "Export" || n1.Content != "New body" {
		t.Fatalf("remote row = %+v", n1)
	}
	if got := tg.byRef(t, "N1"); got == nil || got.ID != local.Entity.ID {
		t.Fatalf("the local ref does not name the remote row: %+v", got)
	}

	v := version(t, tg, local.Entity.ID)
	src.sends = nil
	before := tg.count(t, "requirement")
	runSync(t, tg, m, src, Options{})
	runSync(t, tg, m, src, Options{})
	if src.writes() != 0 || tg.count(t, "requirement") != before || version(t, tg, local.Entity.ID) != v ||
		len(src.ents) != 5 {

		t.Fatalf("later runs wrote: sends %v, %d remote rows", src.sends, len(src.ents))
	}
}

// TestPush_LostLedgerDoesNotCreateTwice pins the crash recovery of a
// create: the local ref links the rows again without a second POST.
func TestPush_LostLedgerDoesNotCreateTwice(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	if _, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement",
		Properties: map[string]any{"title": "Export"}}, entity.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	runSync(t, tg, m, src, Options{})
	delete(tg.kv.m, ledgerKey("cat"))
	src.sends = nil
	runSync(t, tg, m, src, Options{})
	for _, s := range src.sends {
		if strings.HasPrefix(s, http.MethodPost) {
			t.Fatalf("created again after the ledger was lost: %v", src.sends)
		}
	}
	if len(src.ents) != 5 {
		t.Fatalf("%d remote rows", len(src.ents))
	}
}

func TestPush_ShareSelectsRows(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) {
		em := p.Entities["req"]
		em.Share = "entity.status == 'open'"
		p.Entities["req"] = em
	})
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	for _, st := range []string{"open", "done"} {
		if _, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement",
			Properties: map[string]any{"title": "T " + st, "status": st}}, entity.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	// A linked row that leaves the share scope is no longer pushed.
	r2 := tg.byRef(t, "R2") // status done
	patch(t, tg, r2.ID, map[string]any{"title": "Changed"})

	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Created != 1 || rep.Pushed.Updated != 0 {
		t.Fatalf("push: %+v", rep.Pushed)
	}
	if src.ents["N1"].Props["title"] != "T open" || src.ents["R2"].Props["title"] != "Logout" {
		t.Fatalf("remote rows: %+v %+v", src.ents["N1"], src.ents["R2"])
	}
}

// TestPush_RetriesOnceAfterPreconditionFailed pins the 412 path: the run
// fetches and merges the remote's change, then sends again; a row that
// keeps changing waits for the next run.
func TestPush_RetriesOnceAfterPreconditionFailed(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Ours"})

	edits := 1
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPatch && edits > 0 {
			edits--
			src.mu.Lock()
			src.ents["R1"].Props = map[string]any{"title": "Login", "state": "done"}
			src.mu.Unlock()
		}
		return 0, ""
	}
	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Updated != 1 || rep.Pushed.Conflicts != 0 {
		t.Fatalf("retry: %+v %+v", rep.Pushed, rep.Issues)
	}
	if got := src.ents["R1"].Props; got["title"] != "Ours" || got["state"] != "done" {
		t.Fatalf("remote R1 = %+v", got)
	}
	if got := tg.byRef(t, "R1").Properties; got["status"] != "done" || got["title"] != "Ours" {
		t.Fatalf("local R1 = %+v", got)
	}

	patch(t, tg, r1.ID, map[string]any{"title": "Ours 2"})
	n := 0
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPatch {
			n++
			src.mu.Lock()
			src.ents["R1"].Content = strings.Repeat("x", n)
			src.mu.Unlock()
		}
		return 0, ""
	}
	rep = runSync(t, tg, m, src, Options{})
	if rep.Pushed.Conflicts != 1 || rep.Pushed.Updated != 0 || n != maxAttempts {
		t.Fatalf("busy row: %+v, %d patches", rep.Pushed, n)
	}
}

// TestPush_RefusalHoldsTheRow pins that a 422 is never overwritten or
// retried as is: the row waits on the conflict list until a local value it
// tried to send changes, or someone resolves it.
func TestPush_RefusalHoldsTheRow(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Bad"})
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPatch {
			return http.StatusUnprocessableEntity, "title is not allowed\x1b[31m"
		}
		return 0, ""
	}
	rep := runSync(t, tg, m, src, Options{})
	if rep.Pushed.Refused != 1 || rep.Held != 1 {
		t.Fatalf("refused: %+v held %d", rep.Pushed, rep.Held)
	}
	list, _ := LoadConflicts(ctx, tg.kv, "cat")
	if len(list) != 1 || list[0].Reason != ReasonRefused || list[0].Field != "" ||
		!strings.Contains(list[0].Detail, "422") || strings.Contains(list[0].Detail, "\x1b") {

		t.Fatalf("conflict list: %+v", list)
	}

	src.sends = nil
	runSync(t, tg, m, src, Options{})
	if src.writes() != 0 {
		t.Fatalf("a held row was pushed: %v", src.sends)
	}

	// A new local value is judged again.
	src.before = nil
	patch(t, tg, r1.ID, map[string]any{"title": "Good"})
	rep = runSync(t, tg, m, src, Options{})
	if rep.Pushed.Updated != 1 || rep.Held != 0 || src.ents["R1"].Props["title"] != "Good" {
		t.Fatalf("after the fix: %+v held %d", rep.Pushed, rep.Held)
	}
}

func TestPush_RefusedCreate(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement",
		Properties: map[string]any{"title": "Secret"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPost {
			return http.StatusBadRequest, "no create"
		}
		return 0, ""
	}
	if rep := runSync(t, tg, m, src, Options{}); rep.Pushed.Refused != 1 {
		t.Fatalf("refused create: %+v", rep.Pushed)
	}
	list, _ := LoadConflicts(ctx, tg.kv, "cat")
	if _, err = CheckResolve(list, local.Entity.ID, "", TakeRemote); err == nil {
		t.Fatal("a refused create resolved to the remote side")
	}
	if _, err = Resolve(ctx, tg.kv, m, local.Entity.ID, "", TakeLocal); err != nil {
		t.Fatal(err)
	}
	src.before = nil
	if rep := runSync(t, tg, m, src, Options{}); rep.Pushed.Created != 1 || rep.Held != 0 {
		t.Fatalf("retried create: %+v held %d", rep.Pushed, rep.Held)
	}
	// Deleting the local row drops a refused create from the list.
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPost {
			return http.StatusForbidden, "no create"
		}
		return 0, ""
	}
	gone, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement",
		Properties: map[string]any{"title": "Gone"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep := runSync(t, tg, m, src, Options{}); rep.Held != 1 {
		t.Fatalf("second refusal: held %d", rep.Held)
	}
	if _, err := tg.mgr.DeleteEntity(ctx, gone.Entity.ID, false); err != nil {
		t.Fatal(err)
	}
	if rep := runSync(t, tg, m, src, Options{}); rep.Held != 0 {
		t.Fatalf("after the delete: held %d", rep.Held)
	}
}

// TestPush_ConflictIsHeldAndResolved pins hold: a field both sides changed
// is written on neither side and not pushed; resolving it moves the merge
// base so the next sync sends the winner.
func TestPush_ConflictIsHeldAndResolved(t *testing.T) {
	for _, take := range []string{TakeLocal, TakeRemote} {
		t.Run(take, func(t *testing.T) {
			ctx := context.Background()
			tg := newTarget(t)
			m := pushMapping(t, tg, nil)
			src := newPushSource(seedSource())
			runSync(t, tg, m, src, Options{})
			r1 := tg.byRef(t, "R1")
			patch(t, tg, r1.ID, map[string]any{"title": "Ours", "status": "done"})
			src.ents["R1"].Props = map[string]any{"title": "Theirs", "state": "open"}

			rep := runSync(t, tg, m, src, Options{})
			if rep.Entities.Conflicts != 1 || rep.Held != 1 || rep.Pushed.Updated != 1 {
				t.Fatalf("conflict run: %+v %+v", rep, rep.Pushed)
			}
			// The other field still flows; the held one stays on each side.
			if src.ents["R1"].Props["title"] != "Theirs" || src.ents["R1"].Props["state"] != "done" {
				t.Fatalf("remote R1 = %+v", src.ents["R1"].Props)
			}
			if tg.byRef(t, "R1").Properties["title"] != "Ours" {
				t.Fatal("the local value was overwritten")
			}

			if _, err := Resolve(ctx, tg.kv, m, r1.ID, "nope", take); !errors.Is(err, ErrNoConflict) {
				t.Fatalf("resolve of an unknown field: %v", err)
			}
			if n, err := Resolve(ctx, tg.kv, m, r1.ID, "title", take); err != nil || n != 1 {
				t.Fatalf("resolve: %d, %v", n, err)
			}
			rep = runSync(t, tg, m, src, Options{})
			want := map[string]string{TakeLocal: "Ours", TakeRemote: "Theirs"}[take]
			if rep.Held != 0 || src.ents["R1"].Props["title"] != want || tg.byRef(t, "R1").Properties["title"] != want {
				t.Fatalf("after resolve: %+v, remote %v, local %v", rep, src.ents["R1"].Props["title"],
					tg.byRef(t, "R1").Properties["title"])
			}
			src.sends = nil
			v := version(t, tg, r1.ID)
			runSync(t, tg, m, src, Options{})
			if src.writes() != 0 || version(t, tg, r1.ID) != v {
				t.Fatalf("the resolve echoed: %v", src.sends)
			}
		})
	}
}

func TestPush_DryRunWritesNothing(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	patch(t, tg, tg.byRef(t, "R1").ID, map[string]any{"title": "Ours"})
	if _, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement",
		Properties: map[string]any{"title": "New"}}, entity.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	src.sends = nil
	rep := runSync(t, tg, m, src, Options{DryRun: true})
	if rep.Pushed.Created != 1 || rep.Pushed.Updated != 1 || src.writes() != 0 {
		t.Fatalf("dry run: %+v, sends %v", rep.Pushed, src.sends)
	}
}

func TestPush_ServerErrorsFailTheRow(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	patch(t, tg, tg.byRef(t, "R1").ID, map[string]any{"title": "Ours"})
	src.before = func(string, string) (int, string) { return http.StatusInternalServerError, "boom" }
	rep, err := Run(context.Background(), m, tg.pushDeps(src), Options{})
	if err == nil || rep.Pushed.Failed != 1 || rep.Held != 0 {
		t.Fatalf("5xx: %+v, %v", rep.Pushed, err)
	}
	src.before = func(string, string) (int, string) { return http.StatusNotFound, "" }
	rep = runSync(t, tg, m, src, Options{})
	if rep.Pushed.Deferred != 1 {
		t.Fatalf("404: %+v", rep.Pushed)
	}
}

func TestPush_RefIsSetOnCreate(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "component",
		Properties: map[string]any{"name": "Cache"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runSync(t, tg, m, src, Options{})
	got, err := tg.st.GetEntity(ctx, local.Entity.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := metamodel.ExternalRefID(got.Properties["cat"]); id != "N1" {
		t.Fatalf("ref = %v", got.Properties["cat"])
	}
}

// TestPush_UncertainCreateIsNotRepeated pins that a create whose outcome is
// unknown is held until a person checks the remote: sending it again could
// make a second copy there.
func TestPush_UncertainCreateIsNotRepeated(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "requirement",
		Properties: map[string]any{"title": "Maybe"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	src.before = func(method, _ string) (int, string) {
		if method == http.MethodPost {
			return http.StatusBadGateway, "upstream timeout"
		}
		return 0, ""
	}
	if rep, err := Run(ctx, m, tg.pushDeps(src), Options{}); err == nil || rep.Held != 1 {
		t.Fatalf("uncertain create: %+v, %v", rep, err)
	}
	list, _ := LoadConflicts(ctx, tg.kv, "cat")
	if len(list) != 1 || list[0].Reason != ReasonUncertain {
		t.Fatalf("conflict list: %+v", list)
	}
	src.before, src.sends = nil, nil
	patch(t, tg, local.Entity.ID, map[string]any{"title": "Maybe 2"})
	if rep := runSync(t, tg, m, src, Options{}); rep.Held != 1 || src.writes() != 0 {
		t.Fatalf("an uncertain create was sent again: %+v, %v", rep, src.sends)
	}
	if _, err := Resolve(ctx, tg.kv, m, local.Entity.ID, "", TakeRemote); err == nil {
		t.Fatal("an uncertain create resolved to the remote side")
	}
	if _, err := Resolve(ctx, tg.kv, m, local.Entity.ID, "", TakeLocal); err != nil {
		t.Fatal(err)
	}
	if rep := runSync(t, tg, m, src, Options{}); rep.Pushed.Created != 1 || rep.Held != 0 {
		t.Fatalf("after resolve: %+v", rep.Pushed)
	}
}

// newRow creates a local row of typ with props and returns its id.
func newRow(t *testing.T, tg *target, typ string, props map[string]any) string {
	t.Helper()
	res, err := tg.mgr.CreateEntity(context.Background(), &entity.Entity{Type: typ, Properties: props},
		entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Entity.ID
}

// heldReasons returns the reason of each conflict list entry by local id.
func heldReasons(t *testing.T, tg *target) map[string]string {
	t.Helper()
	list, err := LoadConflicts(context.Background(), tg.kv, "cat")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range list {
		out[c.Local] = c.Reason
	}
	return out
}

// TestPush_CreateMayHaveReachedTheRemote pins that every create whose
// outcome is unknown is held, so no later run sends it twice: a cancelled
// run, a busy answer, and a success answer the run cannot use. A request
// that never left is not held.
func TestPush_CreateMayHaveReachedTheRemote(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		body    string
		held    bool
		runFail bool
	}{
		{name: "cancelled", err: context.Canceled, held: true, runFail: true},
		{name: "busy", err: &reconclient.RefusedError{Status: http.StatusServiceUnavailable}, held: true, runFail: true},
		{name: "unusable answer", status: http.StatusCreated, body: "not json", held: true, runFail: true},
		{name: "accepted", status: http.StatusAccepted, body: `{"id":"N9","type":"req"}`, held: true, runFail: true},
		{name: "wrong type", status: http.StatusCreated, body: `{"id":"N9","type":"component"}`, held: true, runFail: true},
		{name: "linked id", status: http.StatusCreated, body: `{"id":"R1","type":"req"}`, held: true, runFail: true},
		{name: "never connected", err: &net.OpError{Op: "dial", Err: errors.New("refused")}, runFail: true},
		{name: "credential", err: httpremote.ErrDenied, runFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tg := newTarget(t)
			m := pushMapping(t, tg, nil)
			src := newPushSource(seedSource())
			runSync(t, tg, m, src, Options{})
			local := newRow(t, tg, "requirement", map[string]any{"title": "Maybe"})
			if tc.err != nil {
				src.postErr = map[string]error{"reqs": tc.err}
			} else {
				src.before = func(method, _ string) (int, string) {
					if method == http.MethodPost {
						return tc.status, tc.body
					}
					return 0, ""
				}
			}
			if _, err := Run(ctx, m, tg.pushDeps(src), Options{}); (err != nil) != tc.runFail {
				t.Fatalf("run error = %v", err)
			}
			if got := heldReasons(t, tg)[local] == ReasonUncertain; got != tc.held {
				t.Fatalf("held as uncertain = %v, want %v (%v)", got, tc.held, heldReasons(t, tg))
			}
		})
	}
}

// TestPush_StoppedPushKeepsHolds pins that a push that ends early does not
// drop the holds of rows it did not reach: it cannot tell those rows left
// the share scope.
func TestPush_StoppedPushKeepsHolds(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	req := newRow(t, tg, "requirement", map[string]any{"title": "Maybe"})
	src.before = func(method, path string) (int, string) {
		if method == http.MethodPost && path == "reqs" {
			return http.StatusBadGateway, "upstream"
		}
		return 0, ""
	}
	_, _ = Run(ctx, m, tg.pushDeps(src), Options{})
	if heldReasons(t, tg)[req] != ReasonUncertain {
		t.Fatalf("not held: %v", heldReasons(t, tg))
	}
	// Components push before requirements; a refused credential there ends
	// the push before it reaches the held requirement.
	src.before = nil
	newRow(t, tg, "component", map[string]any{"name": "Cache"})
	src.postErr = map[string]error{"components": httpremote.ErrDenied}
	if _, err := Run(ctx, m, tg.pushDeps(src), Options{}); err == nil {
		t.Fatal("a refused credential did not end the run")
	}
	if heldReasons(t, tg)[req] != ReasonUncertain {
		t.Fatalf("the stopped push dropped the hold: %v", heldReasons(t, tg))
	}
}

// TestPush_PushedRowIsNotDeletedWhenGone pins that on_gone: delete never
// deletes a row this side created and pushed.
func TestPush_PushedRowIsNotDeletedWhenGone(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, func(p *Remote) { p.OnGone = OnGoneDelete })
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local := newRow(t, tg, "component", map[string]any{"name": "Cache"})
	if rep := runSync(t, tg, m, src, Options{}); rep.Pushed.Created != 1 {
		t.Fatalf("create: %+v", rep.Pushed)
	}
	delete(src.ents, "N1")
	runSync(t, tg, m, src, Options{})
	if _, err := tg.st.GetEntity(ctx, entity.Ref{ID: local}); err != nil {
		t.Fatalf("the pushed row was deleted: %v", err)
	}
}

// TestPush_RefIsRepaired pins that a linked row whose ref is missing gets
// it back on the next push, so a lost ledger still links the two rows.
func TestPush_RefIsRepaired(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	local := newRow(t, tg, "component", map[string]any{"name": "Cache"})
	runSync(t, tg, m, src, Options{})
	e, err := tg.st.GetEntity(ctx, entity.Ref{ID: local})
	if err != nil {
		t.Fatal(err)
	}
	delete(e.Properties, "cat")
	if err = tg.st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	runSync(t, tg, m, src, Options{})
	if e, _ = tg.st.GetEntity(ctx, entity.Ref{ID: local}); e.Properties["cat"] == nil {
		t.Fatal("the ref was not set again")
	}
}

// TestPush_AgreedConflictClears pins that a held field whose local value
// becomes the remote's value leaves the list without a write.
func TestPush_AgreedConflictClears(t *testing.T) {
	tg := newTarget(t)
	m := pushMapping(t, tg, nil)
	src := newPushSource(seedSource())
	runSync(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	patch(t, tg, r1.ID, map[string]any{"title": "Ours"})
	src.ents["R1"].Props["title"] = "Theirs"
	if rep := runSync(t, tg, m, src, Options{}); rep.Held != 1 {
		t.Fatalf("no conflict: %+v", rep)
	}
	patch(t, tg, r1.ID, map[string]any{"title": "Theirs"})
	src.sends = nil
	if rep := runSync(t, tg, m, src, Options{}); rep.Held != 0 || src.writes() != 0 {
		t.Fatalf("agreed conflict: held %d, sends %v", rep.Held, src.sends)
	}
}

// TestPushBody_RoutesContentLikeThePull pins that the push sends a field to
// the same remote place the pull reads it from: the local body mapped to a
// remote property travels as that property.
func TestPushBody_RoutesContentLikeThePull(t *testing.T) {
	body := syncmerge.Field{Name: syncmerge.ContentField, Kind: syncmerge.KindContent}
	e := &entity.Entity{Content: "text"}
	for _, tc := range []struct {
		remote, want string
	}{
		{syncmerge.ContentField, `{"content":"text","properties":{}}`},
		{"description", `{"properties":{"description":"text"}}`},
	} {
		raw, _, err := pushBody([]FieldMapping{{Local: body, Remote: tc.remote}}, e, false)
		if err != nil || string(raw) != tc.want {
			t.Errorf("remote %s: %s, %v", tc.remote, raw, err)
		}
	}
}

// redactGate withholds field from every row it reads, as a visible: rule
// does.
type redactGate struct {
	inner ReadGate
	field string
}

func (g redactGate) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	e, err := g.inner.GetAddress(ctx, addr)
	if err != nil {
		return nil, err
	}
	e = e.Clone()
	delete(e.Properties, g.field)
	e.Redacted = []string{g.field}
	return e, nil
}

// TestPush_SendsOnlyWhatThePusherReads pins that the push sends no row and
// no field the principal cannot read, and names no hidden row, as the pull
// merges into none (TKT-A6U657Q).
func TestPush_SendsOnlyWhatThePusherReads(t *testing.T) {
	t.Run("hidden rows", func(t *testing.T) {
		tg := newTarget(t)
		m := pushMapping(t, tg, nil)
		src := newPushSource(seedSource())
		runSync(t, tg, m, src, Options{})
		r1 := tg.byRef(t, "R1")
		patch(t, tg, r1.ID, map[string]any{"title": "Secret edit"})
		hidden := newRow(t, tg, "requirement", map[string]any{"title": "Secret row"})
		tg.gate = hideGate{inner: tg.gate, hidden: map[string]bool{r1.ID: true, hidden: true}}
		src.sends = nil
		rep := runSync(t, tg, m, src, Options{})
		if src.writes() != 0 || src.ents["R1"].Props["title"] == "Secret edit" {
			t.Fatalf("a hidden row was pushed: %v", src.sends)
		}
		noIssueNames(t, rep, r1.ID, hidden)
	})
	t.Run("redacted field", func(t *testing.T) {
		tg := newTarget(t)
		m := pushMapping(t, tg, nil)
		src := newPushSource(seedSource())
		runSync(t, tg, m, src, Options{})
		r1 := tg.byRef(t, "R1")
		patch(t, tg, r1.ID, map[string]any{"title": "Secret title", "status": "done"})
		newRow(t, tg, "requirement", map[string]any{"title": "Secret new", "status": "open"})
		tg.gate = redactGate{inner: tg.gate, field: "title"}
		rep := runSync(t, tg, m, src, Options{})
		if rep.Pushed.Updated != 1 || rep.Pushed.Created != 1 {
			t.Fatalf("push: %+v", rep.Pushed)
		}
		for id, e := range src.ents {
			if title, _ := e.Props["title"].(string); strings.HasPrefix(title, "Secret") {
				t.Fatalf("remote %s got the withheld title %q", id, title)
			}
		}
		if src.ents["R1"].Props["state"] != "done" {
			t.Fatalf("the readable field was not pushed: %+v", src.ents["R1"].Props)
		}
	})
}
