package dataentry

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoStrayWriteRequestConstruction is AC10 — the structural
// same-code-path invariant for action affordances. Direct construction
// of `acl.WriteRequest{Op:` outside `affordances.go` would drift the
// read-time verdict (the `_actions` map) from the write-time
// enforcement (the actual handler). [translateVerb] is the single
// source of truth — both the serializer and the write handlers route
// their request construction through it.
//
// Write handlers in `internal/dataentry` reach the ACL via
// `entityManager.Manager`, which does the construction inside
// `internal/entitymanager` — also a single point of construction.
// So `internal/dataentry` should never need a literal
// `acl.WriteRequest{Op:`.
//
// Adding a new verb in a follow-up phase: add an entry to
// [translateVerb], do not introduce a parallel construction site.
func TestNoStrayWriteRequestConstruction(t *testing.T) {
	const allowedFile = "affordances.go"
	const needle = "acl.WriteRequest{Op:"

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Limit to .go non-test sources in this package directory.
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			return nil
		}
		if filepath.Dir(path) != root {
			return nil
		}
		if base == allowedFile {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), needle) {
			t.Errorf("file %s contains %q — the only allowed construction site is %s (translateVerb). Add new verbs there.", path, needle, allowedFile)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestNavFilterStaysPresentational is the structural guard for the sidebar
// navigation filter (TKT-TXDK8U).
//
// `permitsNavEntry` decides which nav entries appear in /_sidebar. That is a
// UX affordance: it enforces nothing, and every target re-checks independently.
// Calling it from anywhere else — a read path, a write path, a second endpoint
// — would quietly turn a menu-tidying helper into an authorization decision,
// and the endpoint it "protected" would be gated by a predicate designed for
// presentation.
//
// This is a live risk, not a hypothetical: an earlier attempt at menu filtering
// was reverted (TKT-M1AX6P) precisely because it drifted toward being treated
// as concealment. A prose rule in CLAUDE.md failed to prevent that; a grep test
// does not decay.
//
// Genuinely need the predicate elsewhere? That is the moment to stop and ask
// whether you want an authorization check instead — and if so, to build one
// (see authorizeCommand), not to reuse this.
//
// TKT-53KICM added a SECOND presentation surface (dashboard cards) and shared
// the underlying policy as `permitsGatedUIElement`, so this guards two needles
// with two different allow-lists rather than one widened list:
//
//   - `permitsNavEntry(` — still exactly one file. It is nav-specific and has no
//     business anywhere else.
//   - `permitsGatedUIElement(` — the shared switch, allowed in the two
//     presentation handlers that legitimately gate on a `permission:`.
//
// Widening either list is a deliberate, argued exception and not routine: the
// guard's value comes from the list being short, because the reviewer's question
// stays "why are you calling this at all?" rather than "why is your file
// different from the several that already do?".
func TestNavFilterStaysPresentational(t *testing.T) {
	// Each needle maps to the set of non-test files permitted to call it.
	guards := []struct {
		needle  string
		allowed map[string]bool
		why     string
	}{
		{
			needle:  "permitsNavEntry(",
			allowed: map[string]bool{"views_handler.go": true},
			why:     "the sidebar filter is presentation only",
		},
		{
			needle:  "permitsGatedUIElement(",
			allowed: map[string]bool{"views_handler.go": true, "dashboard_handler.go": true},
			why:     "the shared UI-element filter is presentation only",
		},
		{
			// Unlike the two above, this needle guards a real boundary rather
			// than a presentation filter. toDocumentRenderConfig is the ONLY
			// producer of documentRenderConfig.Elevated == true, i.e. the single
			// switch that turns on raw ACL bypass for a render. Both permitted
			// callers check gateElevatedDocument first; a third caller that
			// forgot would compile and silently elevate.
			needle: "toDocumentRenderConfig(",
			allowed: map[string]bool{
				"standalone_document_handler.go": true,
				"api_v1.go":                      true,
				"handlers_document.go":           true, // the definition itself
			},
			why: "it is the only switch that enables elevated (ACL-bypassing) reads, " +
				"and every caller must pass gateElevatedDocument first",
		},
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			return nil
		}
		if filepath.Dir(path) != root {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, g := range guards {
			if g.allowed[base] {
				continue
			}
			if strings.Contains(string(body), g.needle) {
				t.Errorf("file %s calls %q — %s and must stay in %s; "+
					"if you need an authorization check, build one (see authorizeCommand)",
					path, g.needle, g.why, strings.Join(sortedKeys(g.allowed), " / "))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// sortedKeys returns m's keys in sorted order, for deterministic messages.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
