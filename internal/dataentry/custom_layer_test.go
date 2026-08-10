package dataentry

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

// topLevelRootInsideLayer reports whether css (which starts at the `@layer
// rela` block) contains a `:root` DECLARATION rule at the layer's own nesting
// depth — i.e. one that should have been carved out.
//
// Depth-aware on purpose: a `:root` inside `@media`/`@supports` sits one level
// deeper and is a legitimate conditional override, not a token declaration.
// A flat offset comparison would flag it and fail on valid CSS.
func topLevelRootInsideLayer(css string) (int, bool) {
	depth := 0
	for i := range len(css) {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ':':
			// Only depth 1 is "directly inside the @layer rela block".
			if depth != 1 || !strings.HasPrefix(css[i:], ":root") {
				continue
			}
			// A token rule is `:root{` or `:root.dark{` — the selector ends at
			// the brace. Anything else (`:root .fa-x`, `:root, .y`) is a
			// descendant or list selector and belongs in the layer.
			rest := strings.TrimLeft(css[i+len(":root"):],
				"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_")
			if strings.HasPrefix(rest, "{") {
				return i, true
			}
		}
	}
	return 0, false
}

// TestBuiltCSSIsLayered pins the invariant that makes operator `custom.css`
// win the cascade: every stylesheet rela emits declares `@layer rela`, and
// the `:root` token blocks stay OUTSIDE it.
//
// Why this matters, concretely: the build emits ~19 stylesheets. One is linked
// eagerly from index.html; the rest are route-level chunks that Vite appends to
// <head> at RUNTIME, i.e. after the injected operator <link>. At equal
// specificity the later sheet wins, so before layering, operator CSS lost every
// tie against a route view — a skin worked on the dashboard and silently died
// on a list view. An unlayered declaration outranks a layered one regardless of
// order or specificity, which is what restores the operator's precedence.
//
// If a future asset escapes the wrap (new plugin ordering, a new emit path),
// this fails rather than letting the regression reach an operator.
func TestBuiltCSSIsLayered(t *testing.T) {
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		t.Skipf("embedded SPA not built: %v", err)
	}
	if _, statErr := fs.Stat(spaFS, spaIndexFile); statErr != nil {
		t.Skipf("embedded SPA not built: %v", statErr)
	}

	var checked int
	err = fs.WalkDir(spaFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".css" {
			return err
		}
		b, readErr := fs.ReadFile(spaFS, p)
		if readErr != nil {
			return readErr
		}
		checked++
		css := string(b)

		t.Run(path.Base(p), func(t *testing.T) {
			if !strings.Contains(css, "@layer rela") {
				t.Errorf("%s does not declare @layer rela — operator custom.css would "+
					"lose cascade ties against it (see TKT-3DBK6I)", p)
				return
			}
			// The bare `@layer rela;` declaration must come first (modulo
			// @charset/@import), pinning layer order at first parse rather than
			// letting whichever chunk loads first establish it.
			trimmed := strings.TrimSpace(css)
			startsWithPrelude := strings.HasPrefix(trimmed, "@charset") ||
				strings.HasPrefix(trimmed, "@import")
			if !startsWithPrelude && !strings.HasPrefix(trimmed, "@layer rela;") {
				t.Errorf("%s does not begin with the `@layer rela;` order declaration", p)
			}

			layerAt := strings.Index(css, "@layer rela{")
			if layerAt < 0 {
				layerAt = strings.Index(css, "@layer rela {")
			}
			if layerAt < 0 {
				return
			}
			// TOP-LEVEL token rules must sit ahead of the layer (DECISION 1).
			// A `:root` nested in @media/@supports is a conditional component
			// override, not part of the token contract, and correctly stays
			// inside — so only flag one at brace depth 0 within the layer.
			if at, ok := topLevelRootInsideLayer(css[layerAt:]); ok {
				t.Errorf("%s has a TOP-LEVEL :root token rule INSIDE @layer rela "+
					"(offset %d within the layer); token declarations must stay "+
					"unlayered so they behave identically in the SPA and in "+
					"custom-app iframes", p, at)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded SPA: %v", err)
	}
	if checked == 0 {
		t.Skip("no CSS assets in the embedded SPA build")
	}
}

// TestTopLevelRootInsideLayer exercises the guard itself: it must catch a
// carve-out failure without flagging valid CSS. A guard that cries wolf gets
// weakened by the next person to hit it.
func TestTopLevelRootInsideLayer(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want bool
	}{
		{"clean layer", "@layer rela{.a{color:red}}", false},
		{"top-level :root inside layer", "@layer rela{:root{--a:1}}", true},
		{"top-level :root.dark inside layer", "@layer rela{:root.dark{--a:1}}", true},
		{":root nested in @media is fine", "@layer rela{@media(min-width:0){:root{--a:1}}}", false},
		{":root nested in @supports is fine", "@layer rela{@supports(display:grid){:root{--a:1}}}", false},
		{"descendant selector is fine", "@layer rela{:root .fa-rotate-90{filter:none}}", false},
		{"selector list is fine", "@layer rela{:root, .x{--a:1}}", false},
		{"var() reference is fine", "@layer rela{.a{color:var(--x)}}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, got := topLevelRootInsideLayer(tt.css); got != tt.want {
				t.Errorf("topLevelRootInsideLayer(%q) = %v, want %v", tt.css, got, tt.want)
			}
		})
	}
}
