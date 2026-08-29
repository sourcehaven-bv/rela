// Command gen-icons generates every artifact derived from the canonical icon
// table in internal/dataentryconfig/icondefs.
//
// Run it with `just generate-icons`. That recipe is the ONE supported invocation
// path: it passes an explicit -root, so the tool never has to guess where the
// repository is from its working directory. (A `go:generate` directive would run
// with a different CWD than a `just` recipe, and the two would silently write to
// different places.)
//
// Outputs:
//
//	internal/dataentryconfig/icons_gen.go            ValidIconNames
//	frontend/src/utils/iconRegistry.generated.ts     imports, ICONS, chrome exports
//	docs-project/entities/guides/GUIDE-data-entry.md the documentation table
//
// The docs target is the guide ENTITY, not docs/data-entry.md — that file is
// itself generated from the entity by `just docs`, so writing to it directly
// would be reverted on the next docs run. The two generators chain; ordering is
// enforced by the justfile.
//
// With -check the tool writes nothing and exits non-zero if any output is stale.
// That is the CI drift gate.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig/icondefs"
)

// Output paths, relative to the repository root.
const (
	goOut   = "internal/dataentryconfig/icons_gen.go"
	tsOut   = "frontend/src/utils/iconRegistry.generated.ts"
	docsOut = "docs-project/entities/guides/GUIDE-data-entry.md"
)

// Markers delimiting the generated region inside the docs guide. The rest of the
// file is hand-written prose, so the generator only ever replaces what is
// between these two lines.
const (
	docsBegin = "<!-- BEGIN generated: icons -->"
	docsEnd   = "<!-- END generated: icons -->"
)

func main() { // coverage-ignore: generator entry point, exercised via genicons_test.go
	root := flag.String("root", "", "repository root (required)")
	check := flag.Bool("check", false, "verify outputs are up to date; write nothing")
	flag.Parse()

	if *root == "" {
		fail(errors.New("-root is required (the justfile passes it; see the package doc)"))
	}
	if err := run(*root, *check); err != nil {
		fail(err)
	}
}

func fail(err error) { // coverage-ignore: trivial exit path
	fmt.Fprintln(os.Stderr, "gen-icons:", err)
	os.Exit(1)
}

// run renders every artifact and either writes it or compares it.
func run(root string, check bool) error {
	defs := icondefs.All()
	if err := Validate(defs); err != nil {
		return err
	}

	goSrc, err := RenderGo(defs)
	if err != nil {
		return err
	}
	tsSrc := RenderTS(defs)

	docsPath := filepath.Join(root, docsOut)
	guide, err := os.ReadFile(docsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", docsOut, err)
	}
	guideSrc, err := ReplaceRegion(string(guide), RenderDocs(defs))
	if err != nil {
		return fmt.Errorf("%s: %w", docsOut, err)
	}

	outputs := []struct {
		path string
		body string
	}{
		{goOut, goSrc},
		{tsOut, tsSrc},
		{docsOut, guideSrc},
	}

	var stale []string
	for _, o := range outputs {
		full := filepath.Join(root, o.path)
		if check {
			existing, err := os.ReadFile(full)
			switch {
			case err == nil:
				if string(existing) != o.body {
					stale = append(stale, o.path)
				}
			case os.IsNotExist(err):
				stale = append(stale, o.path)
			default:
				// A permissions problem or a truncated read is NOT drift, and
				// telling the reader to regenerate would send them chasing a
				// bug they do not have.
				return fmt.Errorf("read %s: %w", o.path, err)
			}
			continue
		}
		if err := os.WriteFile(full, []byte(o.body), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", o.path, err)
		}
	}

	if len(stale) > 0 {
		return fmt.Errorf("these generated files are out of date:\n  %s\n\n"+
			"The icon set is defined once, in internal/dataentryconfig/icondefs, and\n"+
			"everything else is derived from it. Run:\n\n    just generate-icons\n\n"+
			"and commit the result. Do not hand-edit a generated file — the next run\n"+
			"reverts it", strings.Join(stale, "\n  "))
	}
	if !check {
		fmt.Printf("gen-icons: wrote %d icons to %d files\n", len(defs), len(outputs))
	}
	return nil
}

// Validate rejects a table that would produce broken or ambiguous output.
//
// Every check here is a mistake that is cheap to make while editing a
// two-hundred-line table and expensive to notice afterwards: a duplicate name
// silently loses an entry when the slice becomes a map, and a missing
// description renders an empty docs cell.
//
// Two names MAY share a Lucide component (a synonym pair is legitimate), so
// that is deliberately not an error. No entry does today.
func Validate(defs []icondefs.IconDef) error {
	if len(defs) == 0 {
		return errors.New("icon table is empty")
	}

	seen := make(map[string]bool, len(defs))
	for _, d := range defs {
		switch {
		case d.Name == "":
			return fmt.Errorf("icon with Lucide %q has no Name", d.Lucide)
		case d.Lucide == "":
			return fmt.Errorf("icon %q has no Lucide component", d.Name)
		case d.Category == "":
			return fmt.Errorf("icon %q has no Category", d.Name)
		case d.Desc == "":
			return fmt.Errorf("icon %q has no Desc", d.Name)
		case seen[d.Name]:
			return fmt.Errorf("icon %q is defined twice", d.Name)
		case !isASCIIKebab(d.Name):
			// Load-bearing beyond tidiness: suggestIcon's edit distance
			// compares BYTES, so a multi-byte name would score by encoding
			// length rather than by characters.
			return fmt.Errorf("icon %q must be lowercase ASCII kebab-case", d.Name)
		}
		// The reserved opt-out must never become a real glyph: it is accepted by
		// config validation but has no component, and a table entry claiming
		// otherwise would make `icon: none` render something.
		if d.Name == icondefs.NoIcon {
			return fmt.Errorf("icon %q is the reserved no-icon name and cannot be a table entry", d.Name)
		}
		seen[d.Name] = true
	}

	// Every chrome name must exist, or the generated named export vanishes and
	// the SPA loses a glyph with nothing failing.
	var missing []string
	for _, name := range chromeList() {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("these names are referenced outside author config but the "+
			"table no longer defines them: %s\n(see icondefs.spaChromeNames and "+
			"icondefs.DerivedNames — renaming one blanks a sidebar glyph, or makes "+
			"every entry of a kind render the fallback, without failing any build)",
			strings.Join(missing, ", "))
	}
	return nil
}

// isASCIIKebab reports whether a name is lowercase ASCII letters, digits and
// hyphens.
func isASCIIKebab(name string) bool {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return name != ""
}

// chromeList returns the SPA-referenced names, sorted.
func chromeList() []string {
	var out []string
	for _, d := range icondefs.All() {
		if icondefs.IsChrome(d.Name) {
			out = append(out, d.Name)
		}
	}
	sort.Strings(out)
	return out
}

// sortedByName returns the table sorted by config name, for the artifacts whose
// order should not depend on the documentation grouping.
func sortedByName(defs []icondefs.IconDef) []icondefs.IconDef {
	out := append([]icondefs.IconDef(nil), defs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RenderGo produces the Go allowlist.
func RenderGo(defs []icondefs.IconDef) (string, error) {
	var b bytes.Buffer
	b.WriteString(`// Code generated by cmd/gen-icons. DO NOT EDIT.
// Source: internal/dataentryconfig/icondefs. Regenerate with ` + "`just generate-icons`" + `.

package dataentryconfig

// ValidIconNames is the allowlist of icon names an author may reference from
// data-entry.yaml.
//
// Derived from the canonical table so it cannot drift from what the SPA can
// actually render: a name the config accepts but the SPA lacks would validate
// and then show a fallback glyph with no error anywhere, and a name the SPA
// knows but the config rejects is a feature no author can reach.
//
// The reserved no-icon name is deliberately absent — it names no component. See
// validateIconName, which accepts it separately.
var ValidIconNames = map[string]bool{
`)
	for _, d := range sortedByName(defs) {
		fmt.Fprintf(&b, "\t%q: true,\n", d.Name)
	}
	b.WriteString("}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		return "", fmt.Errorf("format generated Go: %w", err)
	}
	return string(src), nil
}

// RenderTS produces the SPA registry: named imports, the ICONS map, and one
// named export per chrome icon.
func RenderTS(defs []icondefs.IconDef) string {
	byName := sortedByName(defs)

	// Import each component once, sorted, even though several names may share
	// one — a duplicate import identifier is a TypeScript error.
	comps := map[string]bool{}
	for _, d := range byName {
		comps[d.Lucide] = true
	}
	imports := make([]string, 0, len(comps))
	for c := range comps {
		imports = append(imports, c)
	}
	sort.Strings(imports)

	var b bytes.Buffer
	b.WriteString(`// Code generated by cmd/gen-icons. DO NOT EDIT.
// Source: internal/dataentryconfig/icondefs. Regenerate with ` + "`just generate-icons`" + `.
//
// Icons are imported BY NAME so the bundler tree-shakes the rest of Lucide, and
// so that resolving a config string stays a lookup in a closed, statically-known
// map. A config value must never be able to name an arbitrary component; see
// resolveIcon in ./icons for the lookup that enforces it.

import type { Component } from 'vue'
import {
`)
	// Aliased with a prefix, not imported bare. Lucide exports a component
	// literally called `Component`, which collides with Vue's `Component` type
	// the moment an icon happens to use it — and the failure is a confusing
	// "only refers to a type" error far from its cause. Prefixing every import
	// makes the collision impossible rather than merely unlikely.
	for _, c := range imports {
		fmt.Fprintf(&b, "  %s as %s,\n", c, lucideAlias(c))
	}
	b.WriteString(`} from 'lucide-vue-next'

/** ICONS maps a config-facing name to its component. Keys are the public
 * contract — renaming one breaks every project that authored it. */
export const ICONS: Record<string, Component> = {
`)
	for _, d := range byName {
		fmt.Fprintf(&b, "  '%s': %s,\n", d.Name, lucideAlias(d.Lucide))
	}
	b.WriteString(`}

/* Icons the SPA itself renders, outside any author-supplied config.
 *
 * Exported by name ON PURPOSE. Reaching through ICONS.<name> would type as
 * Component rather than Component | undefined, so dropping or renaming an entry
 * raises no error and Vue renders nothing — the glyph just disappears. As a
 * named import it is a build failure instead. */
`)
	for _, d := range byName {
		if export := icondefs.SPAExport(d.Name); export != "" {
			fmt.Fprintf(&b, "export const %s: Component = %s\n", export, lucideAlias(d.Lucide))
		}
	}
	return b.String()
}

// lucideAlias namespaces a Lucide component name so it cannot collide with
// anything else in the generated module's scope — notably Vue's `Component`.
func lucideAlias(name string) string { return "Lu" + name }

// RenderDocs produces the documentation table, grouped by category in table
// order.
func RenderDocs(defs []icondefs.IconDef) string {
	var b strings.Builder
	fmt.Fprintf(&b, `
The %d names below are the complete set. This table is **generated** from the
same definition the server validates against and the app renders from, so it
cannot fall out of step with either — an earlier hand-written copy went stale
within a single release, which is why it is machine-written now.

Use `+"`none`"+` to suppress an icon entirely; it is not in the table because it
draws nothing.
`, len(defs))

	var current string
	for _, d := range defs {
		if d.Category != current {
			current = d.Category
			fmt.Fprintf(&b, "\n#### %s\n\n| Name | Glyph | Description |\n| --- | --- | --- |\n", current)
		}
		// The Lucide component name is published deliberately. It is the only
		// thing that disambiguates a near-pair at a glance (five Circle*
		// glyphs differ only in their interiors at 18px), it lets an author
		// look the glyph up, and it makes "name the glyph, not the use site"
		// visible in review — a mismatched row is obvious in the rendered
		// table. It is not a leaked implementation detail: the SPA ships the
		// icons and the set is operator-facing config either way.
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", d.Name, d.Lucide, d.Desc)
	}
	return b.String()
}

// ReplaceRegion swaps the content between the generated-region markers.
//
// It fails rather than appending when a marker is missing: silently adding a
// second region would leave the docs with two tables, one of them permanently
// stale.
func ReplaceRegion(doc, body string) (string, error) {
	start := strings.Index(doc, docsBegin)
	if start < 0 {
		return "", fmt.Errorf("missing marker %q", docsBegin)
	}
	end := strings.Index(doc[start:], docsEnd)
	if end < 0 {
		return "", fmt.Errorf("missing marker %q after %q", docsEnd, docsBegin)
	}
	end += start
	return doc[:start+len(docsBegin)] + body + doc[end:], nil
}
