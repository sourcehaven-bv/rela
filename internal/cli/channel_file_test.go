package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingReader struct{ sent bool }

func (r *failingReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "part"), nil
	}
	return 0, errors.New("connection lost")
}

func TestWriteFileAtomic(t *testing.T) {
	t.Run("writes the whole file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.txt")
		if err := writeFileAtomic(path, strings.NewReader("hello")); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "hello" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("a failed read leaves nothing and keeps an existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := writeFileAtomic(path, &failingReader{}); err == nil {
			t.Fatal("want an error")
		}
		got, _ := os.ReadFile(path)
		if string(got) != "old" {
			t.Errorf("file is %q, want the old content", got)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Errorf("dir holds %d entries, want only out.txt", len(entries))
		}
	})
	t.Run("a missing directory fails", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "no", "out.txt")
		if err := writeFileAtomic(path, io.LimitReader(strings.NewReader("x"), 1)); err == nil {
			t.Fatal("want an error")
		}
	})
}
