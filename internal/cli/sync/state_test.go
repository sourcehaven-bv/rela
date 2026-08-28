package sync

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestState_LoadSaveRoundTrip(t *testing.T) {
	fs := storage.NewMemFS()
	dir := "/proj/.rela"

	// Missing file → empty state, not an error.
	s, err := LoadState(fs, dir)
	if err != nil {
		t.Fatalf("LoadState (missing): %v", err)
	}
	if len(s.Records) != 0 || s.Cursor != "" {
		t.Fatalf("fresh state not empty: %+v", s)
	}

	s.Set("TKT-1", "etag-a", "hash-a", "ticket")
	s.Set("A/blocks/B", "etag-b", "hash-b", "blocks")
	s.Cursor = "42"
	if saveErr := s.Save(fs, dir); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	got, err := LoadState(fs, dir)
	if err != nil {
		t.Fatalf("LoadState (after save): %v", err)
	}
	if b, _ := got.Baseline("TKT-1"); b.Server != "etag-a" || b.Local != "hash-a" {
		t.Errorf("TKT-1 baseline=%+v, want {etag-a hash-a}", b)
	}
	if b, _ := got.Baseline("A/blocks/B"); b.Server != "etag-b" || b.Local != "hash-b" {
		t.Errorf("relation baseline=%+v, want {etag-b hash-b}", b)
	}
	if got.Cursor != "42" {
		t.Errorf("cursor=%q, want 42", got.Cursor)
	}
}

// TestLoadState_MigratesLegacySingleStringFormat pins the backward-compat path:
// an index written in the pre-TKT-8P1TM7 single-string shape must load, mapping
// the old shared canonical hash onto Local and leaving Server empty (forcing a
// re-fetch to learn the primary's /api/v1 ETag).
func TestLoadState_MigratesLegacySingleStringFormat(t *testing.T) {
	fs := storage.NewMemFS()
	dir := "/proj/.rela"
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{"records":{"TKT-1":"hash-a","A/blocks/B":"hash-b"},"cursor":"7"}`
	if err := fs.WriteFile(dir+"/"+stateFileName, []byte(legacy), 0o644); err != nil {
		t.Fatalf("seed legacy state: %v", err)
	}
	got, err := LoadState(fs, dir)
	if err != nil {
		t.Fatalf("LoadState (legacy): %v", err)
	}
	if b, _ := got.Baseline("TKT-1"); b.Local != "hash-a" || b.Server != "" {
		t.Errorf("migrated TKT-1=%+v, want Local=hash-a Server=\"\"", b)
	}
	if b, _ := got.Baseline("A/blocks/B"); b.Local != "hash-b" || b.Server != "" {
		t.Errorf("migrated relation=%+v, want Local=hash-b Server=\"\"", b)
	}
	if got.Cursor != "7" {
		t.Errorf("cursor=%q, want 7", got.Cursor)
	}
}

func TestState_CorruptFile_Errors(t *testing.T) {
	fs := storage.NewMemFS()
	dir := "/proj/.rela"
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := fs.WriteFile(dir+"/"+stateFileName, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	// A corrupt index must error rather than silently re-pushing everything.
	if _, err := LoadState(fs, dir); err == nil {
		t.Fatal("LoadState on corrupt file should error")
	}
}

// Pull both-dirty: a record changed remotely AND locally halts as a conflict,
// the local copy is preserved, and the cursor does NOT advance (so a re-run
// revisits the conflict).
func TestPull_BothDirty_Conflict(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Shared baseline via a push (the primary mints the id).
	h.createLocalEntity(t, "tmp-7", map[string]any{"title": "base"})
	if _, err := h.engine.Push(ctx); err != nil {
		t.Fatalf("baseline push: %v", err)
	}
	id := h.onlyServerEntityID(t)
	cursorBefore := h.idx.Cursor

	// Remote edits the record.
	h.server.mu.Lock()
	h.server.entities[id].Properties["title"] = "remote"
	h.server.recordChange("e", id, "ticket", false)
	h.server.mu.Unlock()

	// Local also edits it (now dirty vs index).
	if err := h.st.UpdateEntity(ctx, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": "local"}}); err != nil {
		t.Fatalf("local edit: %v", err)
	}

	rep, err := h.engine.Pull(ctx)
	if err != nil {
		t.Fatalf("pull both-dirty errored instead of halting: %v", err)
	}
	if rep.Conflicts != 1 || rep.Applied != 0 {
		t.Fatalf("conflicts=%d applied=%d, want 1/0", rep.Conflicts, rep.Applied)
	}
	// Local copy preserved.
	got, _ := h.st.GetEntity(ctx, id)
	if got.Properties["title"] != "local" {
		t.Fatalf("local clobbered: title=%v, want local", got.Properties["title"])
	}
	// Cursor unchanged → re-run revisits.
	if h.idx.Cursor != cursorBefore {
		t.Fatalf("cursor advanced past conflict: %q -> %q", cursorBefore, h.idx.Cursor)
	}

	// Force-pull resolves: remote wins.
	if _, err := h.engine.ForcePull(ctx, id); err != nil {
		t.Fatalf("force pull: %v", err)
	}
	got, _ = h.st.GetEntity(ctx, id)
	if got.Properties["title"] != "remote" {
		t.Fatalf("force pull title=%v, want remote", got.Properties["title"])
	}
}
