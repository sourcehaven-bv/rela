package docs

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// worldGrantPolicy mirrors the shipped worlds prototype: an editor who reads
// every face and every world, and a reader narrowed on BOTH axes — one face of
// a policy, and only the published world.
//
// Both narrowings are load-bearing. A fixture that narrowed only one would let
// a regression in the other render as a full ✓ and still pass.
func worldGrantPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"*", "world:published", "world:preview"}},
			"reader": {Read: []string{"policy@published", "world:published"}},
		},
		Assignments: map[string]string{"ed": "editor", "pub": "reader"},
	}
}

// A face-scoped read grant must render DIFFERENTLY from a bare one.
//
// Both make acl.GrantsRead true, so a matrix built on that predicate alone
// rendered `✓` for each and stated that a reader who sees only published
// policies has the same read access as an editor who sees every draft. The
// assertion is the DIFFERENCE, not the presence of the annotation: a
// regression that dropped the suffix entirely would still satisfy a test that
// only looked for "✓".
func TestRolesMatrix_FaceScopedReadRendersDistinctly(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nroles_matrix{type=\"policy\"}\n```\n",
		Options{Meta: worldFixtureMeta(t), Policy: worldGrantPolicy()})

	readRow := rowFor(t, out, "read")
	if !strings.Contains(readRow, "@published") {
		t.Errorf("the reader's face-scoped grant must be visible in the read row, got:\n%s", readRow)
	}
	// The editor's cell is a bare ✓ and the reader's is annotated, so the two
	// cells must not be equal. Counting "✓" alone would pass on a table that
	// annotated both.
	editorCell, readerCell := cells(readRow)[2], cells(readRow)[3]
	if editorCell == readerCell {
		t.Errorf("a bare read grant and a face-scoped one rendered identically (%q) — "+
			"the distinction the face grant exists to draw is invisible", editorCell)
	}
}

// A bare grant must NOT be annotated, or the suffix stops meaning anything.
func TestRolesMatrix_FullReadGrantIsUnannotated(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nroles_matrix{type=\"risico\"}\n```\n",
		Options{Meta: fixtureMeta(t), Policy: fixturePolicy()})
	if strings.Contains(out, "@") {
		t.Errorf("a role reading every face must render a plain ✓, got:\n%s", out)
	}
}

// The `everyone` role is folded into every cell, so a role narrowed to one face
// still reads every face when everyone grants the type outright. Annotating it
// would UNDERSTATE effective access — the wrong direction for a security doc.
func TestRolesMatrix_EveryoneWideningSuppressesTheFaceSuffix(t *testing.T) {
	t.Parallel()
	policy := &acl.Policy{Roles: map[string]acl.RoleDef{
		"reader":         {Read: []string{"policy@published"}},
		acl.EveryoneRole: {Read: []string{"policy"}},
	}}
	out := build(t, "```rela\nroles_matrix{type=\"policy\"}\n```\n",
		Options{Meta: worldFixtureMeta(t), Policy: policy})
	if strings.Contains(out, "@published") {
		t.Errorf("everyone grants every face, so the cell must not claim a narrower one:\n%s", out)
	}
}

// The worlds table is the point of the split: worlds are a navigation fact, so
// they get their own table rather than a column in the verb matrix.
func TestWorldsMatrix_RendersRoleByWorld(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nworlds_matrix{}\n```\n",
		Options{Meta: worldFixtureMeta(t), Policy: worldGrantPolicy()})

	for _, want := range []string{"| Role |", "`default`", "`preview`", "`published`"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// The reader holds `world:published` but NOT `world:preview`; the editor
	// holds both. That difference is the whole claim the table makes.
	editorRow, readerRow := rowFor(t, out, "editor"), rowFor(t, out, "reader")
	if strings.Count(editorRow, "✓") <= strings.Count(readerRow, "✓") {
		t.Errorf("the editor reads more worlds than the reader, but the table does not say so:\n"+
			"  editor: %s\n  reader: %s", editorRow, readerRow)
	}
}

// The default world is the ABSENCE of a grant (acl.RoleDef.Worlds is empty for
// a role naming no `world:` entry). Rendering that as an empty row would say
// the role may ask for nothing — the exact inverse of the truth.
func TestWorldsMatrix_DefaultWorldIsAlwaysGranted(t *testing.T) {
	t.Parallel()
	policy := &acl.Policy{Roles: map[string]acl.RoleDef{
		"plain": {Read: []string{"*"}}, // no world: entry at all
	}}
	out := build(t, "```rela\nworlds_matrix{}\n```\n",
		Options{Meta: worldFixtureMeta(t), Policy: policy})
	if !strings.Contains(rowFor(t, out, "plain"), "✓") {
		t.Errorf("a role with no world: grant still reads the default world, got:\n%s", out)
	}
}

// A project with no worlds would render a table whose only column is the
// default world — a table that states nothing, which is the vacuous shape this
// package refuses everywhere else.
func TestWorldsMatrix_RefusesAWorldlessProject(t *testing.T) {
	t.Parallel()
	_, err := Build(context.Background(), "```rela\nworlds_matrix{}\n```\n",
		Options{Meta: fixtureMeta(t), Policy: fixturePolicy()})
	if err == nil {
		t.Fatal("a worldless project must refuse the table rather than render an empty claim")
	}
	if !strings.Contains(err.Error(), "declares no worlds") {
		t.Errorf("failed for the wrong reason: %v", err)
	}
}

// rowFor returns the first markdown table row with want as one of its cells.
//
// Cell equality, not substring: "read" is a substring of the "reader" COLUMN
// HEADER, so a contains-match returns the header row and the assertion below
// then reports a missing annotation that is actually present two lines down.
func rowFor(t *testing.T, out, want string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if slices.Contains(cells(line), want) {
			return line
		}
	}
	t.Fatalf("no table row with a %q cell in:\n%s", want, out)
	return ""
}

// cells splits a markdown table row into its trimmed cells.
func cells(row string) []string {
	parts := strings.Split(strings.Trim(row, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
