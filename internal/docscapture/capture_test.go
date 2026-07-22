package docscapture

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

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
