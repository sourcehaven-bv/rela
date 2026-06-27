package store_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// constReader yields an endless stream of one byte, for feeding an
// oversize payload without allocating it.
type constReader byte

func (c constReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(c)
	}
	return len(p), nil
}

func TestCapAttachmentReader(t *testing.T) {
	t.Run("under the limit reads fully", func(t *testing.T) {
		r := store.CapAttachmentReader(strings.NewReader("hello"), 10)
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("data = %q, want %q", data, "hello")
		}
	})

	t.Run("exactly at the limit succeeds", func(t *testing.T) {
		r := store.CapAttachmentReader(strings.NewReader("12345"), 5)
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll at limit: %v", err)
		}
		if len(data) != 5 {
			t.Errorf("len = %d, want 5", len(data))
		}
	})

	t.Run("one byte over the limit errors", func(t *testing.T) {
		r := store.CapAttachmentReader(strings.NewReader("123456"), 5)
		_, err := io.ReadAll(r)
		if !errors.Is(err, store.ErrAttachmentTooLarge) {
			t.Errorf("err = %v, want ErrAttachmentTooLarge", err)
		}
	})

	t.Run("unbounded source trips the cap", func(t *testing.T) {
		r := store.CapAttachmentReader(constReader('a'), 1024)
		_, err := io.ReadAll(r)
		if !errors.Is(err, store.ErrAttachmentTooLarge) {
			t.Errorf("err = %v, want ErrAttachmentTooLarge", err)
		}
	})

	t.Run("zero limit rejects any content", func(t *testing.T) {
		r := store.CapAttachmentReader(strings.NewReader("x"), 0)
		_, err := io.ReadAll(r)
		if !errors.Is(err, store.ErrAttachmentTooLarge) {
			t.Errorf("err = %v, want ErrAttachmentTooLarge", err)
		}
	})
}

func TestValidateFileName(t *testing.T) {
	ok := []string{"a.txt", "report (1).pdf", "图片.png", "no-ext"}
	for _, n := range ok {
		if err := store.ValidateFileName(n); err != nil {
			t.Errorf("ValidateFileName(%q) = %v, want nil", n, err)
		}
	}
	bad := []string{"", "a/b.txt", "a\\b.txt", "a\x00b", ".", ".."}
	for _, n := range bad {
		if err := store.ValidateFileName(n); err == nil {
			t.Errorf("ValidateFileName(%q) = nil, want error", n)
		}
	}
}

func TestNormalizeFileName(t *testing.T) {
	cases := map[string]string{
		"report.pdf":     "report.pdf",
		"dir/report.pdf": "report.pdf", // strips path
		"a\\b.png":       "b.png",      // strips windows path
		"a\x01b.txt":     "a_b.txt",    // control char replaced
		"  spaced.txt  ": "spaced.txt", // trims spaces
		"":               "file",       // empty → fallback
		"..":             "file",       // traversal → fallback
		".":              "file",       // fallback
	}
	for in, want := range cases {
		if got := store.NormalizeFileName(in); got != want {
			t.Errorf("NormalizeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSuffixOnCollision(t *testing.T) {
	taken := map[string]bool{"report.pdf": true, "report (1).pdf": true, "noext": true}
	exists := func(n string) bool { return taken[n] }

	if got := store.SuffixOnCollision("fresh.pdf", exists); got != "fresh.pdf" {
		t.Errorf("no collision: got %q, want fresh.pdf", got)
	}
	if got := store.SuffixOnCollision("report.pdf", exists); got != "report (2).pdf" {
		t.Errorf("collision: got %q, want report (2).pdf", got)
	}
	if got := store.SuffixOnCollision("noext", exists); got != "noext (1)" {
		t.Errorf("no-extension collision: got %q, want \"noext (1)\"", got)
	}
}
