package docscapture

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// requireBrowser skips the test when a browser or the built SPA is absent, so
// the standard CI matrix stays green; the dedicated browser job runs them.
func requireBrowser(t *testing.T) {
	t.Helper()
	if _, ok := hasChrome(); !ok {
		t.Skip("no Chrome/Chromium browser available")
	}
	if err := dataentry.CheckEmbeddedSPA(); err != nil {
		t.Skip("data-entry SPA not built (run just build-frontend)")
	}
}

// protoDir is the in-tree prototype project used as the capture corpus.
func protoDir(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../prototypes/data-entry/project")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p, "metamodel.yaml")); err != nil {
		t.Skip("prototype project not found")
	}
	return p
}

// A seeded ticket's edit form captures to a valid PNG.
func TestCapture_Form(t *testing.T) {
	requireBrowser(t)
	capr, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer capr.Close()

	out := filepath.Join(t.TempDir(), "form.png")
	spec := docs.CaptureSpec{
		ProjectDir: protoDir(t),
		Seed: []docs.SeedOp{{
			Kind: "create", Type: "ticket", ID: "TICKET-cap",
			Properties: map[string]any{
				"title": "Login 500s under load", "status": "in-progress",
				"priority": "high", "reporter": "cap@example.com",
			},
		}},
		View: "form", Type: "ticket", Entity: "TICKET-cap",
		OutPath: out,
	}
	png, err := capr.Capture(context.Background(), spec)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	assertPNG(t, png)
}

// An arrow annotation anchored to a real field succeeds; an unknown field fails.
func TestCapture_AnnotationAndFailLoud(t *testing.T) {
	requireBrowser(t)
	capr, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer capr.Close()

	seed := []docs.SeedOp{{
		Kind: "create", Type: "ticket", ID: "TICKET-anno",
		Properties: map[string]any{
			"title": "x", "status": "open", "priority": "low", "reporter": "a@b.c",
		},
	}}
	base := docs.CaptureSpec{
		ProjectDir: protoDir(t), Seed: seed,
		View: "form", Type: "ticket", Entity: "TICKET-anno",
	}

	// Valid field anchor → success.
	ok := base
	ok.OutPath = filepath.Join(t.TempDir(), "anno.png")
	ok.Arrows = []docs.Annotation{{At: "status", Text: "the lifecycle state"}}
	if _, err := capr.Capture(context.Background(), ok); err != nil {
		t.Fatalf("annotated capture: %v", err)
	}
	assertPNG(t, ok.OutPath)

	// Unknown field anchor → fail loud.
	bad := base
	bad.OutPath = filepath.Join(t.TempDir(), "bad.png")
	bad.Arrows = []docs.Annotation{{At: "nosuchfield", Text: "x"}}
	if _, err := capr.Capture(context.Background(), bad); err == nil {
		t.Error("expected fail-loud on an unknown annotation anchor")
	}
}

// Two screenshots on ONE reused Capturer where the second entity is seeded
// AFTER the first capture (the seed grows across islands) — the reused server
// must pick up the new entity (regression for the seed-staleness bug).
func TestCapture_SeedGrowsAcrossIslands(t *testing.T) {
	requireBrowser(t)
	capr, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer capr.Close()

	mk := func(id string) docs.SeedOp {
		return docs.SeedOp{Kind: "create", Type: "ticket", ID: id, Properties: map[string]any{
			"title": id, "status": "open", "priority": "low", "reporter": "a@b.c",
		}}
	}
	// First island: seed = [t1], capture t1 (stands up the server).
	seed := []docs.SeedOp{mk("TICKET-i1")}
	if _, err := capr.Capture(context.Background(), docs.CaptureSpec{
		ProjectDir: protoDir(t), Seed: seed, View: "form", Type: "ticket",
		Entity: "TICKET-i1", OutPath: filepath.Join(t.TempDir(), "1.png"),
	}); err != nil {
		t.Fatalf("island 1: %v", err)
	}
	// Second island: a new create() grew the seed to [t1, t2]; capture t2 against
	// the ALREADY-RUNNING server. Without syncSeed this fails "entity not found".
	seed = append(seed, mk("TICKET-i2"))
	out2 := filepath.Join(t.TempDir(), "2.png")
	if _, err := capr.Capture(context.Background(), docs.CaptureSpec{
		ProjectDir: protoDir(t), Seed: seed, View: "form", Type: "ticket",
		Entity: "TICKET-i2", OutPath: out2,
	}); err != nil {
		t.Fatalf("island 2 (seed grew): %v", err)
	}
	assertPNG(t, out2)
}

// Capturing an entity that isn't in the seed must fail loud via the
// renderability gate (form-state=error) — NOT capture a blank schema-only form
// and NOT eat the capture timeout (DR-S4 / the fail-OPEN hole the spike hit).
func TestCapture_UnrenderableEntity_FailsLoud(t *testing.T) {
	requireBrowser(t)
	capr, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer capr.Close()

	// Seed nothing of this id; ask to capture it.
	start := time.Now()
	_, err = capr.Capture(context.Background(), docs.CaptureSpec{
		ProjectDir: protoDir(t), View: "form", Type: "ticket",
		Entity: "TICKET-does-not-exist", OutPath: filepath.Join(t.TempDir(), "x.png"),
	})
	if err == nil {
		t.Fatal("expected a fail-loud error for an unrenderable entity")
	}
	if !strings.Contains(err.Error(), "failed to load") {
		t.Errorf("error should name the load failure, got: %v", err)
	}
	// It must short-circuit, not run out the full per-capture timeout.
	if elapsed := time.Since(start); elapsed > perCaptureTimeout-time.Second {
		t.Errorf("gate ate the timeout (%s) instead of short-circuiting", elapsed)
	}
}

func assertPNG(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png: %v", err)
	}
	if len(data) < 1000 {
		t.Errorf("png suspiciously small: %d bytes", len(data))
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Errorf("not a PNG (bad magic)")
	}
}
