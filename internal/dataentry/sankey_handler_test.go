package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// newSankeyTestApp builds an app with risk -> control -> clause entities and a
// sankey "flow" over them.
func newSankeyTestApp(t *testing.T, mutate ...func(*dataentryconfig.Sankey)) *App {
	t.Helper()
	return newSankeyTestAppOn(t, nil, mutate...)
}

// newSankeyTestAppOn is newSankeyTestApp over a store of the caller's choice
// (appbuildtest.WithStore), for tests that count store reads.
func newSankeyTestAppOn(t *testing.T, opts []appbuildtest.Option, mutate ...func(*dataentryconfig.Sankey)) *App {
	t.Helper()
	typeDef := func(prefix string) metamodel.EntityDef {
		return metamodel.EntityDef{
			Label: prefix, IDPrefix: prefix + "-", DisplayProperty: "title",
			Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: "string", Required: true},
				"status": {Type: "string"},
				"num":    {Type: "string"},
			},
			PropertyOrder: []string{"title", "status", "num"},
		}
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"risk": typeDef("RISK"), "control": typeDef("CTL"), "clause": typeDef("CL"), "evidence": typeDef("EV"),
		},
		Relations: map[string]metamodel.RelationDef{
			"mitigated-by": {Label: "mitigated by", From: []string{"risk"}, To: []string{"control"}},
			"covers":       {Label: "covers", From: []string{"control"}, To: []string{"clause"}},
			"evidenced-by": {Label: "evidenced by", From: []string{"control"}, To: []string{"evidence"}},
		},
	}
	sk := dataentryconfig.Sankey{
		Title:   "Flow",
		Columns: [][]string{{"risk"}, {"control"}, {"clause"}},
		Edges:   []string{"mitigated-by", "covers"},
		Types: map[string]dataentryconfig.SankeyType{
			"control": {Order: "num", ColorBy: "status"},
		},
	}
	for _, m := range mutate {
		m(&sk)
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Sankey Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Sankeys:    map[string]dataentryconfig.Sankey{"flow": sk},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	dataentryconfig.NormalizeSankeys(cfg)
	return newAppFromParts(cfg, meta, newFixture(), opts...)
}

func sankeyGet(ctx context.Context, app *App, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sankeys/"+id, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.sankey.handleV1Sankey(rec, req)
	return rec
}

func decodeSankey(t *testing.T, rec *httptest.ResponseRecorder) v1.SankeyResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var resp v1.SankeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return resp
}

func seedSankeyData(app *App) {
	seedEntity(app, &entity.Entity{ID: "RISK-1", Type: "risk", Properties: map[string]any{"title": "Phishing"}})
	seedEntity(app, &entity.Entity{ID: "RISK-2", Type: "risk", Properties: map[string]any{"title": "Loss"}})
	seedEntity(app, &entity.Entity{ID: "CTL-1", Type: "control", Properties: map[string]any{
		"title": "MFA", "status": "done", "num": "1.1"}})
	seedEntity(app, &entity.Entity{ID: "CTL-2", Type: "control", Properties: map[string]any{
		"title": "Backups", "status": "planned", "num": "1.2"}})
	seedEntity(app, &entity.Entity{ID: "CL-1", Type: "clause", Properties: map[string]any{"title": "A.5"}})
	seedRelation(app, &entity.Relation{From: "RISK-1", Type: "mitigated-by", To: "CTL-1"})
	seedRelation(app, &entity.Relation{From: "RISK-2", Type: "mitigated-by", To: "CTL-2"})
	seedRelation(app, &entity.Relation{From: "CTL-1", Type: "covers", To: "CL-1"})
}

func sankeyIDs(r v1.SankeyResponse) string {
	ids := []string{}
	for _, n := range r.Nodes {
		ids = append(ids, n.ID)
	}
	return strings.Join(ids, ",")
}

func TestSankey_UnknownConfigID404(t *testing.T) {
	app := newSankeyTestApp(t)
	rec := sankeyGet(context.Background(), app, "ghost")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Sankey not found") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestSankey_NodesAndLinks(t *testing.T) {
	app := newSankeyTestApp(t)
	seedSankeyData(app)
	resp := decodeSankey(t, sankeyGet(context.Background(), app, "flow"))

	if got := sankeyIDs(resp); got != "RISK-1,RISK-2,CTL-1,CTL-2,CL-1" {
		t.Errorf("nodes = %s", got)
	}
	var ctl v1.SankeyNode
	for _, n := range resp.Nodes {
		if n.ID == "CTL-2" {
			ctl = n
		}
	}
	if ctl.Title != "Backups" || ctl.Tone != "planned" || ctl.SortKey != "1.2" {
		t.Errorf("CTL-2 = %+v", ctl)
	}
	if len(resp.Links) != 3 {
		t.Errorf("links = %+v, want 3", resp.Links)
	}
	if resp.Truncated {
		t.Error("truncated should be false")
	}
}

func TestSankey_WhereFiltersNodesAndDropsTheirLinks(t *testing.T) {
	app := newSankeyTestApp(t, func(s *dataentryconfig.Sankey) {
		s.Types["control"] = dataentryconfig.SankeyType{Where: []string{"status=done"}}
	})
	seedSankeyData(app)
	resp := decodeSankey(t, sankeyGet(context.Background(), app, "flow"))
	if got := sankeyIDs(resp); got != "RISK-1,RISK-2,CTL-1,CL-1" {
		t.Errorf("nodes = %s", got)
	}
	for _, l := range resp.Links {
		if l.Source == "CTL-2" || l.Target == "CTL-2" {
			t.Errorf("link to a filtered node: %+v", l)
		}
	}
}

func TestSankey_MaxNodesTruncates(t *testing.T) {
	app := newSankeyTestApp(t, func(s *dataentryconfig.Sankey) { s.MaxNodes = 3 })
	seedSankeyData(app)
	resp := decodeSankey(t, sankeyGet(context.Background(), app, "flow"))
	if len(resp.Nodes) != 3 || !resp.Truncated {
		t.Fatalf("nodes=%d truncated=%v", len(resp.Nodes), resp.Truncated)
	}
	ids := map[string]bool{}
	for _, n := range resp.Nodes {
		ids[n.ID] = true
	}
	for _, l := range resp.Links {
		if !ids[l.Source] || !ids[l.Target] {
			t.Errorf("link leaves the node set: %+v", l)
		}
	}
}

// TestSankey_ACLHidesNodesAndTheirLinks: a principal who may not read controls
// sees no control node and no link that ends at one.
func TestSankey_ACLHidesNodesAndTheirLinks(t *testing.T) {
	app := newSankeyTestApp(t)
	seedSankeyData(app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"risk", "clause"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	resp := decodeSankey(t, sankeyGet(gateCtxFor(aliceCtx(), t, d), app, "flow"))
	if got := sankeyIDs(resp); got != "RISK-1,RISK-2,CL-1" {
		t.Errorf("nodes = %s", got)
	}
	if len(resp.Links) != 0 {
		t.Errorf("LEAK: links to hidden nodes: %+v", resp.Links)
	}
	b, _ := json.Marshal(resp)
	if strings.Contains(string(b), "CTL-") {
		t.Errorf("hidden entity id in response: %s", b)
	}
}

func TestSankey_MethodNotAllowed(t *testing.T) {
	app := newSankeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_sankeys/flow", http.NoBody)
	rec := httptest.NewRecorder()
	app.sankey.handleV1Sankey(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d", rec.Code)
	}
}

func withPips(limit int) func(*dataentryconfig.Sankey) {
	return func(s *dataentryconfig.Sankey) {
		c := s.Types["control"]
		c.Pips = &dataentryconfig.SankeyPips{Relation: "evidenced-by", Type: "evidence", ColorBy: "status", Max: limit}
		s.Types["control"] = c
	}
}

func seedEvidence(app *App, control string, n int) {
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("EV-%s-%02d", control, i)
		status := "ok"
		if i%2 == 0 {
			status = "stale"
		}
		seedEntity(app, &entity.Entity{ID: id, Type: "evidence", Properties: map[string]any{
			"title": "Doc " + id, "status": status}})
		seedRelation(app, &entity.Relation{From: control, Type: "evidenced-by", To: id})
	}
}

func nodeByID(r v1.SankeyResponse, id string) v1.SankeyNode {
	for _, n := range r.Nodes {
		if n.ID == id {
			return n
		}
	}
	return v1.SankeyNode{}
}

func TestSankey_PipsAreSortedCappedAndCounted(t *testing.T) {
	app := newSankeyTestApp(t, withPips(2))
	seedSankeyData(app)
	seedEvidence(app, "CTL-1", 3)
	resp := decodeSankey(t, sankeyGet(context.Background(), app, "flow"))

	n := nodeByID(resp, "CTL-1")
	if len(n.Pips) != 2 || n.PipsMore != 1 {
		t.Fatalf("pips=%+v more=%d, want 2 and 1", n.Pips, n.PipsMore)
	}
	if n.Pips[0].ID != "EV-CTL-1-01" || n.Pips[1].ID != "EV-CTL-1-02" {
		t.Errorf("pips not ordered by id: %+v", n.Pips)
	}
	if n.Pips[0].Tone != "ok" || n.Pips[1].Tone != "stale" || n.Pips[0].Title != "Doc EV-CTL-1-01" {
		t.Errorf("pip tone/title wrong: %+v", n.Pips)
	}
	if got := nodeByID(resp, "CTL-2"); len(got.Pips) != 0 || got.PipsMore != 0 {
		t.Errorf("CTL-2 has no evidence but got %+v", got)
	}
	if len(resp.Nodes) != 5 {
		t.Errorf("pips must not become nodes: %s", sankeyIDs(resp))
	}
}

// TestSankey_ACLHidesPipsAndTheirCount: a principal who may not read evidence
// sees neither the dots nor the "+n" count, which would reveal how many exist.
func TestSankey_ACLHidesPipsAndTheirCount(t *testing.T) {
	app := newSankeyTestApp(t, withPips(2))
	seedSankeyData(app)
	seedEvidence(app, "CTL-1", 3)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"risk", "control", "clause"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	resp := decodeSankey(t, sankeyGet(gateCtxFor(aliceCtx(), t, d), app, "flow"))
	n := nodeByID(resp, "CTL-1")
	if len(n.Pips) != 0 || n.PipsMore != 0 {
		t.Errorf("LEAK: pips=%+v more=%d", n.Pips, n.PipsMore)
	}
	b, _ := json.Marshal(resp)
	if strings.Contains(string(b), "EV-") {
		t.Errorf("hidden entity id in response: %s", b)
	}
}

// TestSankey_PipsCostFollowsTheDiagramNotTheStore pins the batching: the store
// calls and rows read are the same over a store holding 100 and 1,000 more
// evidence rows, and the same however many controls carry pips. The padding is
// the pip type with edges of the pip relation that touch no node, which is
// what a scan of the type or of the relation would return.
func TestSankey_PipsCostFollowsTheDiagramNotTheStore(t *testing.T) {
	measure := func(controls, pad int) (storetest.Cost, string) {
		counting := storetest.NewCounting(memstore.New())
		app := newSankeyTestAppOn(t, []appbuildtest.Option{appbuildtest.WithStore(counting)}, withPips(3))
		for i := 1; i <= controls; i++ {
			id := fmt.Sprintf("CTL-%03d", i)
			seedEntity(app, &entity.Entity{ID: id, Type: "control", Properties: map[string]any{"title": id}})
			seedEvidence(app, id, 2)
		}
		for i := range pad {
			id := fmt.Sprintf("EV-PAD-%04d", i)
			seedEntity(app, &entity.Entity{ID: id, Type: "evidence", Properties: map[string]any{"title": id}})
			seedRelation(app, &entity.Relation{From: id, Type: "evidenced-by", To: fmt.Sprintf("EV-PAD-%04d", i+1)})
		}
		counting.Reset()
		decodeSankey(t, sankeyGet(context.Background(), app, "flow"))
		return counting.Cost(), counting.String()
	}
	storetest.AssertScaleFree(t, func(pad int) (storetest.Cost, string) { return measure(3, pad) })

	small, smallDetail := measure(3, 0)
	large, largeDetail := measure(30, 0)
	if small.Calls != large.Calls {
		t.Errorf("store calls grow with the node count: %d at 3 controls, %d at 30 (%s | %s)",
			small.Calls, large.Calls, smallDetail, largeDetail)
	}
}

// TestSankey_TruncatedIsPostFilter: the node cap is applied after the ACL
// filter, so hidden nodes cannot make a restricted response look truncated.
func TestSankey_TruncatedIsPostFilter(t *testing.T) {
	app := newSankeyTestApp(t, func(s *dataentryconfig.Sankey) { s.MaxNodes = 3 })
	seedSankeyData(app) // 5 nodes: 3 are visible to alice (risks and clause)

	if !decodeSankey(t, sankeyGet(context.Background(), app, "flow")).Truncated {
		t.Fatal("privileged view should be truncated at MaxNodes=3")
	}
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"risk", "clause"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	resp := decodeSankey(t, sankeyGet(gateCtxFor(aliceCtx(), t, d), app, "flow"))
	if len(resp.Nodes) != 3 || resp.Truncated {
		t.Errorf("ORACLE: nodes=%d truncated=%v for a principal whose visible set fits the cap",
			len(resp.Nodes), resp.Truncated)
	}
}

// TestSankey_HiddenFieldIsJudgedAfterRedaction: a `visible:`-hidden property
// gives no tone, matches no where: clause and does not color a pip. Judging
// the raw value would let `where: status=done` reveal it by which nodes appear.
func TestSankey_HiddenFieldIsJudgedAfterRedaction(t *testing.T) {
	hideStatus := fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"status": false}}}

	t.Run("tone", func(t *testing.T) {
		app := newSankeyTestApp(t, withPips(3))
		setTestResolver(t, app, hideStatus)
		seedSankeyData(app)
		seedEvidence(app, "CTL-1", 2)
		resp := decodeSankey(t, sankeyGet(context.Background(), app, "flow"))
		ctl := nodeByID(resp, "CTL-1")
		if ctl.Tone != "" {
			t.Errorf("LEAK: node tone %q from a hidden property", ctl.Tone)
		}
		if len(ctl.Pips) != 2 {
			t.Fatalf("pips = %+v, want the two evidence rows", ctl.Pips)
		}
		for _, p := range ctl.Pips {
			if p.Tone != "" {
				t.Errorf("LEAK: pip tone %q from a hidden property", p.Tone)
			}
		}
	})

	t.Run("where", func(t *testing.T) {
		app := newSankeyTestApp(t, func(s *dataentryconfig.Sankey) {
			s.Types["control"] = dataentryconfig.SankeyType{Where: []string{"status=done"}}
		})
		setTestResolver(t, app, hideStatus)
		seedSankeyData(app)
		resp := decodeSankey(t, sankeyGet(context.Background(), app, "flow"))
		for _, n := range resp.Nodes {
			if n.Type == "control" {
				t.Errorf("LEAK: where: matched the hidden value, node %s appears", n.ID)
			}
		}
	})
}
