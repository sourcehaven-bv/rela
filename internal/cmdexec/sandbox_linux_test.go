package cmdexec

import (
	"slices"
	"testing"
)

// TestSystemReadOnlyPathsAreFixed enforces "Do not grow this list" on
// systemReadOnlyPaths. Anything else a command needs is host-specific and comes
// from the operator (RELA_SANDBOX_*_READ_PATHS); compiling it in exposes it on
// every host. Changing this list must be a deliberate edit to this test too.
func TestSystemReadOnlyPathsAreFixed(t *testing.T) {
	want := []string{"/usr", "/bin", "/sbin", "/lib", "/lib64", "/lib32"}
	if !slices.Equal(systemReadOnlyPaths, want) {
		t.Errorf("systemReadOnlyPaths = %v, want %v; host paths belong in the operator read paths",
			systemReadOnlyPaths, want)
	}
}
