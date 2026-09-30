package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// `rela analyze` and `rela trace` on a project mixing faced and faceless
// types (BUG-95W7MV). A faced type has no row at the bare id; before the
// fix it was absent from every check. The CLI is the operator shell and
// reads ungated; the hidden-face cases are covered on MCP, the data-entry
// analyze endpoint and the visibility tracer.
func facedCLIBundles(t *testing.T) *cliBundles {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {
				Label: "Policy", IDPrefix: "POL-", DisplayProperty: "title",
				Faces:      map[string]metamodel.FaceDef{"draft": {}, "published": {}},
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
			"note": {
				Label: "Note", IDPrefix: "NOTE-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"cites": {From: []string{"policy"}, To: []string{"note"}, Scope: metamodel.ScopeContent},
		},
	}
	ss := newStoreSeeder(meta)
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Access"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "Access"}},
		{ID: "POL-2", Type: "policy", Face: "published", Properties: map[string]any{"title": "Access"}},
		{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "Linked"}},
		{ID: "NOTE-2", Type: "note", Properties: map[string]any{"title": "Loose"}},
	} {
		if err := ss.s.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
		}
	}
	if _, err := ss.s.CreateRelation(ctx, entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "NOTE-1"}, &store.RelationData{}); err != nil {
		t.Fatal(err)
	}
	return ss.build(t)
}

func TestAnalyzeFaced_Text(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func(b *cliBundles) error
		want []string
	}{
		{"orphans", func(b *cliBundles) error { return (&AnalyzeOrphansCmd{}).Run(ctx, b.analysis) }, []string{
			"Coverage: families",
			"- NOTE-2 (note) Loose",
			"- POL-2 (policy) [faces: published]",
		}},
		{"duplicates", func(b *cliBundles) error { return (&AnalyzeDuplicatesCmd{}).Run(ctx, b.analysis) }, []string{
			"- POL-1@draft (policy)",
			"- POL-1@published (policy)",
			"- POL-2@published (policy)",
		}},
		{"trace", func(b *cliBundles) error { return (&TraceFromCmd{ID: "POL-1"}).Run(ctx, b.read) }, []string{
			"POL-1", "{faces: draft, published}", "NOTE-1",
		}},
		{"path", func(b *cliBundles) error { return (&TracePathCmd{From: "NOTE-1", To: "POL-1"}).Run(ctx, b.read) }, []string{
			"POL-1 (policy) {faces: draft, published}",
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := facedCLIBundles(t)
			buf := withOutput(t, output.FormatTable)
			if err := tc.run(b); err != nil {
				t.Fatalf("run: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(buf.String(), w) {
					t.Errorf("output lacks %q:\n%s", w, buf)
				}
			}
		})
	}
}

func TestAnalyzeFaced_OrphansJSON(t *testing.T) {
	b := facedCLIBundles(t)
	buf := withOutput(t, output.FormatJSON)
	if err := (&AnalyzeOrphansCmd{}).Run(context.Background(), b.analysis); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Coverage string `json:"coverage"`
		Count    int    `json:"count"`
		Details  []struct {
			ID    string        `json:"id"`
			Faces []entity.Face `json:"faces"`
		} `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode %s: %v", buf, err)
	}
	if res.Coverage == "" || res.Count != 2 || len(res.Details) != 2 {
		t.Fatalf("result = %+v, want coverage and 2 orphans", res)
	}
	if d := res.Details[1]; d.ID != "POL-2" || len(d.Faces) != 1 || d.Faces[0] != "published" {
		t.Errorf("faced orphan = %+v, want POL-2 [published]", d)
	}
}
