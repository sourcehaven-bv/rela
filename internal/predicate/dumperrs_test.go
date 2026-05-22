package predicate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// TestDumpRejectErrors is a development helper, not a regression
// gate. It prints the actual error string and type the engine
// produces for each .lua file in testdata/reject/. Useful for
// reviewing the user-facing UX of error messages, and as the source
// of truth when updating the .want files after a deliberate message
// change.
//
// Skipped unless PREDICATE_DUMP_REJECT_ERRORS=1 is set. Run with:
//
//	PREDICATE_DUMP_REJECT_ERRORS=1 go test ./internal/predicate/... \
//	    -run TestDumpRejectErrors -v
func TestDumpRejectErrors(t *testing.T) {
	if os.Getenv("PREDICATE_DUMP_REJECT_ERRORS") == "" {
		t.Skip("set PREDICATE_DUMP_REJECT_ERRORS=1 to dump")
	}
	env := testEnv(t)
	dir := filepath.Join("testdata", "reject")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		src, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		_, err := predicate.Compile(env, string(src))
		t.Logf("\n=== %s\n  type=%T\n  msg=%s\n", e.Name(), err, err.Error())
	}
}
