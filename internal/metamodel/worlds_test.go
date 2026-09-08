package metamodel

import (
	"strings"
	"testing"
)

// worldsSchema is a schema fixture with two faced types and one
// faceless one, plus whatever `worlds:` block the test supplies.
func worldsSchema(worldsBlock string) string {
	return `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
    bare_face: draft
    faces:
      draft: {}
      published: {}
  policy:
    label: Policy
    id_prefix: POL
    properties:
      title: {type: string}
    bare_face: draft
    faces:
      draft: {}
      review: {}
      published: {}
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
` + worldsBlock
}

// TestWorlds_ParseAndDeclare pins the happy-path shape of the `faces:`
// and `worlds:` declarations (TKT-WAV8XP, design doc §4.1), including the
// select-as-scalar sugar and the per-type override.
func TestWorlds_ParseAndDeclare(t *testing.T) {
	m, err := Parse([]byte(worldsSchema(`worlds:
  published:
    select: published
    otherwise: exclude
  editorial:
    select: [review, published]
    overrides:
      page: draft
    otherwise: default
    edits: review
`)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := len(m.Entities["page"].Faces); got != 2 {
		t.Errorf("page faces = %d, want 2", got)
	}
	// Which face the bare id addresses is ONE fact on the type, so there is a
	// single value to assert rather than a flag per face that could disagree.
	if got := m.Entities["page"].BareFace; got != "draft" {
		t.Errorf("page bare_face = %q, want %q", got, "draft")
	}
	if got := len(m.Entities["ticket"].Faces); got != 0 {
		t.Errorf("ticket declares no faces, got %d", got)
	}

	// Scalar `select:` is sugar for a one-element chain.
	pub := m.Worlds["published"]
	if want := []string{"published"}; len(pub.Select) != 1 || pub.Select[0] != want[0] {
		t.Errorf("published.Select = %v, want %v", pub.Select, want)
	}
	if pub.Otherwise != OtherwiseExclude {
		t.Errorf("published.Otherwise = %q, want %q", pub.Otherwise, OtherwiseExclude)
	}

	ed := m.Worlds["editorial"]
	if len(ed.Select) != 2 || ed.Select[0] != "review" || ed.Select[1] != "published" {
		t.Errorf("editorial.Select = %v, want [review published]", ed.Select)
	}
	if ed.Edits != "review" {
		t.Errorf("editorial.Edits = %q, want review", ed.Edits)
	}

	// ChainFor: override wins for page, global chain for everyone else.
	if chain, ok := ed.ChainFor("page"); !ok || len(chain) != 1 || chain[0] != "draft" {
		t.Errorf("editorial.ChainFor(page) = %v, %v; want [draft], true", chain, ok)
	}
	if chain, ok := ed.ChainFor("policy"); !ok || len(chain) != 2 {
		t.Errorf("editorial.ChainFor(policy) = %v, %v; want the global chain", chain, ok)
	}
}

// TestWorlds_AbsentBlockIsUnchanged pins the compatibility guarantee that
// every other decision rests on: a metamodel with no `worlds:` and no
// `faces:` parses to empty declarations, so such a project behaves
// byte-identically to the pre-worlds system.
func TestWorlds_AbsentBlockIsUnchanged(t *testing.T) {
	m, err := Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Worlds != nil {
		t.Errorf("Worlds = %v, want nil for a schema with no worlds block", m.Worlds)
	}
	if m.Entities["ticket"].Faces != nil {
		t.Errorf("Faces = %v, want nil", m.Entities["ticket"].Faces)
	}
}

// TestWorlds_ValidationRejects covers every load-time refusal. The
// mandatory-`otherwise:` case is the load-bearing one: a silent fallback
// is the leak content states exist to prevent.
func TestWorlds_ValidationRejects(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		wantSubstr []string
	}{
		{
			name: "otherwise missing",
			schema: worldsSchema(`worlds:
  published:
    select: published
`),
			wantSubstr: []string{`world "published"`, "`otherwise:` is required", "exclude", "default"},
		},
		{
			name: "otherwise invalid value",
			schema: worldsSchema(`worlds:
  published:
    select: published
    otherwise: fallback
`),
			wantSubstr: []string{`world "published"`, "`otherwise:` is required", `"fallback"`},
		},
		{
			name: "world named default",
			schema: worldsSchema(`worlds:
  default:
    select: published
    otherwise: exclude
`),
			wantSubstr: []string{`world "default"`, "reserved", "implicit and total"},
		},
		{
			// A world named `Default` is not the reserved lowercase one, but
			// letting both exist is a confusion with no upside.
			name: "world named Default differs only in case",
			schema: worldsSchema(`worlds:
  Default:
    select: published
    otherwise: exclude
`),
			wantSubstr: []string{`world "Default"`, "reserved", "implicit and total"},
		},
		{
			// Forgetting `select:` (or slipping its indentation) resolves
			// every faced entity through `otherwise:` alone — a world
			// that shows nothing, failing safe but with no diagnostic.
			name: "world selects nothing at all",
			schema: worldsSchema(`worlds:
  oops:
    otherwise: exclude
`),
			wantSubstr: []string{`world "oops"`, "neither `select:` nor `overrides:`", "shows nothing"},
		},
		{
			name: "select is an explicitly empty list",
			schema: worldsSchema(`worlds:
  oops:
    select: []
    otherwise: exclude
`),
			wantSubstr: []string{`world "oops"`, "neither `select:` nor `overrides:`"},
		},
		{
			name: "override chain is empty",
			schema: worldsSchema(`worlds:
  published:
    select: published
    overrides:
      page: []
    otherwise: exclude
`),
			wantSubstr: []string{`world "published"`, `"page"`, "empty chain"},
		},
		{
			// An empty world name would make a lookup with an unpopulated
			// name succeed and return a real, non-default world.
			name: "world name is empty",
			schema: worldsSchema(`worlds:
  "":
    select: published
    otherwise: exclude
`),
			wantSubstr: []string{"invalid name", "must not be empty"},
		},
		{
			name: "select names an undeclared face",
			schema: worldsSchema(`worlds:
  ghost:
    select: archived
    otherwise: exclude
`),
			wantSubstr: []string{`world "ghost"`, `face "archived"`, "no entity type declares"},
		},
		{
			name: "override names an unknown entity type",
			schema: worldsSchema(`worlds:
  published:
    select: published
    overrides:
      nonesuch: draft
    otherwise: exclude
`),
			wantSubstr: []string{`world "published"`, "unknown entity type", `"nonesuch"`},
		},
		{
			name: "override names a faceless type",
			schema: worldsSchema(`worlds:
  published:
    select: published
    overrides:
      ticket: draft
    otherwise: exclude
`),
			wantSubstr: []string{`world "published"`, `"ticket"`, "declares no faces"},
		},
		{
			name: "override selects a face the type lacks",
			schema: worldsSchema(`worlds:
  published:
    select: published
    overrides:
      page: review
    otherwise: exclude
`),
			wantSubstr: []string{`world "published"`, `face "review"`, `"page"`, "declares"},
		},
		{
			name: "edits names an undeclared face",
			schema: worldsSchema(`worlds:
  editorial:
    select: published
    otherwise: default
    edits: staging
`),
			wantSubstr: []string{`world "editorial"`, "`edits:`", `"staging"`},
		},
		{
			// Two faces can no longer BOTH claim the bare id — `bare_face` is
			// one key on the type, so the old "at most one" check has no case
			// left to catch. What can still go wrong is naming a face the type
			// does not declare, which would leave the bare-id row unnamed while
			// the intended face became a separate suffixed row.
			name: "bare_face names an undeclared face",
			schema: `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
    bare_face: drfat
    faces:
      draft: {}
      published: {}
`,
			wantSubstr: []string{`entity "page"`, "bare_face: drfat", "names no declared face", "draft, published"},
		},
		{
			name: "bare_face on a type declaring no faces",
			schema: `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    bare_face: draft
    properties:
      title: {type: string}
`,
			wantSubstr: []string{`entity "page"`, "declares no `faces:`"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.schema))
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			for _, want := range tc.wantSubstr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}

// TestWorlds_SelectMayMissSomeTypes pins that a world's global chain
// naming a face only SOME types declare is legal — that is resolution
// rule 3, which `otherwise:` answers. Rejecting it would make any mixed
// schema unrepresentable.
func TestWorlds_SelectMayMissSomeTypes(t *testing.T) {
	// `review` is declared by policy but not by page; both coexist.
	if _, err := Parse([]byte(worldsSchema(`worlds:
  editorial:
    select: [review, published]
    otherwise: default
`))); err != nil {
		t.Fatalf("a chain some types cannot satisfy must be legal: %v", err)
	}
}

// TestWorlds_UnknownKeyBeforeUpgrade documents the forward-compat
// behavior: an older binary meeting a `worlds:` block reports it as an
// unknown key rather than silently ignoring it. Loud is correct — the
// same stance the face codec takes on multi-axis coordinates.
func TestWorlds_UnknownKeyIsRecognizedNow(t *testing.T) {
	if !validTopLevelKeys["worlds"] {
		t.Fatal("`worlds` must be a recognized top-level key")
	}
}

func TestOtherwise_IsValid(t *testing.T) {
	tests := []struct {
		in   Otherwise
		want bool
	}{
		{OtherwiseExclude, true},
		{OtherwiseDefault, true},
		{OtherwiseUnset, false}, // the zero value is deliberately invalid
		{Otherwise("Exclude"), false},
		{Otherwise("fallback"), false},
	}
	for _, tc := range tests {
		if got := tc.in.IsValid(); got != tc.want {
			t.Errorf("Otherwise(%q).IsValid() = %v, want %v", string(tc.in), got, tc.want)
		}
	}
}
