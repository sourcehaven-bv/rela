package storage

import (
	"errors"
	"io"
	"io/fs"
	"testing"
)

func TestReadOnlyFS(t *testing.T) {
	_, err := NewReadOnlyFS(nil)
	if err == nil {
		t.Fatal("NewReadOnlyFS(nil) succeeded")
	}

	mem := NewMemFS()
	if err = mem.MkdirAll("/p", 0o755); err != nil {
		t.Fatal(err)
	}
	if err = mem.WriteFile("/p/a.txt", []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ro, err := NewReadOnlyFS(mem)
	if err != nil {
		t.Fatal(err)
	}

	writes := []struct {
		name string
		call func() error
	}{
		{"WriteFile", func() error { return ro.WriteFile("/p/b.txt", nil, 0o644) }},
		{"Remove", func() error { return ro.Remove("/p/a.txt") }},
		{"Rename", func() error { return ro.Rename("/p/a.txt", "/p/c.txt") }},
		{"MkdirAll", func() error { return ro.MkdirAll("/p/d", 0o755) }},
	}
	for _, tc := range writes {
		t.Run(tc.name, func(t *testing.T) {
			if err = tc.call(); !errors.Is(err, ErrReadOnly) {
				t.Fatalf("got %v, want ErrReadOnly", err)
			}
		})
	}

	data, err := ro.ReadFile("/p/a.txt")
	if err != nil || string(data) != "a" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	if _, err = ro.Stat("/p/a.txt"); err != nil {
		t.Fatalf("Stat: %v", err)
	}
	entries, err := ro.ReadDir("/p")
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadDir = %v, %v", entries, err)
	}
	var walked int
	if err = ro.Walk("/p", func(string, fs.DirEntry, error) error { walked++; return nil }); err != nil {
		t.Fatal(err)
	}
	if walked != 2 {
		t.Fatalf("walked %d entries, want 2", walked)
	}
	rc, err := ro.Open("/p/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != "a" {
		t.Fatalf("Open read %q", got)
	}
	if _, err := ro.Getwd(); err != nil {
		t.Fatalf("Getwd: %v", err)
	}
}
