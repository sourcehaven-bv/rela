package state_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statetest"
)

// TestValidateKey pins the key rules directly. They mirror
// storage.RootedFS.resolve deliberately: a key accepted by a database backend
// must still be accepted after a migration to the filesystem backend, so the
// two must agree. The conformance suite exercises the same rules through both
// backends; this table documents them in one readable place.
func TestValidateKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		ok   bool
	}{
		{"simple", "cache.json", true},
		{"nested", "documents/DOC-1-abc.html", true},
		{"deeply nested", "a/b/c/d.txt", true},
		{"dots in filename", "aliases.v2.json", true},
		{"leading dot file", ".hidden", true},

		{"empty", "", false},
		{"traversal", "..", false},
		{"traversal mid-key", "sub/../esc", false},
		{"leading traversal", "../etc/passwd", false},
		{"absolute", "/abs", false},
		{"backslash", "with\\bs", false},
		{"colon", "c:drive", false},
		{"empty segment", "a//b", false},
		{"dot segment", "./rel", false},
		{"trailing slash", "dir/", false},
		{"NUL", "nul\x00byte", false},
		{"control char", "bell\x07", false},
		{"DEL", "del\x7f", false},
		{"windows reserved", "con", false},
		{"windows reserved with ext", "PRN.txt", false},
		{"windows reserved in segment", "sub/aux/file", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := state.ValidateKey(tc.key)
			if tc.ok && err != nil {
				t.Fatalf("ValidateKey(%q) = %v, want nil", tc.key, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("ValidateKey(%q) = nil, want an error", tc.key)
			}
		})
	}
}

// permissiveKV is a KV with no key policy at all — the shape of a database
// backend, which stores whatever string it is handed.
type permissiveKV struct {
	data map[string][]byte
}

func newPermissiveKV() *permissiveKV { return &permissiveKV{data: map[string][]byte{}} }

func (p *permissiveKV) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := p.data[key]
	if !ok {
		return nil, &os.PathError{Op: "get", Path: key, Err: os.ErrNotExist}
	}
	return append([]byte(nil), v...), nil
}

func (p *permissiveKV) Put(_ context.Context, key string, data []byte) error {
	p.data[key] = append([]byte(nil), data...)
	return nil
}

func (p *permissiveKV) Delete(_ context.Context, key string) error {
	delete(p.data, key)
	return nil
}

// TestValidatedKV_RejectsBeforeDelegating is the point of the wrapper: an
// invalid key must never reach the inner store. Asserting the inner store stays
// empty (rather than just that an error came back) is what proves rejection
// happened first — a wrapper that delegated and then errored would leave the
// row behind.
func TestValidatedKV_RejectsBeforeDelegating(t *testing.T) {
	inner := newPermissiveKV()
	kv, err := state.NewValidatedKV(inner)
	if err != nil {
		t.Fatalf("NewValidatedKV: %v", err)
	}
	ctx := context.Background()

	if err := kv.Put(ctx, "../escape", []byte("x")); err == nil {
		t.Fatal("Put with a traversal key must be rejected")
	}
	if len(inner.data) != 0 {
		t.Fatalf("invalid key reached the inner store: %v", inner.data)
	}
	if _, err := kv.Get(ctx, "../escape"); err == nil {
		t.Fatal("Get with a traversal key must be rejected")
	}
	if err := kv.Delete(ctx, "../escape"); err == nil {
		t.Fatal("Delete with a traversal key must be rejected")
	}
}

// TestValidatedKV_PassesValidKeysThrough ensures the wrapper is transparent for
// keys that are fine — it must not become a second, stricter policy.
func TestValidatedKV_PassesValidKeysThrough(t *testing.T) {
	inner := newPermissiveKV()
	kv, err := state.NewValidatedKV(inner)
	if err != nil {
		t.Fatalf("NewValidatedKV: %v", err)
	}
	ctx := context.Background()
	const key = "documents/DOC-1-abc.html"

	if err = kv.Put(ctx, key, []byte("<html>")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := kv.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "<html>" {
		t.Fatalf("Get = %q", got)
	}
	if err = kv.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err = kv.Get(ctx, key); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Get after Delete = %v, want not-exist", err)
	}
}

func TestNewValidatedKV_RejectsNilInner(t *testing.T) {
	if _, err := state.NewValidatedKV(nil); err == nil {
		t.Fatal("a nil inner KV must fail at construction, not at first use")
	}
}

// TestValidatedKV_Conformance runs the full shared contract through the wrapper
// over a permissive backend — the same composition production uses over
// pgstore, so the wrapper is proven not to break any clause of the contract.
func TestValidatedKV_Conformance(t *testing.T) {
	statetest.RunAll(t, func(tb testing.TB) state.KV {
		tb.Helper()
		kv, err := state.NewValidatedKV(newPermissiveKV())
		if err != nil {
			tb.Fatalf("NewValidatedKV: %v", err)
		}
		return kv
	})
}
