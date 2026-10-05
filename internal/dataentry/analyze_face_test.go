package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// _analyze on a faced type, for a principal who cannot read one face
// (BUG-95W7MV, A4). Faced families appear; a hidden face is in no face
// list, and an edge hung from it neither connects a family nor shows.
//
// Seed, on top of facedApp's POL-1 (draft, published) and FEAT-1:
//   - POL-2 at draft and published, whose only edge cites FEAT-2 from the draft.
//   - POL-3 at draft only, titled like POL-1's published face.
func analyzeFaceApp(t *testing.T, read []string) *App {
	t.Helper()
	app, _ := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: read}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	// facedMeta names no display property, so every title would be the id
	// and nothing could be a duplicate.
	meta := app.State().Meta
	def := meta.Entities["policy"]
	def.DisplayProperty = "title"
	meta.Entities["policy"] = def
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Two draft"}},
		{ID: "POL-2", Type: "policy", Face: "published", Properties: map[string]any{"title": "Two"}},
		{ID: "POL-3", Type: "policy", Face: "draft", Properties: map[string]any{"title": "PUBLISHED TEXT"}},
		{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "Feature two"}},
	} {
		if err := app.store.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
		}
	}
	for _, e := range []struct {
		from, typ, to string
		tail          entity.Face
	}{
		{"POL-1", "implements", "FEAT-1", ""},
		{"POL-2", "cites", "FEAT-2", "draft"},
	} {
		if _, err := app.store.CreateRelation(ctx, entity.RelationKey{From: e.from, FromFace: e.tail, Type: e.typ, To: e.to}, &store.RelationData{}); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}
	return app
}

// analyzeIssues runs _analyze through the router and returns the issues of
// check, keyed by entity ref, with the raw body.
func analyzeIssues(t *testing.T, app *App, check string) (issues map[string]APIIssue, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("_analyze = %d %s", rec.Code, rec.Body)
	}
	var res APIAnalysisResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]APIIssue)
	for _, iss := range res.Issues {
		if iss.CheckType == check {
			out[entity.FormatStateRef(iss.EntityID, entity.Face(iss.Face))] = iss
		}
	}
	return out, rec.Body.String()
}

func TestAnalyze_FacedTypes(t *testing.T) {
	all := []string{"*", "policy@draft", "policy@published", "world:published"}
	pubOnly := []string{"policy@published", "feature", "world:published"}

	t.Run("orphans every face", func(t *testing.T) {
		got, _ := analyzeIssues(t, analyzeFaceApp(t, all), "Orphans")
		// POL-3 is the only unconnected family; the draft edge connects
		// POL-2 and FEAT-2.
		iss, ok := got["POL-3"]
		if len(got) != 1 || !ok {
			t.Fatalf("orphans = %v, want only POL-3", got)
		}
		if !strings.Contains(iss.Message, "(draft)") {
			t.Errorf("message %q must list the family's faces", iss.Message)
		}
	})

	t.Run("orphans published only", func(t *testing.T) {
		got, body := analyzeIssues(t, analyzeFaceApp(t, pubOnly), "Orphans")
		// The draft edge is hidden, so POL-2 and FEAT-2 are orphans. POL-3
		// has no readable face, so it does not exist for this principal.
		if len(got) != 2 {
			t.Fatalf("orphans = %v, want POL-2 and FEAT-2", got)
		}
		if iss := got["POL-2"]; !strings.Contains(iss.Message, "(published)") || strings.Contains(iss.Message, "draft") {
			t.Errorf("POL-2 message %q must list the published face only", iss.Message)
		}
		if _, ok := got["FEAT-2"]; !ok {
			t.Errorf("FEAT-2's only edge is hidden, so it is an orphan: %v", got)
		}
		for _, leak := range []string{"POL-3", "Two draft", "DRAFT TEXT"} {
			if strings.Contains(body, leak) {
				t.Errorf("LEAK: %q in the response of a published-only principal", leak)
			}
		}
	})

	t.Run("duplicates", func(t *testing.T) {
		got, _ := analyzeIssues(t, analyzeFaceApp(t, all), "Duplicates")
		if _, ok := got["POL-1@published"]; !ok || len(got) != 2 {
			t.Fatalf("duplicates = %v, want POL-1@published and POL-3@draft", got)
		}
		if _, ok := got["POL-3@draft"]; !ok {
			t.Fatalf("duplicates = %v, want POL-3@draft", got)
		}
		got, _ = analyzeIssues(t, analyzeFaceApp(t, pubOnly), "Duplicates")
		if len(got) != 0 {
			t.Fatalf("a hidden face formed a duplicate group: %v", got)
		}
	})

	// cites is content-scoped, so its outgoing bound is judged per face:
	// POL-2's draft cites FEAT-2, its published face cites nothing.
	t.Run("cardinality per face", func(t *testing.T) {
		citesMin := func(app *App) *App {
			meta := app.State().Meta
			def := meta.Relations["cites"]
			one := 1
			def.MinOutgoing = &one
			meta.Relations["cites"] = def
			return app
		}
		keys := func(m map[string]APIIssue) []string {
			out := make([]string, 0, len(m))
			for k := range m {
				out = append(out, k)
			}
			slices.Sort(out)
			return out
		}
		got, _ := analyzeIssues(t, citesMin(analyzeFaceApp(t, all)), "Cardinality")
		want := []string{"POL-1@draft", "POL-1@published", "POL-2@published", "POL-3@draft"}
		if !slices.Equal(keys(got), want) {
			t.Fatalf("cardinality = %v, want %v", keys(got), want)
		}
		got, _ = analyzeIssues(t, citesMin(analyzeFaceApp(t, pubOnly)), "Cardinality")
		want = []string{"POL-1@published", "POL-2@published"}
		if !slices.Equal(keys(got), want) {
			t.Fatalf("published-only cardinality = %v, want %v (no hidden face)", keys(got), want)
		}
	})

	// TKT-5LW875: a count is folded from the edges the principal may read.
	// POL-1 implements FEAT-1; a principal who can read POL-1 but not FEAT-1
	// must see POL-1 with no implements edge, not a count that reveals it.
	t.Run("cardinality counts visible edges only", func(t *testing.T) {
		implementsMin := func(app *App) *App {
			meta := app.State().Meta
			def := meta.Relations["implements"]
			one := 1
			def.MinOutgoing = &one
			meta.Relations["implements"] = def
			return app
		}
		got, _ := analyzeIssues(t, implementsMin(analyzeFaceApp(t, all)), "Cardinality")
		if _, ok := got["POL-1"]; ok {
			t.Fatalf("POL-1 has a readable implements edge, got a violation: %v", got)
		}
		if _, ok := got["POL-2"]; !ok {
			t.Fatalf("POL-2 has no implements edge, want a violation: %v", got)
		}

		noFeature := []string{"policy@draft", "policy@published", "world:published"}
		got, body := analyzeIssues(t, implementsMin(analyzeFaceApp(t, noFeature)), "Cardinality")
		iss, ok := got["POL-1"]
		if !ok {
			t.Fatalf("POL-1's only edge leads to a hidden entity, want a violation: %v", got)
		}
		if !strings.HasSuffix(iss.Message, "has 0") {
			t.Errorf("message %q must count the visible edges only", iss.Message)
		}
		if strings.Contains(body, "FEAT-1") {
			t.Errorf("LEAK: hidden neighbor FEAT-1 in the response")
		}
	})
}
