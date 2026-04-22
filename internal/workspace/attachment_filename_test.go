package workspace

import (
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestUploadSanitizerAgreesWithRootedFS verifies that the upload path's
// filename-sanitization (currently: filepath.Base, applied in
// workspace.AttachFile) produces filenames that RootedFS.resolve
// accepts — or that RootedFS rejects loudly when Base's sanitization
// is insufficient.
//
// This models the production path exactly: user supplies a source
// filepath; workspace.AttachFile takes filepath.Base as the stored
// filename; fsstore builds an attachment key attachments/<eid>/<prop>/<fname>;
// RootedFS.resolve validates that key before hitting SafeFS.WriteFile.
//
// Any input where Base's output trips resolve is a loud failure at
// upload time — documented, not silently corrupting.
func TestUploadSanitizerAgreesWithRootedFS(t *testing.T) {
	fs := storage.NewMemFS()
	rfs, err := storage.NewRootedFS(fs, "/")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}

	cases := []struct {
		input        string // what the caller passes to AttachFile
		writeSucceed bool   // whether the resulting attachment key writes OK
		explanation  string
	}{
		// Path traversal — Base strips dirs, surviving component is safe.
		{"../../etc/passwd", true, "Base strips traversal → 'passwd'"},
		{"/absolute/path/x.txt", true, "Base strips → 'x.txt'"},
		{"foo/bar", true, "Base strips → 'bar'"},

		// Base preserves these unchanged.
		{"normal.txt", true, "plain name preserved"},
		{"héllo.txt", true, "unicode preserved"},
		{"with spaces.txt", true, "spaces preserved"},
		{"file.tar.gz", true, "multi-dot preserved"},

		// Problematic inputs that Base does NOT sanitize away → RootedFS
		// rejects them loudly.
		{"..", false, "Base returns '..', RootedFS rejects"},
		{"file:name.txt", false, "colon: RootedFS rejects"},
		{"CON.txt", false, "Windows reserved: RootedFS rejects"},
		{"nul", false, "Windows reserved: RootedFS rejects"},
		{"com1.log", false, "Windows reserved: RootedFS rejects"},
		{"with\x00nul", false, "NUL byte: RootedFS rejects"},

		// filepath.Base on a backslash-separator input behaves OS-dependently
		// (POSIX: the whole string is the base; Windows: splits on '\').
		// Skipped because the assertion would diverge between POSIX CI and
		// Windows CI; covered indirectly by the resolve-level test on
		// rooted_test.go.
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			sanitized := filepath.Base(tc.input)
			key := "attachments/E1/prop/" + sanitized
			err := rfs.WriteFile(key, []byte("x"), 0o644)
			if tc.writeSucceed {
				if err != nil {
					t.Fatalf("expected write to succeed (%s): base=%q, got %v",
						tc.explanation, sanitized, err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected write to fail (%s): base=%q",
						tc.explanation, sanitized)
				}
			}
		})
	}
}
