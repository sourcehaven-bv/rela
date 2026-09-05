package entitymanager

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// faceExemptMarker opts a single `acl.EntitySubject{...}` literal out of the
// Face requirement. It must appear in a comment shortly before the literal
// (see [exemptLookback]), together with the reason.
//
// A per-LITERAL marker rather than a per-file exemption list: manager.go holds
// six subject literals of which exactly one (rename) is legitimately faceless,
// so a file-level exemption would silently un-guard the other five — the same
// too-coarse-granularity mistake that let the original defect through.
//
// The requirement fails closed: a newly added `acl.EntitySubject{` literal
// without a Face must either set one or carry this marker. `Face` was
// introduced for the copy kernel, and every pre-existing construction site
// simply never learned about it — the zero value reads as "the default face",
// so the omission was invisible rather than a compile error (BUG-Y0GNSB).
const faceExemptMarker = "facesubject:no-face"

// subjectLiteral matches an acl.EntitySubject composite literal.
var subjectLiteral = regexp.MustCompile(`acl\.EntitySubject\{[^}]*\}`)

// exemptLookback is how many bytes before a literal are searched for
// [faceExemptMarker]. A fixed window rather than a comment-block regex: the
// marker's comment is separated from the literal by the `Subject:` field name
// and, at the rename site, by a multi-line justification — so anchoring on
// "comment immediately preceding" is brittle in exactly the case that needs it.
const exemptLookback = 600

// TestEveryEntitySubjectNamesItsFace scans this package's non-test sources and
// fails on any `acl.EntitySubject{...}` literal that does not set Face.
//
// # Why a source scan rather than a type change
//
// The obvious alternative — make Face a required constructor argument — was
// considered and is the better end state, but it is a wider change than a
// security fix should carry: `acl.EntitySubject` is a sealed sum member used
// across packages, and a constructor would have to thread through every one.
// A scan pins the invariant now, at the cost of being a heuristic.
//
// # What it does NOT catch
//
// A literal that sets `Face:` to the WRONG expression (e.g. the caller-supplied
// body face where the stored one is authoritative). That is a semantic question
// no regex can answer, and it is what TestFacedIDWrite_* in internal/dataentry
// and ApplyEntity's ErrFaceImmutable guard exist to cover. This test only
// ensures the field is not silently forgotten.
func TestEveryEntitySubjectNamesItsFace(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, readErr := os.ReadFile(filepath.Clean(name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		text := string(src)
		for _, loc := range subjectLiteral.FindAllStringIndex(text, -1) {
			lit := text[loc[0]:loc[1]]
			if strings.Contains(lit, "Face:") {
				continue
			}
			from := max(loc[0]-exemptLookback, 0)
			if strings.Contains(text[from:loc[0]], faceExemptMarker) {
				continue
			}
			t.Errorf("%s: acl.EntitySubject literal does not set Face:\n\t%s\n"+
				"A zero Face means THE DEFAULT STATE, so omitting it authorizes a "+
				"faced write against the wrong face (BUG-Y0GNSB). Set Face from the "+
				"entity being written, or mark the literal %q with a reason.",
				name, lit, faceExemptMarker)
		}
	}
}
