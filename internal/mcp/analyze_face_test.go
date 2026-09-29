package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// facedGatedServer is gatedServer over a faced type (BUG-95W7MV, A4):
// `policy` has a draft and a published face, and alice may read what read
// grants. POL-1 has both faces and one edge, cited from its draft, to
// CTL-1; POL-2 has a published face and no edge. extra rows are seeded
// after these. A policy title is declared unique.
func facedGatedServer(t *testing.T, read []string, extra ...*entity.Entity) (*Server, context.Context) {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {
				Label: "Policy", IDPrefix: "POL", DisplayProperty: "title",
				Faces:      map[string]metamodel.FaceDef{"draft": {}, "published": {}},
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Unique: true}},
			},
			"control": {
				Label: "Control", IDPrefix: "CTL", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"cites": {Label: "cites", From: []string{"policy"}, To: []string{"control"}, Scope: metamodel.ScopeContent},
		},
	}
	st := memstore.New()
	ctx := context.Background()
	for _, e := range append([]*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Draft secret"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "Published"}},
		{ID: "POL-2", Type: "policy", Face: "published", Properties: map[string]any{"title": "Two"}},
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Control"}},
	}, extra...) {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
		}
	}
	if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "CTL-1"}, &store.RelationData{}); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: read}},
		Assignments: map[string]string{"alice": "reader"},
	}, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	reads := svc.GatedReads()
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, Deps{
		Store:         reads.Reader,
		Traversals:    reads.Traversals,
		Meta:          meta,
		Tracer:        reads.Tracer,
		Searcher:      reads.Searcher,
		Validator:     reads.Validator,
		EntityManager: svc.EntityManager(),
		Config:        svc.Config(),
		LuaWriteDeps:  lua.WriteDeps{ReadDeps: reads.LuaReads, EntityManager: svc.EntityManager()},
		Watcher:       nopWatcher{},
		ProjectRoot:   t.TempDir(),
		Attachments:   testAttachmentDeps(t, svc, meta, audit.Nop{}),
	})
	return srv, principal.With(ctx, principal.Principal{User: "alice", Tool: principal.ToolMCP})
}

var (
	everyFace     = []string{"policy@draft", "policy@published", "control"}
	publishedOnly = []string{"policy@published", "control"}
)

// orphansOf runs analyze orphans and returns the decoded findings.
func orphansOf(ctx context.Context, t *testing.T, s *Server) []orphanSummary {
	t.Helper()
	res, err := s.handleAnalyze(ctx, makeToolRequest(map[string]any{"check": checkOrphans}))
	if err != nil || isErrorResult(res) {
		t.Fatalf("analyze orphans: %v %v", err, res)
	}
	text := getResultText(t, res)
	if strings.HasPrefix(text, "No orphan") {
		return nil
	}
	var body struct {
		Coverage string          `json:"coverage"`
		Results  []orphanSummary `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	if body.Coverage != coverageFamily {
		t.Errorf("coverage = %q, want %q", body.Coverage, coverageFamily)
	}
	return body.Results
}

func TestAnalyzeOrphans_FacedFamilies(t *testing.T) {
	t.Parallel()
	s, ctx := facedGatedServer(t, everyFace)
	got := orphansOf(ctx, t, s)
	if len(got) != 1 || got[0].ID != "POL-2" || !reflect.DeepEqual(got[0].Faces, []entity.Face{"published"}) {
		t.Fatalf("orphans = %+v, want POL-2 [published]", got)
	}
}

// A published-only principal: the draft is in no face list, and the edge
// hung from it connects nothing.
func TestAnalyzeOrphans_HiddenFace(t *testing.T) {
	t.Parallel()
	s, ctx := facedGatedServer(t, publishedOnly)
	got := orphansOf(ctx, t, s)
	ids := make([]string, 0, len(got))
	for _, o := range got {
		ids = append(ids, o.ID)
		if slices.Contains(o.Faces, "draft") {
			t.Errorf("LEAK: hidden draft face listed for %s", o.ID)
		}
	}
	if !reflect.DeepEqual(ids, []string{"CTL-1", "POL-1", "POL-2"}) {
		t.Fatalf("orphans = %+v, want CTL-1, POL-1 and POL-2", got)
	}
}

func TestTrace_FacedFamilyAndHiddenFace(t *testing.T) {
	t.Parallel()
	trace := func(read []string) string {
		s, ctx := facedGatedServer(t, read)
		res, err := group(s, selTrace).handleTrace(ctx, makeToolRequest(map[string]any{"id": "POL-1"}))
		if err != nil || isErrorResult(res) {
			t.Fatalf("trace: %v %v", err, res)
		}
		return getResultText(t, res)
	}
	all := trace(everyFace)
	if !strings.Contains(all, `"draft"`) || !strings.Contains(all, "CTL-1") {
		t.Fatalf("full trace must list both faces and the draft edge: %s", all)
	}
	pub := trace(publishedOnly)
	if strings.Contains(pub, "draft") || strings.Contains(pub, "CTL-1") || strings.Contains(pub, "Draft secret") {
		t.Fatalf("LEAK: published-only trace shows the draft or its edge: %s", pub)
	}
	if !strings.Contains(pub, `"published"`) {
		t.Fatalf("published-only trace must list the published face: %s", pub)
	}
}

// unique is judged per face, and a hidden face takes no part: POL-3 shares
// POL-1's draft title and POL-2's published title.
func TestAnalyzeUnique_PerFace(t *testing.T) {
	t.Parallel()
	pol3 := []*entity.Entity{
		{ID: "POL-3", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Draft secret"}},
		{ID: "POL-3", Type: "policy", Face: "published", Properties: map[string]any{"title": "Two"}},
	}
	unique := func(read []string) (string, []uniqueViolation) {
		s, ctx := facedGatedServer(t, read, pol3...)
		res, err := s.handleAnalyze(ctx, makeToolRequest(map[string]any{"check": checkUnique}))
		if err != nil || isErrorResult(res) {
			t.Fatalf("analyze unique: %v %v", err, res)
		}
		text := getResultText(t, res)
		var body struct {
			Coverage string            `json:"coverage"`
			Results  []uniqueViolation `json:"results"`
		}
		if err := json.Unmarshal([]byte(text), &body); err != nil {
			t.Fatalf("decode %s: %v", text, err)
		}
		if body.Coverage != coveragePerFace {
			t.Errorf("coverage = %q, want %q", body.Coverage, coveragePerFace)
		}
		return text, body.Results
	}
	_, got := unique(everyFace)
	want := []uniqueViolation{
		{EntityType: "policy", Property: "title", Face: "draft", Value: "Draft secret", EntityIDs: []string{"POL-1", "POL-3"}},
		{EntityType: "policy", Property: "title", Face: "published", Value: "Two", EntityIDs: []string{"POL-2", "POL-3"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unique = %+v, want %+v", got, want)
	}
	text, got := unique(publishedOnly)
	if !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("published-only unique = %+v, want %+v", got, want[1:])
	}
	if strings.Contains(text, "Draft secret") {
		t.Fatalf("LEAK: a hidden draft value in %s", text)
	}
}

// TKT-5LW875: a cardinality count is folded from the edges the principal may
// read. POL-1's draft cites CTL-1, and cites allows none. A principal who can
// read POL-1 but not CTL-1 sees no edge, so neither a violation nor a count
// that reveals the hidden control.
func TestAnalyzeCardinality_CountsVisibleEdgesOnly(t *testing.T) {
	t.Parallel()
	run := func(read []string) string {
		s, ctx := facedGatedServer(t, read)
		meta := s.state.current().deps.Meta
		def := meta.Relations["cites"]
		zero := 0
		def.MaxOutgoing = &zero
		meta.Relations["cites"] = def
		res, err := s.handleAnalyzeCardinality(ctx, makeToolRequest(map[string]any{}))
		if err != nil || isErrorResult(res) {
			t.Fatalf("analyze cardinality: %v %v", err, res)
		}
		return getResultText(t, res)
	}
	all := run(everyFace)
	if !strings.Contains(all, "POL-1") || !strings.Contains(all, "has more than 0 'cites' relation(s): 1") {
		t.Fatalf("POL-1's draft cites CTL-1, want a max violation: %s", all)
	}
	hidden := run([]string{"policy@draft", "policy@published"})
	if strings.Contains(hidden, "POL-1") || strings.Contains(hidden, "CTL-1") {
		t.Fatalf("LEAK: a count over a hidden neighbor: %s", hidden)
	}
}

// TestReadRelationResource_TailFace pins the relation resource's `{from}`
// segment (TKT-KQXVF7). `ID@face` names a content edge's tail, a bare ID
// names the identity edge, and a tail the caller cannot read is not found.
func TestReadRelationResource_TailFace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		read    []string
		from    string
		wantErr bool
	}{
		{"tail readable", everyFace, "POL-1@draft", false},
		{"bare id misses the content edge", everyFace, "POL-1", true},
		{"tail face hidden", publishedOnly, "POL-1@draft", true},
		{"malformed from", everyFace, "POL-1@", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, ctx := facedGatedServer(t, tc.read)
			res, err := group(s, selSchemaRes).handleReadRelation(ctx,
				readResourceReq("rela://relation/"+tc.from+"/cites/CTL-1"))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("read %s: want not-found, got %s", tc.from, res.Contents[0].Text)
				}
				return
			}
			if err != nil {
				t.Fatalf("read %s: %v", tc.from, err)
			}
			if !strings.Contains(res.Contents[0].Text, `"draft"`) {
				t.Errorf("relation %s lost its tail: %s", tc.from, res.Contents[0].Text)
			}
		})
	}
}
