package attachment

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestContentTypeForName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"foo.pdf", "application/pdf"},
		{"diagram.png", "image/png"},
		{"no-ext", "application/octet-stream"},
		{"", "application/octet-stream"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := contentTypeForName(tc.in)
			if got != tc.want && !strings.HasPrefix(got, tc.want) {
				t.Errorf("contentTypeForName(%q) = %q, want %q (or prefix)", tc.in, got, tc.want)
			}
		})
	}
}

// TestUploadSanitizerAgreesWithRootedFS verifies that the upload
// path's filename-sanitization (filepath.Base, applied in
// [Service.Attach]) produces filenames that RootedFS.resolve
// accepts — or that RootedFS rejects loudly when Base's
// sanitization is insufficient.
//
// This models the production path exactly: caller supplies a source
// filepath; [Service.Attach] takes filepath.Base as the stored
// filename; fsstore builds an attachment key
// attachments/<eid>/<prop>/<fname>; RootedFS.resolve validates that
// key before hitting SafeFS.WriteFile.
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
		input        string
		writeSucceed bool
		explanation  string
	}{
		{"../../etc/passwd", true, "Base strips traversal → 'passwd'"},
		{"/absolute/path/x.txt", true, "Base strips → 'x.txt'"},
		{"foo/bar", true, "Base strips → 'bar'"},

		{"normal.txt", true, "plain name preserved"},
		{"héllo.txt", true, "unicode preserved"},
		{"with spaces.txt", true, "spaces preserved"},
		{"file.tar.gz", true, "multi-dot preserved"},

		{"..", false, "Base returns '..', RootedFS rejects"},
		{"file:name.txt", false, "colon: RootedFS rejects"},
		{"CON.txt", false, "Windows reserved: RootedFS rejects"},
		{"nul", false, "Windows reserved: RootedFS rejects"},
		{"com1.log", false, "Windows reserved: RootedFS rejects"},
		{"with\x00nul", false, "NUL byte: RootedFS rejects"},
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
