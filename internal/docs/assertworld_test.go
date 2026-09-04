package docs

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// worldFixtureMeta is an ISMS-shaped schema: a policy with a draft (default)
// and a published face, and two worlds over them — `published` excludes what
// has no published face, `preview` falls back to the default instead.
//
// That pair is the whole point of worlds in one schema: the SAME graph answers
// differently depending on which view you ask, and `otherwise:` is what decides
// whether an unpublished draft is absent or substituted.
func worldFixtureMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {
				Label:    "Policy",
				BareFace: "draft",
				Faces: map[string]metamodel.FaceDef{
					"draft":     {Label: "Draft"},
					"published": {Label: "Published"},
				},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			// A type with NO faces: it has exactly one state, present in every
			// world. Worlds are opt-in per type.
			"control": {
				Label:      "Control",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
			},
		},
		Worlds: map[string]metamodel.WorldDef{
			"published": {Select: []string{"published"}, Otherwise: metamodel.OtherwiseExclude},
			"preview":   {Select: []string{"draft", "published"}, Otherwise: metamodel.OtherwiseDefault},
		},
	}
	m.InitAliases()
	return m
}

func TestShowsWorld(t *testing.T) {
	// POL-1 has both faces; POL-2 is a draft only.
	seed := `create("policy", { id = "POL-1", title = "Access Control" })
face("policy", "POL-1", "published", { title = "Access Control" })
create("policy", { id = "POL-2", title = "Unpublished draft" })
`
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "the default world holds every entity at its default face",
			body: `shows{ type = "policy", exactly = { "POL-1", "POL-2" } }`,
		},
		{
			name: "a filtering world excludes what has no face there",
			body: `shows{ type = "policy", world = "published", exactly = { "POL-1" } }`,
		},
		{
			name: "absent= states the publication bit directly",
			body: `shows{ type = "policy", world = "published", absent = { "POL-2" } }`,
		},
		{
			name: "otherwise:default substitutes rather than excluding",
			body: `shows{ type = "policy", world = "preview", exactly = { "POL-1", "POL-2" } }`,
		},
		{
			// The claim that would pass vacuously if the world were ignored.
			name:    "a wrong world claim fails and names the world",
			body:    `shows{ type = "policy", world = "published", contains = { "POL-2" } }`,
			wantErr: "policy in the published world",
		},
		{
			name:    "an undeclared world is refused, not read as empty",
			body:    `shows{ type = "policy", world = "publsihed", exactly = {} }`,
			wantErr: `no world named "publsihed" is declared`,
		},
		{
			name: "the reserved default name is accepted",
			body: `shows{ type = "policy", world = "default", exactly = { "POL-1", "POL-2" } }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" + seed + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: worldFixtureMeta(t)})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want failure containing %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error does not contain %q:\n%v", tc.wantErr, err)
			}
		})
	}
}

// TestFaceVerb covers seeding a non-default face, and the mistakes that would
// make a world assertion pass for the wrong reason.
func TestFaceVerb(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "seeding a declared face is fine",
			body: `create("policy", { id = "POL-1", title = "T" })
face("policy", "POL-1", "published", { title = "T" })
shows{ type = "policy", world = "published", exactly = { "POL-1" } }`,
		},
		{
			// An undeclared coordinate answers to no world, so every world
			// claim about it would pass for the wrong reason.
			name: "an undeclared coordinate is refused, and the error lists what IS declared",
			body: `create("policy", { id = "POL-1", title = "T" })
face("policy", "POL-1", "approved", { title = "T" })`,
			wantErr: `"approved" is not a declared face of "policy"`,
		},
		{
			name:    "an unknown entity type is refused",
			body:    `face("polciy", "POL-1", "published", {})`,
			wantErr: "no such entity type",
		},
		{
			// A `bare_face` face is STORED under the zero coordinate.
			// Seeding it by name must land on the entity's own row, not mint a
			// second one — otherwise the default world would show duplicates.
			// Seeded ONLY through face(): the declared name maps to the zero
			// coordinate, so the entity's own row comes into existence and
			// the default world shows exactly one POL-1.
			name: "seeding the default face by name lands on the entity's own row",
			body: `face("policy", "POL-1", "draft", { title = "T" })
shows{ type = "policy", exactly = { "POL-1" } }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: worldFixtureMeta(t)})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want failure containing %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error does not contain %q:\n%v", tc.wantErr, err)
			}
		})
	}
}
