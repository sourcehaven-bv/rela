package dataentry

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// defaultWorldStub is a WorldLookup that names the schema's default world,
// as worlds.Compiled does. dataentry may not import worlds.
type defaultWorldStub struct{ scope store.WorldScope }

func (s defaultWorldStub) Lookup(string) (store.WorldScope, bool) { return s.scope, true }
func (s defaultWorldStub) DefaultWorld() store.WorldScope         { return s.scope }

// TestSetWorlds_RewiresValidatorWorld pins RR-HKVULG for the request-path
// validator: NewApp builds it before the worlds exist, so setWorlds must
// rebuild it in the schema's default world. Otherwise a Lua rule keeps
// reading in whatever world NewApp guessed.
func TestSetWorlds_RewiresValidatorWorld(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"page": {
				Faces:      map[string]metamodel.FaceDef{"draft": {}, "published": {}},
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
			"note": {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
		},
		Validations: []metamodel.ValidationRule{{
			Name:       "count-pages",
			EntityType: "note",
			Lua: `
				local pages = rela.list_entities("page")
				return { message = "pages=" .. #pages }
			`,
		}},
	}
	f := newFixture()
	f.AddNode(&entity.Entity{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "n"}})
	f.AddNode(&entity.Entity{ID: "PG-1", Type: "page", Face: "draft", Properties: map[string]any{"title": "d"}})
	app := newAppFromParts(nil, meta, f)

	// PG-1 has only a draft face. Each setWorlds call must change what the
	// rule sees, in both directions, so neither result can come from the
	// world NewApp guessed.
	for _, tc := range []struct {
		face entity.Face
		want string
	}{
		{"draft", "pages=1"},
		{"published", "pages=0"},
	} {
		app.setWorlds(defaultWorldStub{scope: store.NewWorldScope(map[string]store.TypeResolution{
			"page": {Chain: []entity.Face{tc.face}, Fallback: store.FallbackExclude},
		})})
		if got := ruleMessages(t, app, meta.Validations[0]); !strings.Contains(got, tc.want) {
			t.Errorf("default world serving %s: rule messages = %q, want %s", tc.face, got, tc.want)
		}
	}
}

// ruleMessages runs one rule through the App's validator and joins its
// violation messages. A script or load error fails the test, since it would
// otherwise read as "no violations".
func ruleMessages(t *testing.T, app *App, rule metamodel.ValidationRule) string {
	t.Helper()
	res, err := app.validator.CheckRuleFull(context.Background(), rule)
	if err != nil {
		t.Fatalf("CheckRuleFull: %v", err)
	}
	if len(res.ScriptErrors) > 0 || len(res.LoadErrors) > 0 {
		t.Fatalf("rule failed to run: script errors %v, load errors %v", res.ScriptErrors, res.LoadErrors)
	}
	messages := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		messages = append(messages, v.Message)
	}
	return strings.Join(messages, "; ")
}
