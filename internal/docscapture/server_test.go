package docscapture

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// buildRoleAssignee inverts acl.yaml assignments to role→principal, prefers an
// update-capable role for the empty default, and maps a named role. Tested with
// a temp acl.yaml, no browser.
func TestBuildRoleAssignee(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	acl := `roles:
  editor:
    read: ["*"]
    create: ["*"]
    update: ["*"]
  viewer:
    read: ["*"]
assignments:
  alice@example.com: editor
  bob@example.com: viewer
`
	if err := os.WriteFile(filepath.Join(dir, "acl.yaml"), []byte(acl), 0o644); err != nil {
		t.Fatal(err)
	}
	assign := buildRoleAssignee(dir)

	// Named role → the user assigned it.
	if p := assign("viewer"); p.User != "bob@example.com" {
		t.Errorf("assign(viewer).User = %q, want bob@example.com", p.User)
	}
	if p := assign("editor"); p.User != "alice@example.com" {
		t.Errorf("assign(editor).User = %q, want alice@example.com", p.User)
	}
	// Empty default prefers an update-capable role (editor → alice).
	if p := assign(""); p.User != "alice@example.com" {
		t.Errorf("assign(\"\").User = %q, want the update-capable editor alice@example.com", p.User)
	}
	// Always stamps a real tool (an unstamped principal is rejected by a real ACL).
	if assign("editor").Tool != principal.ToolDataEntry {
		t.Error("principal must be stamped with the data-entry tool")
	}
}

// With no acl.yaml, any stamped placeholder user is fine (NopACL admits all).
func TestBuildRoleAssignee_NoPolicy(t *testing.T) {
	t.Parallel()
	assign := buildRoleAssignee(t.TempDir())
	if p := assign(""); p.User == "" {
		t.Error("a principal must always be stamped even without acl.yaml")
	}
}

// copyProjectSchema copies schema/config (not entities) into the temp project.
func TestCopyProjectSchema(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "metamodel.yaml"), "version: '1.0'\n")
	mustWrite(t, filepath.Join(src, "data-entry.yaml"), "name: X\n")
	mustWrite(t, filepath.Join(src, "entities", "ticket", "T-1.md"), "seed\n")

	dst := filepath.Join(t.TempDir(), "proj")
	if err := copyProjectSchema(src, dst); err != nil {
		t.Fatalf("copyProjectSchema: %v", err)
	}
	// Schema copied.
	if _, err := os.Stat(filepath.Join(dst, "metamodel.yaml")); err != nil {
		t.Error("metamodel.yaml should be copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "data-entry.yaml")); err != nil {
		t.Error("data-entry.yaml should be copied")
	}
	// Entities NOT copied (seeded separately).
	if _, err := os.Stat(filepath.Join(dst, "entities")); !os.IsNotExist(err) {
		t.Error("entities/ must NOT be copied — they are seeded separately")
	}
	// .rela cache dir created.
	if _, err := os.Stat(filepath.Join(dst, ".rela")); err != nil {
		t.Error(".rela cache dir should exist")
	}
}

// standUp builds a working server over a temp project + seed (no browser).
func TestStandUp_ServesSeededEntity(t *testing.T) {
	if err := checkSPA(); err != nil {
		t.Skip(err.Error())
	}
	p, err := standUp(context.Background(), protoDir(t), []docs.SeedOp{{
		Kind: "create", Type: "ticket", ID: "TICKET-su",
		Properties: map[string]any{"title": "x", "status": "open", "priority": "low", "reporter": "a@b.c"},
	}})
	if err != nil {
		t.Fatalf("standUp: %v", err)
	}
	defer p.close()

	// The server serves the SPA config with the stamped role — exercises standUp
	// + the per-request principal resolver without a browser. (An unstamped
	// principal would be rejected by the real Declarative ACL.)
	req, _ := http.NewRequest(http.MethodGet, p.server.URL+"/api/v1/_config", nil)
	req.Header.Set("Origin", p.server.URL)
	req.Header.Set(roleHeader, "editor")
	resp, err := p.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET _config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("_config GET = %d, want 200 (server standup + principal resolver)", resp.StatusCode)
	}
}

func TestHasChrome(t *testing.T) {
	t.Parallel()
	// Just exercise it; on CI without Chrome it returns false, which is fine.
	if path, ok := hasChrome(); ok && path == "" {
		t.Error("hasChrome reported found but no path")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// checkSPA skips server tests when the SPA isn't built (no browser needed for
// these, but the App requires the embedded SPA to construct its router).
func checkSPA() error {
	return dataentry.CheckEmbeddedSPA()
}
