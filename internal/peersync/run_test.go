package peersync

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
)

func seedSource() *fakeSource {
	src := newFakeSource()
	src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "Login", "state": "open"}, Content: "Body"})
	src.put(&fakeEnt{Type: "req", ID: "R2", Props: map[string]any{"title": "Logout", "state": "done"}})
	src.put(&fakeEnt{Type: "component", ID: "C1", Props: map[string]any{"name": "Auth"}})
	src.put(&fakeEnt{Type: "component", ID: "C2", Props: map[string]any{"name": "UI"}})
	src.relate("R1", "satisfies", "C1", map[string]any{"weight": 3})
	src.relate("C2", "depends", "C1", nil)
	return src
}

func pull(t *testing.T, tg *target, m *Mapping, src Source, o Options) Report {
	t.Helper()
	rep, err := Run(context.Background(), m, tg.deps(src), o)
	if err != nil {
		t.Fatalf("Run: %v (report %+v)", err, rep)
	}
	return rep
}

func TestRun_PullsAndIsIdempotent(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()

	rep := pull(t, tg, m, src, Options{})
	if rep.Entities.Created != 4 || rep.Relations.Created != 2 {
		t.Fatalf("first run: %+v", rep)
	}
	r1 := tg.byRef(t, "R1")
	if r1 == nil || r1.ID == "R1" || r1.Type != "requirement" {
		t.Fatalf("R1 not pulled under a local id: %+v", r1)
	}
	if r1.Properties["title"] != "Login" || r1.Properties["status"] != "open" || r1.Content != "Body" {
		t.Fatalf("R1 fields: %+v content %q", r1.Properties, r1.Content)
	}
	c1 := tg.byRef(t, "C1")
	rel, err := tg.st.GetRelation(context.Background(), entity.RelationKey{From: r1.ID, Type: "satisfies", To: c1.ID})
	if err != nil {
		t.Fatalf("relation not pulled: %v", err)
	}
	if w, _ := rel.Properties["weight"].(int64); w != 3 {
		t.Fatalf("weight = %v (%T)", rel.Properties["weight"], rel.Properties["weight"])
	}

	src.gets = nil
	rep = pull(t, tg, m, src, Options{})
	if rep.Entities != (Counts{}) || rep.Relations != (Counts{}) || src.fetches() != 0 {
		t.Fatalf("second run should do nothing: %+v, fetches %v", rep, src.gets)
	}
}

func TestRun_MergesOwnedFieldsAndKeepsLocalOnes(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()
	pull(t, tg, m, src, Options{})
	r1, r2 := tg.byRef(t, "R1"), tg.byRef(t, "R2")

	// A local field the source does not own, and a local edit of an owned
	// field the source leaves alone.
	if _, err := tg.mgr.PatchEntity(ctx, r1.ID, entity.Patch{Properties: map[string]any{"note": "mine"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tg.mgr.PatchEntity(ctx, r2.ID, entity.Patch{Properties: map[string]any{"title": "Sign out"}}); err != nil {
		t.Fatal(err)
	}
	src.ents["R1"].Props = map[string]any{"title": "Log in", "state": "open"}
	src.ents["R2"].Props = map[string]any{"title": "Logout", "state": "open"}

	rep := pull(t, tg, m, src, Options{})
	if rep.Entities.Updated != 2 || rep.Entities.Conflicts != 0 {
		t.Fatalf("report: %+v", rep)
	}
	r1, r2 = tg.byRef(t, "R1"), tg.byRef(t, "R2")
	if r1.Properties["title"] != "Log in" || r1.Properties["note"] != "mine" {
		t.Fatalf("R1: %+v", r1.Properties)
	}
	if r2.Properties["title"] != "Sign out" || r2.Properties["status"] != "open" {
		t.Fatalf("R2 should keep its local title and take the status: %+v", r2.Properties)
	}
}

func TestRun_ConflictLeavesRowAndAppliesOthers(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()
	pull(t, tg, m, src, Options{})
	r1 := tg.byRef(t, "R1")
	if _, err := tg.mgr.PatchEntity(ctx, r1.ID, entity.Patch{Properties: map[string]any{"title": "Ours"}}); err != nil {
		t.Fatal(err)
	}
	src.ents["R1"].Props = map[string]any{"title": "Theirs", "state": "open"}
	src.ents["C1"].Props = map[string]any{"name": "Authn"}

	for range 2 { // the conflict stays until someone resolves it
		rep := pull(t, tg, m, src, Options{})
		if rep.Entities.Conflicts != 1 || len(rep.Issues) != 1 || rep.Issues[0].Kind != IssueConflict ||
			rep.Issues[0].Fields[0] != "title" {

			t.Fatalf("report: %+v", rep)
		}
	}
	if got := tg.byRef(t, "R1").Properties["title"]; got != "Ours" {
		t.Fatalf("R1 title = %v", got)
	}
	if got := tg.byRef(t, "C1").Properties["name"]; got != "Authn" {
		t.Fatalf("C1 name = %v", got)
	}
}

func TestRun_GoneGuard(t *testing.T) {
	tg := newTarget(t)
	one := 1
	m := testMapping(t, tg.meta, func(p *Peer) { p.MaxGone = &one })
	src := seedSource()
	pull(t, tg, m, src, Options{})
	before := tg.kv.m[ledgerKey("cat")]

	delete(src.ents, "R1")
	delete(src.ents, "R2")
	delete(src.rels, "R1--satisfies--C1")
	rep, err := Run(context.Background(), m, tg.deps(src), Options{})
	if !errors.Is(err, ErrGoneGuard) || rep.Gone != 3 || !strings.Contains(rep.Refused, "--allow-gone 3") {
		t.Fatalf("err %v, report %+v", err, rep)
	}
	if !bytes.Equal(tg.kv.m[ledgerKey("cat")], before) {
		t.Fatal("a refused run changed the ledger")
	}

	rep = pull(t, tg, m, src, Options{AllowGone: 3})
	if rep.Entities.Detached != 2 || rep.Relations.Detached != 1 {
		t.Fatalf("report: %+v", rep)
	}
	if tg.count(t, "requirement") != 2 {
		t.Fatal("detach must keep the local rows")
	}
	// Detached rows are out of the diff: the next run sees nothing gone.
	rep = pull(t, tg, m, src, Options{})
	if rep.Gone != 0 {
		t.Fatalf("report: %+v", rep)
	}
	// The source listing a detached row again links it again.
	src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "Login", "state": "open"}, Content: "Body"})
	rep = pull(t, tg, m, src, Options{})
	if rep.Entities.Adopted != 1 || tg.count(t, "requirement") != 2 {
		t.Fatalf("relink: %+v", rep)
	}
}

func TestRun_EmptiedTypeIsRefused(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := newFakeSource()
	src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "A"}})
	src.put(&fakeEnt{Type: "component", ID: "C1", Props: map[string]any{"name": "B"}})
	pull(t, tg, m, src, Options{})

	delete(src.ents, "C1")
	rep, err := Run(context.Background(), m, tg.deps(src), Options{})
	if !errors.Is(err, ErrGoneGuard) || !strings.Contains(rep.Refused, `entity type "component"`) {
		t.Fatalf("err %v, report %+v", err, rep)
	}
	pull(t, tg, m, src, Options{AllowGone: 1})
}

func TestRun_OnGoneDelete(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, func(p *Peer) { p.OnGone = OnGoneDelete })
	src := seedSource()
	src.put(&fakeEnt{Type: "req", ID: "R3", Props: map[string]any{"title": "Edited"}})
	pull(t, tg, m, src, Options{})
	r3 := tg.byRef(t, "R3")
	if _, err := tg.mgr.PatchEntity(ctx, r3.ID, entity.Patch{Properties: map[string]any{"title": "Mine"}}); err != nil {
		t.Fatal(err)
	}
	// R1 loses its relation at the source too, R2 has a local relation.
	r2, c2 := tg.byRef(t, "R2"), tg.byRef(t, "C2")
	if _, err := tg.mgr.CreateRelation(ctx, entity.RelationKey{From: r2.ID, Type: "satisfies", To: c2.ID},
		entity.RelationOptions{}); err != nil {
		t.Fatal(err)
	}
	delete(src.ents, "R1")
	delete(src.ents, "R2")
	delete(src.ents, "R3")
	delete(src.rels, "R1--satisfies--C1")

	rep := pull(t, tg, m, src, Options{AllowGone: 4})
	if rep.Entities.Deleted != 1 || rep.Entities.Detached != 2 || rep.Relations.Deleted != 1 {
		t.Fatalf("report: %+v", rep)
	}
	if tg.byRef(t, "R1") != nil || tg.byRef(t, "R2") == nil || tg.byRef(t, "R3") == nil {
		t.Fatal("only the unedited row without relations is deleted")
	}
	var kept int
	for _, is := range rep.Issues {
		if is.Kind == IssueKept {
			kept++
		}
	}
	if kept != 2 {
		t.Fatalf("issues: %+v", rep.Issues)
	}
}

func TestRun_DeniedFailsWithoutWriting(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()
	pull(t, tg, m, src, Options{})
	before := string(tg.kv.m[ledgerKey("cat")])

	src.deny = true
	_, err := Run(context.Background(), m, tg.deps(src), Options{})
	if !errors.Is(err, httpremote.ErrDenied) {
		t.Fatalf("err = %v", err)
	}
	if string(tg.kv.m[ledgerKey("cat")]) != before || tg.count(t, "requirement") != 2 {
		t.Fatal("a denied run changed something")
	}
}

func TestRun_HostileAnswers(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := newFakeSource()
	src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "A", "state": "bogus"}})
	src.put(&fakeEnt{Type: "req", ID: "R2", Props: map[string]any{"title": "B"}})
	src.put(&fakeEnt{Type: "req", ID: "R3", Props: map[string]any{"title": "C"}})
	src.put(&fakeEnt{Type: "req", ID: strings.Repeat("x", 300), Props: map[string]any{"title": "D"}})
	src.answer = func(path string, body []byte) []byte {
		if path == "reqs/R2" {
			return []byte(`{"id":"R9","type":"req","properties":{}}`)
		}
		return body
	}
	rep := pull(t, tg, m, src, Options{})
	if rep.Entities.Created != 1 || rep.Entities.Rejected != 3 {
		t.Fatalf("report: %+v", rep)
	}
	for _, e := range []string{"R1", "R2"} {
		if tg.byRef(t, e) != nil {
			t.Fatalf("%s should be rejected", e)
		}
	}
	// A rejected row is not retried until it changes.
	src.gets = nil
	rep = pull(t, tg, m, src, Options{})
	if src.fetches() != 0 || rep.Entities.Rejected != 0 {
		t.Fatalf("rejected rows refetched: %v, %+v", src.gets, rep)
	}
}

func TestRun_RedactedIsNotClearedAbsentIs(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := newFakeSource()
	src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "A", "state": "open"}})
	pull(t, tg, m, src, Options{})

	src.ents["R1"] = &fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"state": "done"}, Redacted: []string{"title"}}
	pull(t, tg, m, src, Options{})
	if got := tg.byRef(t, "R1").Properties; got["title"] != "A" || got["status"] != "done" {
		t.Fatalf("redacted title cleared: %+v", got)
	}
	src.ents["R1"] = &fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "A"}}
	pull(t, tg, m, src, Options{})
	if got := tg.byRef(t, "R1").Properties; got["status"] != nil {
		t.Fatalf("absent status not cleared: %+v", got)
	}
	// Without _redacted, absent means not reported.
	src.ents["R1"] = &fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"state": "open"}, noRedacted: true}
	pull(t, tg, m, src, Options{})
	if got := tg.byRef(t, "R1").Properties; got["title"] != "A" {
		t.Fatalf("title cleared without _redacted: %+v", got)
	}
}

func TestRun_LocalDeleteIsNotRecreated(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()
	pull(t, tg, m, src, Options{})
	c2 := tg.byRef(t, "C2")
	if _, err := tg.mgr.DeleteEntity(ctx, c2.ID, true); err != nil {
		t.Fatal(err)
	}
	src.ents["C2"].Props = map[string]any{"name": "UI 2"}
	src.rels["C2--depends--C1"].Meta = map[string]any{"x": 1}
	rep := pull(t, tg, m, src, Options{})
	if rep.Entities.Dropped != 1 || rep.Relations.Dropped != 1 || tg.byRef(t, "C2") != nil {
		t.Fatalf("report: %+v", rep)
	}
}

func TestRun_DeferredRows(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, func(p *Peer) { p.MaxFetch = 2 })
	src := seedSource()
	rep := pull(t, tg, m, src, Options{})
	if rep.Entities.Created != 2 || rep.Entities.Deferred != 2 || rep.Relations.Deferred != 1 {
		t.Fatalf("first run: %+v", rep)
	}
	for range 3 {
		pull(t, tg, m, src, Options{})
	}
	if tg.count(t, "requirement") != 2 || tg.count(t, "component") != 2 {
		t.Fatal("the deferred rows did not arrive")
	}
}

func TestRun_DryRunWritesNothing(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	src := seedSource()
	rep := pull(t, tg, m, src, Options{DryRun: true})
	if !rep.DryRun || rep.Entities.Created != 4 {
		t.Fatalf("report: %+v", rep)
	}
	if tg.count(t, "requirement") != 0 || len(tg.kv.m) != 0 {
		t.Fatal("a dry run wrote")
	}
}

func TestRun_AdoptsAnEntityHoldingTheRef(t *testing.T) {
	ctx := context.Background()
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	res, err := tg.mgr.CreateEntity(ctx, &entity.Entity{Type: "component", Properties: map[string]any{
		"name": "Auth", "cat": map[string]any{"id": "C1"},
	}}, entity.CreateOptions{WriteExternalRefs: true})
	if err != nil {
		t.Fatal(err)
	}
	src := newFakeSource()
	src.put(&fakeEnt{Type: "component", ID: "C1", Props: map[string]any{"name": "Auth"}})
	src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "A"}})
	rep := pull(t, tg, m, src, Options{})
	if rep.Entities.Adopted != 1 || tg.byRef(t, "C1").ID != res.Entity.ID {
		t.Fatalf("report: %+v", rep)
	}
}

func TestRun_RejectsBadDeps(t *testing.T) {
	tg := newTarget(t)
	m := testMapping(t, tg.meta, nil)
	d := tg.deps(seedSource())
	d.Writer = nil
	if _, err := Run(context.Background(), m, d, Options{}); err == nil {
		t.Fatal("want an error for a missing writer")
	}
	if _, err := Run(context.Background(), nil, tg.deps(seedSource()), Options{}); err == nil {
		t.Fatal("want an error for a missing mapping")
	}
}
