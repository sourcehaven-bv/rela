// Package statetest is the conformance harness for [state.KV]
// implementations. Any new backend must pass [RunAll].
//
// It exists because the KV contract is carried almost entirely in prose on the
// interface, and two of its clauses are load-bearing in ways a backend author
// would not guess:
//
//   - A missing key must fail with an error satisfying [os.IsNotExist].
//     internal/dataentry's document cache distinguishes "no cached render" from
//     "the cache is broken" by exactly this; a backend returning a bare
//     "not found" error would turn every cache miss into a hard failure.
//   - Deleting a missing key must SUCCEED. logoStore.Delete documents itself as
//     idempotent and calls Delete unconditionally to clear optional state.
//
// The key-validation clause matters for a different reason: FSKV resolves keys
// to filesystem paths, so it must reject traversal and Windows-hostile names. A
// database backend has no such constraint and would happily store `../../etc`,
// silently accepting keys the filesystem backend rejects. Divergence there means
// a project that works on one backend breaks when migrated to the other, so the
// suite holds both to the stricter contract.
package statetest

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/state"
)

// Factory returns a fresh, empty KV for one subtest. Implementations that need
// cleanup should register it on tb.
type Factory func(tb testing.TB) state.KV

// RunAll runs every conformance test against the KV produced by newKV.
func RunAll(t *testing.T, newKV Factory) {
	t.Helper()
	for name, fn := range map[string]func(*testing.T, Factory){
		"RoundTrip":               testRoundTrip,
		"Overwrite":               testOverwrite,
		"HierarchicalKeys":        testHierarchicalKeys,
		"EmptyValue":              testEmptyValue,
		"BinaryValue":             testBinaryValue,
		"GetMissingIsNotExist":    testGetMissingIsNotExist,
		"DeleteRemoves":           testDeleteRemoves,
		"DeleteMissingIsNotError": testDeleteMissingIsNotError,
		"DeleteIsIdempotent":      testDeleteIsIdempotent,
		"RejectsInvalidKeys":      testRejectsInvalidKeys,
		"KeysAreDistinct":         testKeysAreDistinct,
		"ValueIsNotAliased":       testValueIsNotAliased,
	} {
		t.Run(name, func(t *testing.T) { fn(t, newKV) })
	}
}

func testRoundTrip(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	want := []byte(`{"a":1}`)
	if err := kv.Put(ctx, "cache.json", want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := kv.Get(ctx, "cache.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Get = %q, want %q", got, want)
	}
}

func testOverwrite(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "k", []byte("first")); err != nil {
		t.Fatalf("Put first: %v", err)
	}
	if err := kv.Put(ctx, "k", []byte("second")); err != nil {
		t.Fatalf("Put second: %v", err)
	}
	got, err := kv.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("Get = %q, want %q — Put must replace, not append", got, "second")
	}
}

// testHierarchicalKeys pins the documented key shape: callers group state under
// a common prefix (the document cache uses "documents/<id>-<hash>.html"), so a
// backend must accept a `/`-separated key and create whatever intermediate
// structure it needs.
func testHierarchicalKeys(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	const key = "documents/DOC-1-deadbeef.html"
	if err := kv.Put(ctx, key, []byte("<html>")); err != nil {
		t.Fatalf("Put nested: %v", err)
	}
	got, err := kv.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get nested: %v", err)
	}
	if string(got) != "<html>" {
		t.Fatalf("Get = %q, want %q", got, "<html>")
	}
}

// testEmptyValue guards a real ambiguity: a backend that treats a zero-length
// value as absent would make a legitimately-empty cached render look like a
// miss, re-rendering forever.
func testEmptyValue(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "empty", []byte{}); err != nil {
		t.Fatalf("Put empty: %v", err)
	}
	got, err := kv.Get(ctx, "empty")
	if err != nil {
		t.Fatalf("Get empty: %v (an empty value must be present, not missing)", err)
	}
	if len(got) != 0 {
		t.Fatalf("Get = %q, want empty", got)
	}
}

// testBinaryValue matters because the logo store round-trips arbitrary uploaded
// bytes; a backend that assumes UTF-8 text would corrupt them.
func testBinaryValue(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	want := []byte{0x00, 0xff, 0xfe, 0x01, 0x80, '\n', 0x7f}
	if err := kv.Put(ctx, "logo.png", want); err != nil {
		t.Fatalf("Put binary: %v", err)
	}
	got, err := kv.Get(ctx, "logo.png")
	if err != nil {
		t.Fatalf("Get binary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Get = %v, want %v", got, want)
	}
}

func testGetMissingIsNotExist(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	_, err := kv.Get(context.Background(), "never-written.json")
	if err == nil {
		t.Fatal("Get of a missing key must return an error")
	}
	if !errors.Is(err, os.ErrNotExist) && !os.IsNotExist(err) {
		t.Fatalf("Get of a missing key = %v; must satisfy os.IsNotExist so callers "+
			"can tell a cache miss from a broken backend", err)
	}
}

func testDeleteRemoves(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := kv.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := kv.Get(ctx, "k"); !errors.Is(err, os.ErrNotExist) && !os.IsNotExist(err) {
		t.Fatalf("Get after Delete = %v, want a not-exist error", err)
	}
}

func testDeleteMissingIsNotError(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	if err := kv.Delete(context.Background(), "never-existed.json"); err != nil {
		t.Fatalf("Delete of a missing key must succeed (callers clear optional "+
			"state unconditionally), got %v", err)
	}
}

func testDeleteIsIdempotent(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := kv.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete #1: %v", err)
	}
	if err := kv.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete #2 must be a no-op, got %v", err)
	}
}

// testRejectsInvalidKeys holds every backend to FSKV's stricter key rules, so a
// key that works on one backend cannot fail on another. See the package doc.
func testRejectsInvalidKeys(t *testing.T, newKV Factory) {
	t.Helper()
	ctx := context.Background()
	for _, key := range []string{
		"",            // empty
		"..",          // traversal
		"sub/../esc",  // traversal mid-key
		"/abs",        // absolute
		"with\\bs",    // backslash
		"with:colon",  // Windows drive letter / ADS
		"a//b",        // empty segment
		"./rel",       // dot segment
		"nul\x00byte", // control character
	} {
		t.Run(key, func(t *testing.T) {
			kv := newKV(t)
			if err := kv.Put(ctx, key, []byte("x")); err == nil {
				t.Errorf("Put(%q) must be rejected", key)
			}
			if _, err := kv.Get(ctx, key); err == nil {
				t.Errorf("Get(%q) must be rejected", key)
			}
			if err := kv.Delete(ctx, key); err == nil {
				t.Errorf("Delete(%q) must be rejected", key)
			}
		})
	}
}

// testKeysAreDistinct catches a backend that normalizes or collapses keys —
// e.g. treating "a/b" and "a-b" alike, or lowercasing.
func testKeysAreDistinct(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	keys := []string{"a/b", "a-b", "A/b", "a/b.json"}
	for i, k := range keys {
		if err := kv.Put(ctx, k, []byte{byte('0' + i)}); err != nil {
			t.Fatalf("Put(%q): %v", k, err)
		}
	}
	for i, k := range keys {
		got, err := kv.Get(ctx, k)
		if err != nil {
			t.Fatalf("Get(%q): %v", k, err)
		}
		if len(got) != 1 || got[0] != byte('0'+i) {
			t.Errorf("Get(%q) = %q, want %q — keys must not collide", k, got, []byte{byte('0' + i)})
		}
	}
}

// testValueIsNotAliased ensures a backend does not hand back a slice the caller
// can mutate into the store (a live risk for an in-memory implementation).
func testValueIsNotAliased(t *testing.T, newKV Factory) {
	t.Helper()
	kv := newKV(t)
	ctx := context.Background()
	if err := kv.Put(ctx, "k", []byte("original")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := kv.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	for i := range got {
		got[i] = 'X'
	}
	again, err := kv.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get again: %v", err)
	}
	if string(again) != "original" {
		t.Fatalf("mutating a returned value changed the store: %q", again)
	}
}
