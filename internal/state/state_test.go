package state

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newTestKV(t *testing.T) *FSKV {
	t.Helper()
	mem := storage.NewMemFS()
	if err := mem.MkdirAll("/root", 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	rfs, err := storage.NewRootedFS(mem, "/root")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	return NewFSKV(rfs)
}

func TestFSKV_Put_Get_RoundTrip(t *testing.T) {
	kv := newTestKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "cache.json", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := kv.Get(ctx, "cache.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestFSKV_Put_NestedKey_CreatesDirs(t *testing.T) {
	kv := newTestKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "documents/render-abc.html", []byte("<html/>")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := kv.Get(ctx, "documents/render-abc.html")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "<html/>" {
		t.Fatalf("got %q", got)
	}
}

func TestFSKV_Get_MissingKey(t *testing.T) {
	kv := newTestKV(t)
	_, err := kv.Get(context.Background(), "missing.json")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestFSKV_Put_RejectsInvalidKey(t *testing.T) {
	kv := newTestKV(t)
	ctx := context.Background()
	cases := []string{"", "..", "/abs", "with\\bs", "sub/../esc"}
	for _, k := range cases {
		t.Run(k, func(t *testing.T) {
			if err := kv.Put(ctx, k, []byte("x")); err == nil {
				t.Fatalf("expected error for key %q", k)
			}
		})
	}
}

func TestFSKV_Get_RejectsInvalidKey(t *testing.T) {
	kv := newTestKV(t)
	if _, err := kv.Get(context.Background(), ".."); err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestFSKV_Delete_RemovesKey(t *testing.T) {
	kv := newTestKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "cache.json", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := kv.Delete(ctx, "cache.json"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := kv.Get(ctx, "cache.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist after Delete, got %v", err)
	}
}

func TestFSKV_Delete_MissingKeyIsNotError(t *testing.T) {
	kv := newTestKV(t)
	if err := kv.Delete(context.Background(), "never-existed.json"); err != nil {
		t.Fatalf("Delete on missing key should be a no-op, got %v", err)
	}
}

func TestFSKV_Delete_RejectsInvalidKey(t *testing.T) {
	kv := newTestKV(t)
	if err := kv.Delete(context.Background(), ".."); err == nil {
		t.Fatal("expected error for invalid key")
	}
}
