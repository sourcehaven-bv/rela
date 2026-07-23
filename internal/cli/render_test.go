package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// setupRenderGraph builds a graph with an identity ("cp {in} {out}") transform
// so render tests need no external converter like pandoc.
func setupRenderGraph(t *testing.T) *readServices {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefix: "TKT-"},
			"feature": {Label: "Feature", IDPrefix: "FEAT-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {Label: "implements", From: []string{"ticket"}, To: []string{"feature"}},
		},
		Transforms: map[string]metamodel.TransformDef{
			"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "ticket").
		ID("TKT-001").
		With("title", "Do the thing").
		With("status", "in-progress"))
	seeder.addEntity(testutil.EntityFor(meta, "feature").
		ID("FEAT-001").
		With("title", "The Feature"))
	seeder.addRelation("TKT-001", "implements", "FEAT-001")
	return seeder.build(t).read
}

func TestRenderCmd_WritesRenderedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX cp fixture")
	}
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
	svc := setupRenderGraph(t)
	out := filepath.Join(t.TempDir(), "ticket.txt")

	cmd := &RenderCmd{ID: "TKT-001", Transform: "copy", Out: out}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("render: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		"# Do the thing", // title as H1
		"| status | in-progress |",
		"## implements", // relation group (uses relation label)
		"- The Feature", // neighbor display title
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered output missing %q\n---\n%s", want, s)
		}
	}
}

func TestRenderCmd_UnknownTransform(t *testing.T) {
	svc := setupRenderGraph(t)
	cmd := &RenderCmd{ID: "TKT-001", Transform: "nope", Out: filepath.Join(t.TempDir(), "x")}
	err := cmd.Run(context.Background(), svc)
	if err == nil || !strings.Contains(err.Error(), "unknown transform") {
		t.Fatalf("want unknown-transform error, got %v", err)
	}
}

func TestRenderCmd_MissingEntity(t *testing.T) {
	svc := setupRenderGraph(t)
	cmd := &RenderCmd{ID: "TKT-999", Transform: "copy", Out: filepath.Join(t.TempDir(), "x")}
	if err := cmd.Run(context.Background(), svc); err == nil {
		t.Fatal("want error for missing entity")
	}
}
