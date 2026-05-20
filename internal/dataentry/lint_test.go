package dataentry

import (
	"io/fs"
	"os"
	"path/filepath"
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
