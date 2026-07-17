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

	s.Set("TKT-1", "hash-a")
	s.Set("A/blocks/B", "hash-b")
	s.Cursor = "42"
	if saveErr := s.Save(fs, dir); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	got, err := LoadState(fs, dir)
	if err != nil {
		t.Fatalf("LoadState (after save): %v", err)
	}
	if h, _ := got.Hash("TKT-1"); h != "hash-a" {
		t.Errorf("TKT-1 hash=%q, want hash-a", h)
	}
	if h, _ := got.Hash("A/blocks/B"); h != "hash-b" {
		t.Errorf("relation hash=%q, want hash-b", h)
	}
	if got.Cursor != "42" {
		t.Errorf("cursor=%q, want 42", got.Cursor)
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

	// Shared baseline via a push.
	h.createLocalEntity(t, "TKT-7", map[string]any{"title": "base"})
	if _, err := h.engine.Push(ctx); err != nil {
		t.Fatalf("baseline push: %v", err)
	}
	cursorBefore := h.idx.Cursor

	// Remote edits TKT-7.
	h.server.mu.Lock()
	h.server.entities["TKT-7"].Properties["title"] = "remote"
	h.server.recordChange("e", "TKT-7", "ticket", false)
	h.server.mu.Unlock()

	// Local also edits TKT-7 (now dirty vs index).
	if err := h.st.UpdateEntity(ctx, &entity.Entity{ID: "TKT-7", Type: "ticket", Properties: map[string]any{"title": "local"}}); err != nil {
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
	got, _ := h.st.GetEntity(ctx, "TKT-7")
	if got.Properties["title"] != "local" {
		t.Fatalf("local clobbered: title=%v, want local", got.Properties["title"])
	}
	// Cursor unchanged → re-run revisits.
	if h.idx.Cursor != cursorBefore {
		t.Fatalf("cursor advanced past conflict: %q -> %q", cursorBefore, h.idx.Cursor)
	}

	// Force-pull resolves: remote wins.
	if _, err := h.engine.ForcePull(ctx, "TKT-7"); err != nil {
		t.Fatalf("force pull: %v", err)
	}
	got, _ = h.st.GetEntity(ctx, "TKT-7")
	if got.Properties["title"] != "remote" {
		t.Fatalf("force pull title=%v, want remote", got.Properties["title"])
	}
}
