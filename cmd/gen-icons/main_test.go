package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig/icondefs"
)

// repoRoot walks up from the test's working directory to the module root.
//
// The generator takes an explicit -root precisely so it never guesses; the test
// has to find it some way, and go.mod is the unambiguous marker.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the test directory")
		}
		dir = parent
	}
}

// TestGeneratedFilesAreUpToDate is the drift gate.
//
// Every artifact derived from the icon table — the Go allowlist, the SPA
// registry, the documentation table — must match what the generator produces
// right now. This fails both when someone adds an icon without regenerating and
// when someone hand-edits an output, which are the same bug seen from two ends:
// the committed file stops being a function of the canonical table.
func TestGeneratedFilesAreUpToDate(t *testing.T) {
	if err := run(repoRoot(t), true); err != nil {
		t.Fatalf("%v", err)
	}
}

// TestDocsExamplesUseValidNames checks that every `icon:` the guide DEMONSTRATES
// is a name the server would actually accept.
//
// The table is generated and therefore trustworthy; the YAML examples around it
// are hand-written and are not. That asymmetry is a trap — a generated table
// makes the whole page read as machine-checked, so a reader trusts the example
// MORE than before. One shipped example (`icon: progress`) was invalid against
// even the old 16-name list, and survived a rewrite of the surrounding prose.
//
// Copy-pasting the first icon example in the guide should not stop the server
// from booting.
func TestDocsExamplesUseValidNames(t *testing.T) {
	valid := map[string]bool{icondefs.NoIcon: true}
	for _, d := range icondefs.All() {
		valid[d.Name] = true
	}

	// Matches a YAML `icon: <name>` line, with an optional trailing comment.
	iconLine := regexp.MustCompile(`(?m)^\s*icon:\s*([a-z][a-z0-9-]*)\s*(?:#.*)?$`)

	for _, path := range []string{docsOut, "docs/data-entry.md"} {
		src, err := os.ReadFile(filepath.Join(repoRoot(t), path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, m := range iconLine.FindAllStringSubmatch(string(src), -1) {
			if !valid[m[1]] {
				t.Errorf("%s documents `icon: %s`, which config validation rejects "+
					"— an author copy-pasting the example cannot start the server",
					path, m[1])
			}
		}
	}
}

// TestValidate covers the guardrails on the canonical table itself.
//
// Each case is a mistake that is easy to make while editing a two-hundred-line
// literal and hard to spot afterwards, because the damage shows up in a
// generated file nobody reads.
// validTable builds a minimal table that actually passes every check, by
// carrying the names referenced outside author config.
//
// Without this the "valid" case could only assert "it failed for the reason I
// expected", which is not an assertion — and it shared its input with the
// chrome-missing case, so two subtests asserted opposite things about identical
// data and both passed.
func validTable() []icondefs.IconDef {
	defs := []icondefs.IconDef{{Name: "thing", Lucide: "Box", Category: "Misc", Desc: "A thing"}}
	for _, d := range icondefs.All() {
		if icondefs.IsChrome(d.Name) {
			defs = append(defs, d)
		}
	}
	return defs
}

func TestValidate(t *testing.T) {
	ok := icondefs.IconDef{Name: "thing", Lucide: "Box", Category: "Misc", Desc: "A thing"}

	tests := []struct {
		name string
		defs []icondefs.IconDef
		want string
	}{
		{"a valid table", validTable(), ""},
		{"empty table", nil, "empty"},
		{
			// Silently loses an entry when the slice becomes a map, so the
			// count looks right and one icon is simply missing.
			"duplicate name",
			append(validTable(), ok),
			"defined twice",
		},
		{
			// Would make `icon: none` render a glyph, inverting the one thing
			// the name promises.
			"the reserved no-icon name",
			[]icondefs.IconDef{{Name: icondefs.NoIcon, Lucide: "Box", Category: "Misc", Desc: "x"}},
			"reserved",
		},
		{"missing Lucide component", []icondefs.IconDef{{Name: "x", Category: "Misc", Desc: "d"}}, "no Lucide"},
		{"missing category", []icondefs.IconDef{{Name: "x", Lucide: "Box", Desc: "d"}}, "no Category"},
		{"missing description", []icondefs.IconDef{{Name: "x", Lucide: "Box", Category: "Misc"}}, "no Desc"},
		{
			// The SPA imports some of these by name and the server emits the
			// others as literals, so losing one blanks a sidebar glyph — or
			// makes every entry of a kind render the fallback — with no build
			// or test failure anywhere.
			"a name referenced outside author config is missing",
			[]icondefs.IconDef{ok},
			"referenced outside author config",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.defs)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("expected a valid table, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// TestValidate_RealTableIsValid runs the guardrails against the committed set.
func TestValidate_RealTableIsValid(t *testing.T) {
	if err := Validate(icondefs.All()); err != nil {
		t.Fatalf("the committed icon table is invalid: %v", err)
	}
}

// TestRenderTS_AliasesEveryImport pins the collision fix.
//
// Lucide exports a component named `Component`, which collides with Vue's
// `Component` TYPE the moment an icon uses it — and the resulting error ("only
// refers to a type, but is being used as a value") points nowhere near the
// cause. Prefixing makes the collision impossible instead of unlikely.
func TestRenderTS_AliasesEveryImport(t *testing.T) {
	ts := RenderTS(icondefs.All())

	if !strings.Contains(ts, "Component as LuComponent") {
		t.Error("Lucide imports must be alias-prefixed, or an icon named `component` " +
			"shadows Vue's Component type")
	}
	if strings.Contains(ts, "\n  Component,\n") {
		t.Error("found a bare `Component` import; every Lucide import must be aliased")
	}
	if !strings.Contains(ts, "import type { Component } from 'vue'") {
		t.Error("the Vue Component type import went missing")
	}
}

// TestRenderTS_ExportsChromeIconsByName pins the reason chrome icons are named
// exports rather than ICONS lookups.
func TestRenderTS_ExportsChromeIconsByName(t *testing.T) {
	ts := RenderTS(icondefs.All())
	for _, want := range []string{
		"export const IconSearch", "export const IconWarning",
		"export const IconApps", "export const IconSettings",
		"export const IconSun", "export const IconMoon",
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("missing %q — the sidebar imports it, and reaching through "+
				"ICONS instead would let a rename blank the glyph silently", want)
		}
	}

	// The server-derived names must NOT get an export: nothing in the SPA
	// imports them (they arrive as strings on the wire), so an export would be
	// dead code that reads as protection while protecting nothing. Their real
	// guard is TestDerivedIconsAreValidNames on the Go side.
	for _, unwanted := range []string{"export const IconDashboard", "export const IconList"} {
		if strings.Contains(ts, unwanted) {
			t.Errorf("unexpected %q — a server-derived name needs no SPA import, and "+
				"exporting one implies a coupling that does not exist", unwanted)
		}
	}
}

// TestRenderGo_ExcludesNoIcon keeps the reserved name out of the allowlist.
func TestRenderGo_ExcludesNoIcon(t *testing.T) {
	src, err := RenderGo(icondefs.All())
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}
	if strings.Contains(src, `"`+icondefs.NoIcon+`": true`) {
		t.Errorf("%q must not appear in ValidIconNames; validateIconName accepts it "+
			"separately, and listing it here would imply it renders something", icondefs.NoIcon)
	}
}

// TestReplaceRegion covers the docs-region splice.
func TestReplaceRegion(t *testing.T) {
	const doc = "before\n" + docsBegin + "\nold\n" + docsEnd + "\nafter\n"

	got, err := ReplaceRegion(doc, "\nnew\n")
	if err != nil {
		t.Fatalf("ReplaceRegion: %v", err)
	}
	want := "before\n" + docsBegin + "\nnew\n" + docsEnd + "\nafter\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Missing markers must fail rather than append: a second region would leave
	// the docs with two tables, one permanently stale.
	if _, err := ReplaceRegion("no markers here", "x"); err == nil {
		t.Error("a document without markers must be an error, not an append")
	}
	if _, err := ReplaceRegion(docsBegin+" unterminated", "x"); err == nil {
		t.Error("a document with no closing marker must be an error")
	}
}

// TestRenderDocs_CoversEveryName makes the published table complete by
// construction — it is the only place an author discovers these names.
func TestRenderDocs_CoversEveryName(t *testing.T) {
	defs := icondefs.All()
	docs := RenderDocs(defs)
	for _, d := range defs {
		if !strings.Contains(docs, "| `"+d.Name+"` |") {
			t.Errorf("icon %q is missing from the documentation table", d.Name)
		}
		if !strings.Contains(docs, d.Desc) {
			t.Errorf("icon %q has no description in the table", d.Name)
		}
	}
	if !strings.Contains(docs, icondefs.NoIcon) {
		t.Errorf("the table should mention %q, which is valid but draws nothing", icondefs.NoIcon)
	}
}
