package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestAnalyzeSchema_CountsTriggerReferences pins that `analyze schema` reads
// the flows.yaml triggers: an unused type a trigger fires on is reported
// with a flows.yaml reference of kind "trigger". Without flows.yaml the
// same type has no references.
func TestAnalyzeSchema_CountsTriggerReferences(t *testing.T) {
	const flowsYAML = "triggers:\n  stamp-note:\n    on: {entity: note, created: true}\n" +
		"    outputs:\n      - set: {properties: {title: stamped}}\n"
	want := reference{File: "flows.yaml", Section: "triggers.stamp-note", Kind: "trigger"}

	tests := []struct {
		name  string
		flows string
		want  bool
	}{
		{"flows.yaml trigger", flowsYAML, true},
		{"no flows.yaml", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			refs := analyzeSchemaNoteRefs(t, tc.flows)
			if got := slices.Contains(refs, want); got != tc.want {
				t.Errorf("note references = %+v, contains %+v = %v, want %v", refs, want, got, tc.want)
			}
		})
	}
}

// reference mirrors the JSON shape of a schema.Reference.
type reference struct {
	File    string `json:"file"`
	Section string `json:"section"`
	Kind    string `json:"kind"`
}

// analyzeSchemaNoteRefs runs `analyze schema` over a project whose note type
// has no entities, with flows as its flows.yaml unless empty, and returns
// the references reported for note.
func analyzeSchemaNoteRefs(t *testing.T, flows string) []reference {
	t.Helper()
	title := map[string]metamodel.PropertyDef{"title": {Type: "string"}}
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"item": {Label: "Item", IDPrefix: "ITEM", Properties: title},
		"note": {Label: "Note", IDPrefix: "NOTE", Properties: title},
	}}
	st := memstore.New()
	ctx := context.Background()
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "ITEM-1", Type: "item"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fs := storage.NewMemFS()
	if err := fs.MkdirAll("/cfg", 0o755); err != nil {
		t.Fatal(err)
	}
	if flows != "" {
		if err := fs.WriteFile("/cfg/flows.yaml", []byte(flows), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	deps := newTestDeps(t, meta, st)
	deps.Config = config.NewFSLoader(fs, "/cfg")
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	text, isError := callTool(t, srv, "analyze", `{"check":"schema"}`)
	if isError {
		t.Fatalf("analyze schema: %s", text)
	}
	var out struct {
		Results struct {
			UnusedEntityTypes []struct {
				Name       string      `json:"name"`
				References []reference `json:"references"`
			} `json:"unused_entity_types"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	for _, u := range out.Results.UnusedEntityTypes {
		if u.Name == "note" {
			return u.References
		}
	}
	t.Fatalf("note not reported unused: %s", text)
	return nil
}
