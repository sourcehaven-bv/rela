package templating

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

type mapTemplates map[string][]*Template

func (m mapTemplates) EntityTemplate(_ context.Context, entityType, variant string) (*Template, error) {
	for _, t := range m[entityType] {
		if t.Name == variant {
			return t, nil
		}
	}
	return nil, nil //nolint:nilnil // miss
}

func (m mapTemplates) EntityTemplates(_ context.Context, entityType string) ([]*Template, error) {
	return m[entityType], nil
}

func TestEntityTemplateFor_FallsBackToAbstractType(t *testing.T) {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"isms.task": {Extends: "task"},
		"person":    {},
	}}
	store := mapTemplates{
		"task": {
			{Name: "", EntityType: "task", Content: "shared"},
			{Name: "urgent", EntityType: "task", Content: "shared urgent"},
		},
		"isms.task": {{Name: "urgent", EntityType: "isms.task", Content: "own urgent"}},
	}
	ctx := context.Background()

	got, err := EntityTemplateFor(ctx, store, meta, "isms.task", "")
	if err != nil || got == nil || got.Content != "shared" || got.EntityType != "isms.task" {
		t.Fatalf("default = %+v, %v; want the parent's, retyped", got, err)
	}
	if got, _ := EntityTemplateFor(ctx, store, meta, "isms.task", "urgent"); got.Content != "own urgent" {
		t.Errorf("own template did not win: %q", got.Content)
	}
	if got, _ := EntityTemplateFor(ctx, store, meta, "person", ""); got != nil {
		t.Errorf("person got %+v, want no template", got)
	}

	list, err := EntityTemplatesFor(ctx, store, meta, "isms.task")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "" || list[0].Content != "shared" || list[1].Content != "own urgent" {
		for _, l := range list {
			t.Logf("%q %q", l.Name, l.Content)
		}
		t.Errorf("list = %d templates, want default inherited first and own urgent", len(list))
	}
}
