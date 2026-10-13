package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// mentionsFaceBody cites a faced policy on both faces, a draft-only policy, a
// faceless feature and an id that does not exist.
const mentionsFaceBody = "See `POL-1`, `POL-2`, `FEAT-1` and `POL-404`."

// mentionsFaceApp is facedAppWith over world, with alice holding read and
// every request routed as alice. FEAT-SRC carries mentionsFaceBody, and
// POL-2 exists only as a draft.
func mentionsFaceApp(t *testing.T, world store.WorldScope, read []string) *App {
	t.Helper()
	meta := facedMeta(t)
	// facedMeta titles nothing, so every title would be the id and could
	// not tell the faces apart.
	for _, typ := range []string{"policy", "feature"} {
		def, ok := meta.Entities[typ]
		if !ok {
			t.Fatalf("facedMeta has no %s", typ)
		}
		def.DisplayProperty = "title"
		meta.Entities[typ] = def
	}
	app, _ := facedAppWith(t, meta, world, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: read}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})
	for _, e := range []*entity.Entity{
		{ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "ONLY A DRAFT"}},
		{ID: "FEAT-SRC", Type: "feature", Properties: map[string]any{"title": "source"}, Content: mentionsFaceBody},
	} {
		if err := app.store.CreateEntity(t.Context(), e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	return app
}

// mentionsFacePaths are the two routes that serve mentions: the view and the
// entity GET. They share collectMentions, so both must agree.
var mentionsFacePaths = []string{"/api/v1/_views/feature/FEAT-SRC", "/api/v1/features/FEAT-SRC"}

// routedMentions GETs path through the router and returns its mentions map.
func routedMentions(t *testing.T, app *App, path string) map[string]v1.Mention {
	t.Helper()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
	}
	var body struct {
		Mentions map[string]v1.Mention `json:"mentions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET %s: decode: %v", path, err)
	}
	return body.Mentions
}

// TestMentions_FacedTargetsResolveInTheRequestWorld pins that a mention of a
// faced entity resolves like any bare id on the read path: to the face the
// request's world serves among the faces the reader may read. A faced type
// has no implicit-face row, so it used to never resolve at all.
func TestMentions_FacedTargetsResolveInTheRequestWorld(t *testing.T) {
	const worldGrant = "world:published"
	for _, tc := range []struct {
		name  string
		world store.WorldScope
		read  []string
		// want maps each id to its expected title; an absent id has no mention.
		want map[string]string
	}{
		{
			name:  "ranked world, every face readable",
			world: rankedPolicyWorld(),
			read:  []string{"policy", "feature", worldGrant},
			want:  map[string]string{"POL-1": "PUBLISHED TEXT", "POL-2": "ONLY A DRAFT", "FEAT-1": "f"},
		},
		{
			name:  "published denied falls through to the readable draft",
			world: rankedPolicyWorld(),
			read:  []string{"policy@draft", "feature", worldGrant},
			want:  map[string]string{"POL-1": "DRAFT TEXT", "POL-2": "ONLY A DRAFT", "FEAT-1": "f"},
		},
		{
			name:  "no readable face, no mention",
			world: rankedPolicyWorld(),
			read:  []string{"feature", worldGrant},
			want:  map[string]string{"FEAT-1": "f"},
		},
		{
			name:  "a world that excludes drafts serves no draft-only mention",
			world: policyPublishedScope(),
			read:  []string{"policy", "feature", worldGrant},
			want:  map[string]string{"POL-1": "PUBLISHED TEXT", "FEAT-1": "f"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mentionsFaceApp(t, tc.world, tc.read)
			for _, path := range mentionsFacePaths {
				assertMentions(t, path, routedMentions(t, app, path), tc.want)
			}
		})
	}
}

// TestMentions_FacedTargetRedactsHiddenTitle pins BUG-R9EHKV for a faced
// target: the served face is redacted before its title is derived, so a
// hidden display property falls back to the id.
func TestMentions_FacedTargetRedactsHiddenTitle(t *testing.T) {
	app := mentionsFaceApp(t, rankedPolicyWorld(), []string{"policy", "feature", "world:published"})
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}})

	for _, path := range mentionsFacePaths {
		got := routedMentions(t, app, path)
		m, ok := got["POL-1"]
		if !ok {
			t.Fatalf("GET %s: no mention for POL-1: %+v", path, got)
		}
		if m.Title != "POL-1" || m.Type != "policy" {
			t.Errorf("GET %s: POL-1 mention = %+v, want the id as title", path, m)
		}
	}
}

// namedWorlds is a WorldLookup over several worlds, so a request can name one
// other than the configured default.
type namedWorlds map[string]store.WorldScope

func (n namedWorlds) Lookup(name string) (store.WorldScope, bool) {
	s, ok := n[name]
	return s, ok
}

// TestMentions_ResolveInTheWorldTheRequestNames pins that mentions follow the
// request's `?world=`, not the configured default world: the same body cites
// POL-1 as published in the default world and as draft in a draft-first one.
func TestMentions_ResolveInTheWorldTheRequestNames(t *testing.T) {
	// facedMeta declares no worlds, so the default world is the generated
	// `default`; here it ranks published first.
	worlds := namedWorlds{
		metamodel.DefaultWorldName: rankedPolicyWorld(),
		"drafts": store.NewWorldScope(map[string]store.TypeResolution{
			"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
		}),
	}
	read := []string{"policy", "feature", "world:drafts"}
	app := mentionsFaceApp(t, rankedPolicyWorld(), read)
	app.setWorlds(worlds)

	for _, tc := range []struct {
		query string
		want  map[string]string
	}{
		{"", map[string]string{"POL-1": "PUBLISHED TEXT", "POL-2": "ONLY A DRAFT", "FEAT-1": "f"}},
		{"?world=drafts", map[string]string{"POL-1": "DRAFT TEXT", "POL-2": "ONLY A DRAFT", "FEAT-1": "f"}},
	} {
		for _, path := range mentionsFacePaths {
			assertMentions(t, path+tc.query, routedMentions(t, app, path+tc.query), tc.want)
		}
	}
}

// TestMentions_DeniedWorldServesNoMention pins that a request naming a world
// the reader may not read gets no mention from it: the route refuses, or the
// response carries no mentions.
func TestMentions_DeniedWorldServesNoMention(t *testing.T) {
	app := mentionsFaceApp(t, rankedPolicyWorld(), []string{"policy", "feature"})
	app.setWorlds(namedWorlds{
		metamodel.DefaultWorldName: rankedPolicyWorld(),
		"drafts":                   rankedPolicyWorld(),
	})
	for _, path := range mentionsFacePaths {
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?world=drafts", http.NoBody))
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), `"mentions"`) {
			t.Errorf("GET %s?world=drafts in a denied world served mentions: %s", path, rec.Body)
		}
	}
}

// assertMentions checks that got holds exactly the ids of want, each with the
// wanted title and its type's display. Every id in these tests is a policy
// except the FEAT- ones.
func assertMentions(t *testing.T, path string, got map[string]v1.Mention, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("GET %s: mentions = %+v, want ids %v", path, got, want)
	}
	for id, title := range want {
		m, ok := got[id]
		if !ok {
			t.Errorf("GET %s: no mention for %s: %+v", path, id, got)
			continue
		}
		wantType := "policy"
		if strings.HasPrefix(id, "FEAT-") {
			wantType = "feature"
		}
		if m.Title != title || m.Type != wantType {
			t.Errorf("GET %s: %s = %+v, want title %q type %q", path, id, m, title, wantType)
		}
	}
}
