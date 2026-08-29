package dataentry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// newGanttTestApp builds an app around a self-referential containment schema:
// project contains project (unbounded depth), project has-epic epic. The
// gantt "plan" maps each type's date roles under different property names, as
// real schemas do.
func newGanttTestApp(t *testing.T, mutate ...func(*dataentryconfig.Gantt)) *App {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"project": {
				Label:           "Project",
				IDPrefix:        "PRJ-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":         {Type: "string", Required: true},
					"planned_start": {Type: "date"},
					"planned_end":   {Type: "date"},
					"target_date":   {Type: "date"},
				},
				PropertyOrder: []string{"title", "planned_start", "planned_end", "target_date"},
			},
			"epic": {
				Label:           "Epic",
				IDPrefix:        "EPIC-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
					"start": {Type: "date"},
					"end":   {Type: "date"},
				},
				PropertyOrder: []string{"title", "start", "end"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"contains": {Label: "contains", From: []string{"project"}, To: []string{"project"}},
			"has-epic": {Label: "has epic", From: []string{"project"}, To: []string{"epic"}},
		},
	}

	g := dataentryconfig.Gantt{
		Title:     "Plan",
		Hierarchy: []string{"contains", "has-epic"},
		Sources: map[string]dataentryconfig.GanttSource{
			"project": {Start: "planned_start", End: "planned_end", Committed: "target_date"},
			"epic":    {Start: "start", End: "end"},
		},
	}
	for _, m := range mutate {
		m(&g)
	}

	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Gantt Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Gantts:     map[string]dataentryconfig.Gantt{"plan": g},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	dataentryconfig.NormalizeGantts(cfg)

	return newAppFromParts(cfg, meta, newFixture())
}

// ganttGet performs GET /api/v1/_gantts/{id}[?query] with the given ctx.
func ganttGet(ctx context.Context, app *App, idAndQuery string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_gantts/"+idAndQuery, http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.gantt.handleV1Gantt(rec, req)
	return rec
}

// decodeGantt decodes a 200 response body.
func decodeGantt(t *testing.T, rec *httptest.ResponseRecorder) v1.GanttResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var resp v1.GanttResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return resp
}

func seedProject(app *App, id, title string, props map[string]any) {
	p := map[string]any{"title": title}
	maps.Copy(p, props)
	seedEntity(app, &entity.Entity{ID: id, Type: "project", Properties: p})
}

func seedEpic(app *App, id, title, start, end string) {
	seedEntity(app, &entity.Entity{ID: id, Type: "epic",
		Properties: map[string]any{"title": title, "start": start, "end": end}})
}

func TestGantt_UnknownConfigID404(t *testing.T) {
	app := newGanttTestApp(t)
	rec := ganttGet(context.Background(), app, "ghost")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
	// A config key, not an entity id — the named 404 is fine here.
	if !strings.Contains(rec.Body.String(), "Gantt not found") {
		t.Errorf("body should name the missing gantt config: %s", rec.Body)
	}
}

// TestGantt_RecursiveTreeShape pins AC2: self-referential containment renders
// to arbitrary depth with correct parent/child links, mixed relation types
// traversed as one set.
func TestGantt_RecursiveTreeShape(t *testing.T) {
	app := newGanttTestApp(t)
	seedProject(app, "PRJ-A", "Root", nil)
	seedProject(app, "PRJ-B", "Mid", nil)
	seedProject(app, "PRJ-C", "Deep", nil)
	seedEpic(app, "EPIC-D", "Leaf", "2026-03-01", "2026-04-01")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "contains", To: "PRJ-C"})
	seedRelation(app, &entity.Relation{From: "PRJ-C", Type: "has-epic", To: "EPIC-D"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if len(resp.Roots) != 1 {
		t.Fatalf("roots = %d, want 1: %+v", len(resp.Roots), resp.Roots)
	}
	path := []string{}
	for n := &resp.Roots[0]; n != nil; {
		path = append(path, n.ID)
		if len(n.Children) == 0 {
			break
		}
		n = &n.Children[0]
	}
	want := []string{"PRJ-A", "PRJ-B", "PRJ-C", "EPIC-D"}
	if strings.Join(path, ",") != strings.Join(want, ",") {
		t.Errorf("chain = %v, want %v", path, want)
	}
	if resp.Truncated {
		t.Errorf("truncated should be false")
	}
}

// TestGantt_RollUpEnvelope pins AC3: a parent with no dates of its own gets
// exactly the min/max envelope of its transitive descendants.
func TestGantt_RollUpEnvelope(t *testing.T) {
	app := newGanttTestApp(t)
	seedProject(app, "PRJ-A", "Root", nil) // no dates
	seedProject(app, "PRJ-B", "Mid", nil)  // no dates
	seedEpic(app, "EPIC-1", "Early", "2026-01-10", "2026-02-01")
	seedEpic(app, "EPIC-2", "Late", "2026-03-01", "2026-05-15")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "has-epic", To: "EPIC-1"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "has-epic", To: "EPIC-2"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	root := resp.Roots[0]
	if root.Planned != nil {
		t.Errorf("root declares no dates; planned should be absent, got %+v", root.Planned)
	}
	if root.Rolled == nil || root.Rolled.Start != "2026-01-10" || root.Rolled.End != "2026-05-15" {
		t.Errorf("rolled = %+v, want 2026-01-10 .. 2026-05-15", root.Rolled)
	}
	if root.Breach.Before || root.Breach.After {
		t.Errorf("no planned window, so no breach possible: %+v", root.Breach)
	}
}

// TestGantt_Breach pins AC4: both directions, independently.
func TestGantt_Breach(t *testing.T) {
	cases := []struct {
		name                  string
		childStart, childEnd  string
		wantBefore, wantAfter bool
	}{
		{"inside window", "2026-02-01", "2026-05-01", false, false},
		{"starts early", "2026-01-01", "2026-05-01", true, false},
		{"overruns", "2026-02-01", "2026-07-01", false, true},
		{"both", "2026-01-01", "2026-07-01", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newGanttTestApp(t)
			seedProject(app, "PRJ-A", "Root", map[string]any{
				"planned_start": "2026-01-15", "planned_end": "2026-06-15"})
			seedEpic(app, "EPIC-1", "Child", tc.childStart, tc.childEnd)
			seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-1"})

			resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
			root := resp.Roots[0]
			if root.Breach.Before != tc.wantBefore || root.Breach.After != tc.wantAfter {
				t.Errorf("breach = %+v, want before=%v after=%v (rolled=%+v planned=%+v)",
					root.Breach, tc.wantBefore, tc.wantAfter, root.Rolled, root.Planned)
			}
		})
	}
}

// TestGantt_MultiParentFirst pins AC5 (first): a two-parent node renders
// exactly once, under the deterministically first parent.
func TestGantt_MultiParentFirst(t *testing.T) {
	app := newGanttTestApp(t)
	seedProject(app, "PRJ-A", "P1", nil)
	seedProject(app, "PRJ-B", "P2", nil)
	seedEpic(app, "EPIC-S", "Shared", "2026-01-01", "2026-02-01")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-S"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "has-epic", To: "EPIC-S"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	count := 0
	var walk func(ns []v1.GanttNode)
	walk = func(ns []v1.GanttNode) {
		for _, n := range ns {
			if n.ID == "EPIC-S" {
				count++
			}
			walk(n.Children)
		}
	}
	walk(resp.Roots)
	if count != 1 {
		t.Errorf("shared epic rendered %d times, want exactly 1", count)
	}
	// Deterministic: PRJ-A sorts before PRJ-B, so it owns the child.
	for _, n := range resp.Roots {
		if n.ID == "PRJ-A" && (len(n.Children) != 1 || n.Children[0].ID != "EPIC-S") {
			t.Errorf("PRJ-A should own the shared epic: %+v", n.Children)
		}
	}
}

// TestGantt_MultiParentError pins AC5 (error): the request refuses, naming
// the (visible) offender.
func TestGantt_MultiParentError(t *testing.T) {
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) { g.MultiParent = "error" })
	seedProject(app, "PRJ-A", "P1", nil)
	seedProject(app, "PRJ-B", "P2", nil)
	seedEpic(app, "EPIC-S", "Shared", "2026-01-01", "2026-02-01")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-S"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "has-epic", To: "EPIC-S"})

	rec := ganttGet(context.Background(), app, "plan")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "EPIC-S") {
		t.Errorf("error should name the multi-parent entity: %s", rec.Body)
	}
}

// TestGantt_Cycle pins AC6: a containment loop neither hangs nor renders.
// A ⊃ B ⊃ A gives every loop member a parent, so no root reaches it — the
// error policy refuses the request, prune drops the loop silently.
func TestGantt_Cycle(t *testing.T) {
	seed := func(app *App) {
		seedProject(app, "PRJ-OK", "Standalone", map[string]any{
			"planned_start": "2026-01-01", "planned_end": "2026-02-01"})
		seedProject(app, "PRJ-A", "LoopA", nil)
		seedProject(app, "PRJ-B", "LoopB", nil)
		seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
		seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "contains", To: "PRJ-A"})
	}

	t.Run("error policy refuses", func(t *testing.T) {
		app := newGanttTestApp(t)
		seed(app)
		rec := ganttGet(context.Background(), app, "plan")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422; body=%s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "containment loop") {
			t.Errorf("error should describe the cycle: %s", rec.Body)
		}
	})

	t.Run("prune policy drops the loop", func(t *testing.T) {
		app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) { g.OnCycle = "prune" })
		seed(app)
		resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
		if len(resp.Roots) != 1 || resp.Roots[0].ID != "PRJ-OK" {
			t.Errorf("only the standalone project should render: %+v", resp.Roots)
		}
	})

	t.Run("self-loop is ignored as a tree edge", func(t *testing.T) {
		app := newGanttTestApp(t)
		seedProject(app, "PRJ-SELF", "Selfie", nil)
		seedRelation(app, &entity.Relation{From: "PRJ-SELF", Type: "contains", To: "PRJ-SELF"})
		resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
		if len(resp.Roots) != 1 || resp.Roots[0].ID != "PRJ-SELF" {
			t.Errorf("self-loop entity should render as a root: %+v", resp.Roots)
		}
	})
}

// TestGantt_ACLRollupExcludesHiddenChild pins AC7(a), the RR-5KEF8E
// differential: a hidden child that is the max-end descendant must NOT
// stretch the visible parent's rolled span. Same tree, two principals,
// different arithmetic.
func TestGantt_ACLRollupExcludesHiddenChild(t *testing.T) {
	app := newGanttTestApp(t)
	seedProject(app, "PRJ-A", "Root", nil)
	seedProject(app, "PRJ-B", "VisibleChild", map[string]any{
		"planned_start": "2026-01-10", "planned_end": "2026-03-01"})
	seedEpic(app, "EPIC-H", "HiddenLate", "2026-02-01", "2026-09-30") // the max end
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-H"})

	// Privileged view (no gate → permit-all): epic stretches the roll-up.
	priv := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if priv.Roots[0].Rolled == nil || priv.Roots[0].Rolled.End != "2026-09-30" {
		t.Fatalf("privileged rolled = %+v, want end 2026-09-30", priv.Roots[0].Rolled)
	}

	// Alice may read projects but not epics.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"project"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	resp := decodeGantt(t, ganttGet(gateCtxFor(aliceCtx(), t, d), app, "plan"))
	root := resp.Roots[0]
	if root.Rolled == nil || root.Rolled.End != "2026-03-01" {
		t.Errorf("LEAK: restricted rolled = %+v, want end 2026-03-01 (visible child only)", root.Rolled)
	}
	if strings.Contains(ganttBody(t, resp), "EPIC-H") {
		t.Errorf("hidden entity id appears in the response")
	}
}

// ganttBody re-serializes a decoded response for leak grepping.
func ganttBody(t *testing.T, r v1.GanttResponse) string {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestGantt_FieldHiddenDateExcludedFromFold pins AC7(b), the RR-7PK0YW
// laundering case: a ROW-visible entity whose date field is `visible:`-hidden
// contributes nothing to any ancestor's span — the redaction runs before the
// fold, not after it.
func TestGantt_FieldHiddenDateExcludedFromFold(t *testing.T) {
	app := newGanttTestApp(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"end": false}, // epic's end date is secret
	}}
	seedProject(app, "PRJ-A", "Root", nil)
	seedProject(app, "PRJ-B", "Child", map[string]any{
		"planned_start": "2026-01-10", "planned_end": "2026-03-01"})
	seedEpic(app, "EPIC-1", "Epic", "2026-02-01", "2026-09-30")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-1"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	root := resp.Roots[0]
	// The epic's hidden end (2026-09-30) must not appear anywhere — not on
	// the epic's own node, not laundered into the parent's rolled span.
	if strings.Contains(ganttBody(t, resp), "2026-09-30") {
		t.Errorf("hidden date value leaked into the response: %s", ganttBody(t, resp))
	}
	if root.Rolled == nil || root.Rolled.End != "2026-03-01" {
		t.Errorf("rolled = %+v, want end 2026-03-01 (hidden date must not fold)", root.Rolled)
	}
}

// TestGantt_TruncatedIsPostFilter pins RR-Y7MINP: the node cap and truncated
// flag are computed on the ACL-filtered tree, so hidden nodes can never make
// a restricted principal's response look truncated.
func TestGantt_TruncatedIsPostFilter(t *testing.T) {
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) { g.MaxNodes = 2 })
	seedProject(app, "PRJ-A", "Root", nil)
	seedProject(app, "PRJ-B", "Child", map[string]any{
		"planned_start": "2026-01-01", "planned_end": "2026-02-01"})
	seedEpic(app, "EPIC-1", "Third", "2026-01-01", "2026-02-01")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-1"})

	// Privileged: three visible nodes against a cap of two → truncated.
	priv := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if !priv.Truncated {
		t.Fatalf("privileged view should be truncated at MaxNodes=2")
	}

	// Alice sees only the two projects: exactly at the cap, nothing beyond
	// it is VISIBLE — so truncated must be false, or its value would betray
	// the hidden epic's existence.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"project"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	resp := decodeGantt(t, ganttGet(gateCtxFor(aliceCtx(), t, d), app, "plan"))
	if resp.Truncated {
		t.Errorf("ORACLE: truncated=true for a principal whose visible tree fits the cap")
	}
}

// TestGantt_DrillRootUniform404 pins the ?root= contract: a hidden root and a
// missing root are byte-identical 404s (an entity id, unlike the config key).
func TestGantt_DrillRootUniform404(t *testing.T) {
	app := newGanttTestApp(t)
	seedProject(app, "PRJ-A", "Root", nil)
	seedEpic(app, "EPIC-H", "Hidden", "2026-01-01", "2026-02-01")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-H"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"project"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	hidden := ganttGet(ctx, app, "plan?root=EPIC-H")
	missing := ganttGet(ctx, app, "plan?root=PRJ-NONEXISTENT")
	if hidden.Code != http.StatusNotFound || missing.Code != http.StatusNotFound {
		t.Fatalf("both should 404: hidden=%d missing=%d", hidden.Code, missing.Code)
	}
	if stripInstance(t, hidden.Body.String()) != stripInstance(t, missing.Body.String()) {
		t.Errorf("hidden and missing root bodies differ:\n%s\n%s", hidden.Body, missing.Body)
	}

	// And a visible root re-roots the response.
	sub := decodeGantt(t, ganttGet(context.Background(), app, "plan?root=EPIC-H"))
	if len(sub.Roots) != 1 || sub.Roots[0].ID != "EPIC-H" {
		t.Errorf("drill re-root failed: %+v", sub.Roots)
	}
}

// TestGantt_DepthCapFoldsBeyond pins the MaxDepth semantics: children past
// the cap are not emitted, but their dates are already inside the ancestor's
// rolled span because the fold runs before emission.
func TestGantt_DepthCapFoldsBeyond(t *testing.T) {
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) { g.MaxDepth = 1 })
	seedProject(app, "PRJ-A", "Root", nil)
	seedProject(app, "PRJ-B", "Mid", nil)
	seedEpic(app, "EPIC-DEEP", "Deep", "2026-01-05", "2026-08-20")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "has-epic", To: "EPIC-DEEP"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	root := resp.Roots[0]
	if len(root.Children) != 1 || len(root.Children[0].Children) != 0 {
		t.Fatalf("depth cap should emit exactly one level: %+v", root)
	}
	if root.Rolled == nil || root.Rolled.Start != "2026-01-05" || root.Rolled.End != "2026-08-20" {
		t.Errorf("beyond-cap dates must still fold: rolled = %+v", root.Rolled)
	}
}

// TestGantt_HierarchyConfigOrderWins pins the cross-relation-type half of
// parent-selection determinism: hierarchy list order decides which relation
// type claims a child when two types both point at it. (The within-type half
// — lexicographic edge sort — is pinned by TestGantt_MultiParentFirst.)
func TestGantt_HierarchyConfigOrderWins(t *testing.T) {
	// Both relation types can parent EPIC-S; `contains` is not valid
	// project→epic in the metamodel, so use two projects and give has-epic a
	// competitor by reversing the hierarchy list instead.
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) {
		g.Hierarchy = []string{"has-epic", "contains"}
	})
	seedProject(app, "PRJ-A", "Alpha", nil)
	seedProject(app, "PRJ-B", "Beta", nil)
	// PRJ-B is claimed by PRJ-A twice: via contains and (illegally per
	// metamodel, but the store does not enforce that) via has-epic from a
	// LATER-sorting parent. The has-epic edge must win because its type is
	// first in the hierarchy list, even though "PRJ-Z" sorts after "PRJ-A".
	seedProject(app, "PRJ-Z", "Zulu", nil)
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-Z", Type: "has-epic", To: "PRJ-B"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	var parentOfB string
	var walk func(parent string, ns []v1.GanttNode)
	walk = func(parent string, ns []v1.GanttNode) {
		for _, n := range ns {
			if n.ID == "PRJ-B" {
				parentOfB = parent
			}
			walk(n.ID, n.Children)
		}
	}
	walk("", resp.Roots)
	if parentOfB != "PRJ-Z" {
		t.Errorf("parent of PRJ-B = %q, want PRJ-Z (has-epic is first in hierarchy config order)", parentOfB)
	}
}

// TestGantt_WhereOnHiddenPropertyMatchesNothing pins the post-redact filter
// contract as a TEST, not prose: membership must never reflect a predicate
// over a value the principal cannot read.
func TestGantt_WhereOnHiddenPropertyMatchesNothing(t *testing.T) {
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) {
		g.Sources["project"] = dataentryconfig.GanttSource{
			Start: "planned_start", End: "planned_end",
			Where: []string{"planned_end>=2026-01-01"},
		}
	})
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"planned_end": false},
	}}
	seedProject(app, "PRJ-A", "Root", map[string]any{
		"planned_start": "2026-01-01", "planned_end": "2026-03-01"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	// The raw value (2026-03-01) WOULD match; the redacted view has no value,
	// so the entity must be excluded — otherwise its presence is a one-bit
	// oracle on the hidden value.
	if len(resp.Roots) != 0 {
		t.Errorf("ORACLE: where over a hidden property admitted an entity: %+v", resp.Roots)
	}
}

// TestGantt_WhereMatchErrorExcludes pins the match-ERROR direction: an
// unparseable stored value under a comparison filter excludes the entity
// (logged server-side), it does not fail the request or slip through.
func TestGantt_WhereMatchErrorExcludes(t *testing.T) {
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) {
		g.Sources["project"] = dataentryconfig.GanttSource{
			Start: "planned_start", End: "planned_end",
			Where: []string{"planned_end<2026-06-01"},
		}
	})
	seedProject(app, "PRJ-BAD", "BadDate", map[string]any{"planned_end": "soon"})
	seedProject(app, "PRJ-OK", "Good", map[string]any{
		"planned_start": "2026-01-01", "planned_end": "2026-02-01"})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if len(resp.Roots) != 1 || resp.Roots[0].ID != "PRJ-OK" {
		t.Errorf("match-error entity should be excluded, good one kept: %+v", resp.Roots)
	}
}

// TestGantt_DrillDepthIsRelative pins the flame-graph depth semantic: the
// depth cap is measured from the RESPONSE root, so drilling reaches levels a
// root-level response cannot carry.
func TestGantt_DrillDepthIsRelative(t *testing.T) {
	app := newGanttTestApp(t, func(g *dataentryconfig.Gantt) { g.MaxDepth = 1 })
	seedProject(app, "PRJ-A", "L0", nil)
	seedProject(app, "PRJ-B", "L1", nil)
	seedProject(app, "PRJ-C", "L2", nil)
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-B", Type: "contains", To: "PRJ-C"})

	top := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if len(top.Roots[0].Children) != 1 || len(top.Roots[0].Children[0].Children) != 0 {
		t.Fatalf("root response should stop at depth 1: %+v", top.Roots)
	}

	drilled := decodeGantt(t, ganttGet(context.Background(), app, "plan?root=PRJ-B"))
	if len(drilled.Roots) != 1 || len(drilled.Roots[0].Children) != 1 ||
		drilled.Roots[0].Children[0].ID != "PRJ-C" {

		t.Errorf("drilling re-roots the depth cap; PRJ-C should now be reachable: %+v", drilled.Roots)
	}
}

// TestGantt_TimeTypedDateProperties pins the YAML-frontmatter shape: an
// unquoted date parses to a time.Time, not a string, and the date roles must
// read it (GetString silently drops it — the entityTimeValue hazard).
func TestGantt_TimeTypedDateProperties(t *testing.T) {
	app := newGanttTestApp(t)
	seedEntity(app, &entity.Entity{ID: "PRJ-T", Type: "project", Properties: map[string]any{
		"title":         "TimeTyped",
		"planned_start": time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		"planned_end":   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}})

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	root := resp.Roots[0]
	if root.Planned == nil || root.Planned.Start != "2026-02-01" || root.Planned.End != "2026-04-01" {
		t.Errorf("time.Time-valued dates must be read: planned = %+v", root.Planned)
	}
}

// TestGantt_EmptyForest pins the empty edge case: no matching entities is an
// empty 200, not an error.
func TestGantt_EmptyForest(t *testing.T) {
	app := newGanttTestApp(t)
	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if len(resp.Roots) != 0 || resp.Truncated {
		t.Errorf("empty store should give an empty forest: %+v", resp)
	}
}

// TestSidebar_GanttEntry pins the navigation surface, mirroring the calendar
// sidebar tests: href + icon minted server-side, and a permission-gated entry
// hidden from a principal without the grant.
func TestSidebar_GanttEntry(t *testing.T) {
	app := newGanttTestApp(t)
	next := *app.State().Cfg
	next.Navigation = []dataentryconfig.NavigationEntry{{Label: "Plan", Gantt: "plan"}}
	app.schema.Publish(&Schema{Cfg: &next, Meta: app.State().Meta})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sidebar: got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/gantt/plan") {
		t.Errorf("sidebar should mint /gantt/plan href: %s", body)
	}
	if !strings.Contains(body, `"gantt"`) {
		t.Errorf("sidebar should carry the gantt icon: %s", body)
	}
}

// TestSidebar_GanttHiddenWithoutPermission mirrors the calendar's test: a
// permission-gated gantt entry is filtered from the sidebar for a principal
// without the grant. (A UX filter only — the endpoint stays reachable and
// returns ACL-scoped rows.)
func TestSidebar_GanttHiddenWithoutPermission(t *testing.T) {
	app := newGanttTestApp(t)
	next := *app.State().Cfg
	next.Navigation = []dataentryconfig.NavigationEntry{
		{Label: "Plan", Gantt: "plan", Permission: "planner"},
	}
	app.schema.Publish(&Schema{Cfg: &next, Meta: app.State().Meta})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"project"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sidebar: got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "/gantt/plan") {
		t.Errorf("gated gantt entry should be hidden without the permission: %s", rec.Body)
	}
}
