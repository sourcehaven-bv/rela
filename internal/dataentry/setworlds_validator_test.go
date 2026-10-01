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
// validator: NewApp builds it before the worlds exist, so SetWorlds must
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

	// PG-1 has only a draft face. Each SetWorlds call must change what the
	// rule sees, in both directions, so neither result can come from the
	// world NewApp guessed.
	for _, tc := range []struct {
		face entity.Face
		want string
	}{
		{"draft", "pages=1"},
		{"published", "pages=0"},
	} {
		app.SetWorlds(defaultWorldStub{scope: store.NewWorldScope(map[string]store.TypeResolution{
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

// TestBrowsingDefaultWorld pins that schema.yaml's default_world reaches the
// HTTP landing world once app.default_world is removed, so moving the key
// into the schema changes nothing a browser sees.
func TestBrowsingDefaultWorld(t *testing.T) {
	t.Parallel()
	withAlias := func(w string) *Config {
		c := &Config{}
		c.App.DefaultWorld = w
		return c
	}
	tests := []struct {
		name  string
		state *Schema
		want  string
	}{
		{"no state", nil, ""},
		{"nothing set", &Schema{Cfg: &Config{}, Meta: &metamodel.Metamodel{}}, ""},
		{"schema key", &Schema{Cfg: &Config{}, Meta: &metamodel.Metamodel{DefaultWorld: "published"}}, "published"},
		{"alias only", &Schema{Cfg: withAlias("published"), Meta: &metamodel.Metamodel{}}, "published"},
		{"no config", &Schema{Meta: &metamodel.Metamodel{DefaultWorld: "published"}}, "published"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := browsingDefaultWorld(tc.state); got != tc.want {
				t.Errorf("browsingDefaultWorld = %q, want %q", got, tc.want)
			}
		})
	}
}
