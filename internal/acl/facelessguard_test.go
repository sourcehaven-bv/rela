package acl_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// facelessAllowed lists the non-test call sites permitted to build a subject
// that names no face, keyed by repo-relative path, with the reason.
//
// It is an ALLOWLIST, not an exemption list: a new faceless call site fails
// this test until someone writes down why the operation has no single face.
// That is the point of the whole P5 change (BUG-Y0GNSB) — the faceless case is
// real, but it must be argued for, never fallen into. The exemption lists used
// elsewhere in this package (ceilingguard_test.go) fail closed for a new FILE;
// this fails closed for a new CALL, which is the right grain here because one
// file may hold both faced and faceless writes.
var facelessAllowed = map[string]string{
	"internal/entitymanager/manager.go": "rename re-keys the whole entity family " +
		"(store.RenameEntity takes no Face), so naming one face would assert a " +
		"narrower scope than the operation has",
	"internal/docs/assert_acl.go": "the allows{}/refuses{} Lua surface has no " +
		"`face` field, so a doc claim has no face to name",
}

var facelessCall = regexp.MustCompile(`\bNewFacelessEntitySubject\(`)

// TestFacelessSubjectsAreArguedFor scans the repository's non-test sources and
// fails on any [acl.NewFacelessEntitySubject] call from a file not in
// [facelessAllowed].
//
// # Why this guard still exists after P5
//
// P5 made the compiler enforce the original invariant: [acl.EntitySubject] has
// unexported fields, so `acl.EntitySubject{Type: t, ID: i}` — the literal that
// silently authorized the DEFAULT face while the store wrote the PUBLISHED one
// — no longer compiles anywhere. The predecessor of this test
// (TestEveryEntitySubjectNamesItsFace, a regex scan for literals omitting
// `Face:`) is therefore obsolete: it scans for a construct the language now
// rejects, so it could only ever pass.
//
// What the type CANNOT enforce is the remaining discretionary choice. A caller
// must pick between [acl.NewEntitySubject] and [acl.NewFacelessEntitySubject],
// and picking the faceless one is a one-word way back to the original
// behaviour — authorize against the default face — with no compile error. The
// compiler guarantees the choice is MADE; only review can judge whether it is
// RIGHT. This test turns that judgement into a merge-blocking artifact.
//
// # What it does NOT catch
//
// A [acl.NewEntitySubject] call passing the WRONG face expression — for
// instance the caller-supplied body face where the stored one is
// authoritative. No scan can answer that; ApplyEntity's ErrFaceImmutable guard
// and internal/dataentry's TestFacedIDWrite_* cover it.
func TestFacelessSubjectsAreArguedFor(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for path := range facelessAllowed {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("facelessAllowed names %s, which does not exist: %v\n"+
				"A stale allowlist entry silently permits a future faceless call "+
				"in a file nobody argued for.", path, err)
		}
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			return readErr
		}
		if !facelessCall.Match(src) {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "internal/acl/subject.go" {
			return nil // the declaration itself
		}
		if _, ok := facelessAllowed[rel]; !ok {
			t.Errorf("%s calls acl.NewFacelessEntitySubject but is not in facelessAllowed.\n"+
				"A faceless subject authorizes against the DEFAULT face. If the "+
				"operation really spans every face (like rename), add this file to "+
				"facelessAllowed with the reason. If it targets ONE face, call "+
				"acl.NewEntitySubject with the face of the entity being written "+
				"(BUG-Y0GNSB).", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

// repoRoot walks up from the package directory to the module root, so the scan
// covers every package rather than only this one. The faceless constructor is
// exported and callable from anywhere, so a package-local scan — the shape the
// predecessor test had — would miss exactly the new caller this guards against.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
