package docs

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// linkFaceMeta adds a CONTENT-scoped relation to the world fixture, so an edge
// belongs to one face of its source rather than to the entity.
func linkFaceMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m := worldFixtureMeta(t)
	m.Relations = map[string]metamodel.RelationDef{
		"see-also": {
			From:  []string{"policy"},
			To:    []string{"policy"},
			Scope: metamodel.ScopeContent,
		},
	}
	m.InitAliases()
	return m
}

// link()'s optional 4th argument names the SOURCE FACE an edge hangs off.
//
// It was lost once already: a merge took a simpler three-argument luaLink, so
// every content-scoped edge a manual seeded landed on the bare face — where a
// world-scoped read cannot see it, because edge lookup is face-exact with no
// fallback. The manual then described a link the software could not show
// (QA F-6a). These drive the real seeding path rather than reading the source.
func TestLuaLink_SeedsTheNamedFace(t *testing.T) {
	// POL-1 has both faces; the edge is seeded on the PUBLISHED one.
	src := "```rela\n" +
		`create("policy", { id = "POL-1", title = "A" })
face("policy", "POL-1", "published", { title = "A" })
create("policy", { id = "POL-2", title = "B" })
link("POL-1", "see-also", "POL-2", "published")
` + "```\n"

	if _, err := Build(context.Background(), src, Options{Meta: linkFaceMeta(t)}); err != nil {
		t.Fatalf("seeding an edge on a declared face must succeed: %v", err)
	}
}

// An undeclared face fails loudly. A silently-bare edge would be invisible to
// every world-scoped read — the failure mode this argument exists to prevent.
func TestLuaLink_UndeclaredFaceFailsLoudly(t *testing.T) {
	src := "```rela\n" +
		`create("policy", { id = "POL-1", title = "A" })
create("policy", { id = "POL-2", title = "B" })
link("POL-1", "see-also", "POL-2", "nl")
` + "```\n"

	_, err := Build(context.Background(), src, Options{Meta: linkFaceMeta(t)})
	if err == nil {
		t.Fatal("an undeclared face must fail rather than seeding a bare-tailed edge")
	}
	if !strings.Contains(err.Error(), "is not a declared face of") {
		t.Errorf("the error must name the mistake; got: %v", err)
	}
}
