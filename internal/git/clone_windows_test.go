//go:build windows

package git

import (
	"path/filepath"
	"testing"
)

// TestIsFilesystemRoot_RealWindowsPaths asks the real Windows filepath for the
// values isFilesystemRoot compares, instead of asserting on its behalf.
//
// TestIsFilesystemRoot (clone_test.go) hand-transcribes what filepath.Clean is
// believed to return for each Windows path, because Linux CI cannot produce
// those values. That is the only way to exercise the branch there, and it has a
// known failure mode: a wrong transcription passes green over a string Windows
// never emits. It did — the first version of that table gave `\\server\share`
// a trailing separator that Clean does not add, so the spelling that actually
// occurs went unguarded (RR-Q73HKS).
//
// This file is the counterweight. It never spells out a cleaned form; it Cleans
// the input and feeds the result straight through, so it cannot inherit a
// mistaken belief about Clean. It does not run on Linux CI, so it is not a
// replacement for the table — it is what stops the table drifting from reality
// the next time Clean's behaviour moves, and what a maintainer on Windows can
// run to check the claim directly.
//
// `\\?\UNC\host\share` is deliberately absent. volumeNameLen's `\\.\UNC`
// special case keys on the `.` form, so the `?` form takes the generic `\\?`
// branch and its volume is `\\?\UNC` — making the share a path under a volume
// rather than a root. Neither this file nor the stdlib's own volumenametests
// pins that, so asserting either answer would be the same unverified guess the
// bare-UNC row already cost us once.
func TestIsFilesystemRoot_RealWindowsPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"drive root", `C:\`, true},
		{"drive root lowercase", `c:\`, true},
		{"drive root via traversal", `C:\Users\..`, true},
		{"directory on drive", `C:\Users`, false},
		{"nested directory on drive", `C:\Users\dev\clones`, false},
		{"drive relative", `C:foo`, false},

		// The bare UNC volume: the form Clean leaves without a trailing
		// separator, and the one the Linux table originally got wrong.
		{"unc share root bare", `\\server\share`, true},
		{"unc share root trailing separator", `\\server\share\`, true},
		{"directory on share", `\\server\share\repos`, false},

		{"extended drive root", `\\?\C:\`, true},
		{"extended drive volume", `\\?\C:`, true},
		{"directory under extended drive", `\\?\C:\Users`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := filepath.Clean(tt.path)
			volume := filepath.VolumeName(cleaned)
			got := isFilesystemRoot(volume, cleaned, filepath.Separator)
			if got != tt.want {
				t.Errorf("isFilesystemRoot(%q, %q, %q) = %v, want %v (from input %q)",
					volume, cleaned, string(filepath.Separator), got, tt.want, tt.path)
			}
		})
	}
}

// containedPath must refuse a real Windows root outright, rather than accepting
// it and letting filepath.Rel report every path on the volume as contained.
//
// This is the end-to-end half: TestIsFilesystemRoot_RealWindowsPaths proves the
// predicate is right about real Windows values, and this proves containedPath
// still consults it. On Linux the equivalent wiring is covered by
// TestContainedPath_RejectsRootBase.
func TestContainedPath_RejectsWindowsRootBases(t *testing.T) {
	bases := []string{`C:\`, `\\server\share`, `\\server\share\`}

	for _, base := range bases {
		t.Run(base, func(t *testing.T) {
			_, err := containedPath(base, filepath.Join(base, "repo"))
			if err == nil {
				t.Fatalf("containedPath(%q, ...) = nil error, want the root-base rejection", base)
			}
		})
	}
}
