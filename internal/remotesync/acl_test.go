package remotesync

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// puller is the local person who runs the pull. The pull writes as them.
const puller = "alice"

// remotePolicy grants the puller the mapped types when granted is
// true, and only reads otherwise.
func remotePolicy(granted bool) string {
	role := `    read: ["*"]
`
	if granted {
		role += `    create: [requirement, component]
    update: [requirement, component]
    delete: [requirement, component]
`
	}
	return "roles:\n  mirror:\n" + role + "assignments:\n  " + puller + ": mirror\n"
}

// aclTarget is newTarget with the manager on a real acl.yaml policy and a
// memory audit sink, and ctx bound to the puller as the CLI binds it.
func aclTarget(t *testing.T, policy string) (*target, *audit.Memory, context.Context) {
	t.Helper()
	tg := newTarget(t)
	p, err := acl.LoadPolicyBytes([]byte(policy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(tg.st), tg.st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	copyVis, err := entitymanager.NewAllowAllCopyVisibility(tg.st)
	if err != nil {
		t.Fatalf("NewAllowAllCopyVisibility: %v", err)
	}
	sink := audit.NewMemory()
	tg.mgr, err = entitymanager.New(entitymanager.Deps{
		Store: tg.st, Meta: tg.meta, Templater: nopTemplater{}, Audit: sink, ACL: d,
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: copyVis,
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	gate, err := visibility.NewDeclarativeGate(d, store.TrivialScope())
	if err != nil {
		t.Fatalf("NewDeclarativeGate: %v", err)
	}
	policyReader, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, tg.st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	sr, err := visibility.NewScriptReader(policyReader, tg.st, gate)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}
	tg.gate = sr.WithWorld(visibility.WorldOf(store.TrivialScope()))
	ctx := principal.With(context.Background(), principal.Principal{User: puller, Tool: principal.ToolCLI})
	ctx, err = acl.BindRequest(ctx, d)
	if err != nil {
		t.Fatalf("BindRequest: %v", err)
	}
	return tg, sink, ctx
}

func TestRun_LocalACLGatesThePuller(t *testing.T) {
	t.Run("no grant writes nothing and audits the refusal", func(t *testing.T) {
		tg, sink, ctx := aclTarget(t, remotePolicy(false))
		rep, _ := Run(ctx, testMapping(t, tg.meta, nil), tg.deps(seedSource()), Options{})
		if rep.Entities.Created != 0 || rep.Relations.Created != 0 {
			t.Fatalf("created rows without a grant: %+v", rep)
		}
		if n := tg.count(t, "requirement") + tg.count(t, "component"); n != 0 {
			t.Fatalf("%d entities written without a grant", n)
		}
		recs := sink.Records()
		if len(recs) == 0 {
			t.Fatal("refused writes left no audit trail")
		}
		for _, r := range recs {
			if r.Op != audit.OpDeniedWrite || r.Principal.User != puller {
				t.Fatalf("want only denied writes by the puller, got %+v", r)
			}
		}
	})
	t.Run("grant writes as the puller", func(t *testing.T) {
		tg, sink, ctx := aclTarget(t, remotePolicy(true))
		rep, err := Run(ctx, testMapping(t, tg.meta, nil), tg.deps(seedSource()), Options{})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if rep.Entities.Created != 4 || rep.Relations.Created != 2 {
			t.Fatalf("report: %+v", rep)
		}
		recs := sink.Records()
		if len(recs) == 0 {
			t.Fatal("no audit records")
		}
		for _, r := range recs {
			if r.Op == audit.OpDeniedWrite || r.Principal.User != puller ||
				r.Principal.Tool != principal.ToolCLI {

				t.Fatalf("audit record not attributed to the puller: %+v", r)
			}
		}
	})
}

// TestRun_ConflictHidesUnreadableLocalRow pins that a conflict on a local
// row the puller cannot read does not name it. The row holds the ref of a
// source requirement but is a component, which the puller may not read in
// the hidden case.
func TestRun_ConflictHidesUnreadableLocalRow(t *testing.T) {
	const hiddenID = "CMP-SECRET1"
	cases := []struct {
		name      string
		read      string
		wantLocal string
	}{
		{"readable row is named", `["*"]`, hiddenID},
		{"hidden row is not named", `[requirement]`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := "roles:\n  mirror:\n    read: " + tc.read + "\n" +
				"    create: [requirement]\n    update: [requirement]\n" +
				"assignments:\n  " + puller + ": mirror\n"
			tg, _, ctx := aclTarget(t, policy)
			if err := tg.st.CreateEntity(context.Background(), &entity.Entity{
				ID: hiddenID, Type: "component", Properties: map[string]any{
					"name": "Secret", "cat": map[string]any{"id": "R1"},
				},
			}); err != nil {
				t.Fatal(err)
			}
			src := newFakeSource()
			src.put(&fakeEnt{Type: "req", ID: "R1", Props: map[string]any{"title": "Login", "state": "open"}})

			rep, err := Run(ctx, testMapping(t, tg.meta, nil), tg.deps(src), Options{})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if rep.Entities.Conflicts != 1 || rep.Entities.Created != 0 || len(rep.Issues) != 1 {
				t.Fatalf("report: %+v", rep)
			}
			is := rep.Issues[0]
			if is.Kind != IssueConflict || is.Local != tc.wantLocal {
				t.Fatalf("issue = %+v, want a conflict with local %q", is, tc.wantLocal)
			}
			if tc.wantLocal == "" {
				for _, s := range append([]string{is.Reason, is.Address}, is.Fields...) {
					if strings.Contains(s, hiddenID) || strings.Contains(s, "component") {
						t.Fatalf("the issue identifies the hidden row: %+v", is)
					}
				}
			}
		})
	}
}

// hideGate reads as inner does, except that the local ids in hidden are
// rows the puller cannot read.
type hideGate struct {
	inner  ReadGate
	hidden map[string]bool
}

func (g hideGate) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	id, _, _ := strings.Cut(addr, entity.StateRefSeparator)
	if g.hidden[id] {
		return nil, store.ErrNotFound
	}
	return g.inner.GetAddress(ctx, addr)
}

// noIssueNames fails when an issue names one of ids, in its local key or
// its reason.
func noIssueNames(t *testing.T, rep Report, ids ...string) {
	t.Helper()
	for _, is := range rep.Issues {
		for _, id := range ids {
			if strings.Contains(is.Local, id) || strings.Contains(is.Reason, id) {
				t.Fatalf("issue names hidden row %s: %+v", id, is)
			}
		}
	}
}

// TestRun_HiddenLocalRowsAreNotNamed pins that a row linked while the
// puller could read it, and hidden since, is not named on any path: an
// update, a relation that has it as an endpoint, and a gone row.
func TestRun_HiddenLocalRowsAreNotNamed(t *testing.T) {
	setup := func(t *testing.T, edit func(p *Remote)) (*target, *Mapping, *fakeSource, hideGate) {
		t.Helper()
		tg := newTarget(t)
		m := testMapping(t, tg.meta, edit)
		src := seedSource()
		pull(t, tg, m, src, Options{})
		g := hideGate{inner: tg.gate, hidden: map[string]bool{}}
		tg.gate = g
		return tg, m, src, g
	}
	t.Run("update", func(t *testing.T) {
		tg, m, src, g := setup(t, nil)
		r1 := tg.byRef(t, "R1")
		g.hidden[r1.ID] = true
		src.ents["R1"].Props = map[string]any{"title": "Changed", "state": "open"}
		rep := pull(t, tg, m, src, Options{})
		if rep.Entities.Conflicts != 1 || rep.Entities.Updated != 0 {
			t.Fatalf("report: %+v", rep)
		}
		noIssueNames(t, rep, r1.ID)
		if got := tg.byRef(t, "R1").Properties["title"]; got != "Login" {
			t.Fatalf("the hidden row was written: title %v", got)
		}
	})
	t.Run("relation endpoint", func(t *testing.T) {
		tg, m, src, g := setup(t, nil)
		c1 := tg.byRef(t, "C1")
		g.hidden[c1.ID] = true
		src.rels["R1--satisfies--C1"].Meta = map[string]any{"weight": 7}
		rep := pull(t, tg, m, src, Options{})
		if rep.Relations.Conflicts != 1 || rep.Relations.Updated != 0 {
			t.Fatalf("report: %+v", rep)
		}
		noIssueNames(t, rep, c1.ID)
	})
	t.Run("gone entity", func(t *testing.T) {
		tg, m, src, g := setup(t, func(p *Remote) { p.OnGone = OnGoneDelete })
		r2 := tg.byRef(t, "R2")
		g.hidden[r2.ID] = true
		delete(src.ents, "R2")
		rep := pull(t, tg, m, src, Options{})
		if rep.Entities.Detached != 1 || rep.Entities.Deleted != 0 {
			t.Fatalf("report: %+v", rep)
		}
		noIssueNames(t, rep, r2.ID)
		if tg.byRef(t, "R2") == nil {
			t.Fatal("the hidden row was deleted")
		}
	})
	t.Run("gone relation", func(t *testing.T) {
		tg, m, src, g := setup(t, func(p *Remote) { p.OnGone = OnGoneDelete })
		c2 := tg.byRef(t, "C2")
		g.hidden[c2.ID] = true
		delete(src.rels, "C2--depends--C1")
		rep := pull(t, tg, m, src, Options{AllowGone: 1})
		if rep.Relations.Detached != 1 || rep.Relations.Deleted != 0 {
			t.Fatalf("report: %+v", rep)
		}
		noIssueNames(t, rep, c2.ID)
	})
}

func TestDeps_RequiresGate(t *testing.T) {
	tg := newTarget(t)
	d := tg.deps(seedSource())
	d.Gate = nil
	if _, err := Run(context.Background(), testMapping(t, tg.meta, nil), d, Options{}); err == nil ||
		!strings.Contains(err.Error(), "read gate") {

		t.Fatalf("Run without a gate: %v", err)
	}
}

// TestPush_LocalACLGatesThePusher pins on a real acl.yaml that the push
// sends only rows the person it runs as may read: a new row of a type the
// pusher cannot read is not created on the remote and not named, while a
// readable new row is.
func TestPush_LocalACLGatesThePusher(t *testing.T) {
	policy := "roles:\n  mirror:\n    read: [component]\n    create: [requirement, component]\n" +
		"    update: [component]\nassignments:\n  " + puller + ": mirror\n"
	tg, _, ctx := aclTarget(t, policy)
	m := testMapping(t, tg.meta, func(p *Remote) { p.Push = true })
	src := newPushSource(seedSource())
	if _, err := Run(ctx, m, tg.pushDeps(src), Options{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	hidden := &entity.Entity{ID: "REQ-HIDDEN", Type: "requirement", Properties: map[string]any{"title": "Hidden"}}
	if err := tg.st.CreateEntity(ctx, hidden); err != nil {
		t.Fatal(err)
	}
	shown := &entity.Entity{ID: "CMP-SHOWN", Type: "component", Properties: map[string]any{"name": "Shown"}}
	if err := tg.st.CreateEntity(ctx, shown); err != nil {
		t.Fatal(err)
	}
	src.sends = nil
	rep, err := Run(ctx, m, tg.pushDeps(src), Options{})
	if err != nil {
		t.Fatalf("push run: %v (%+v)", err, rep.Issues)
	}
	if rep.Pushed.Created != 1 || src.writes() != 1 {
		t.Fatalf("push: %+v, sends %v", rep.Pushed, src.sends)
	}
	for _, e := range src.ents {
		if e.Props["title"] == "Hidden" {
			t.Fatal("a row the pusher cannot read reached the remote")
		}
	}
	noIssueNames(t, rep, hidden.ID)
}
