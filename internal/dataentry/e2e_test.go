//go:build e2e

// E2E tests for the data entry web application.
// These tests use chromedp to automate a headless Chrome browser.
//
// Requirements:
//   - Chrome or Chromium must be installed
//   - The prototype project must exist at prototypes/data-entry/project
//
// Run with: go test -tags=e2e ./internal/dataentry/...

package dataentry

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// newE2ETestApp creates a full App using a copy of the prototype project for E2E tests.
// Returns the App, the temp directory path, and a cleanup function.
func newE2ETestApp(t *testing.T) (*App, string, func()) {
	t.Helper()

	// Use the prototype project as the test fixture
	protoPath := "../../prototypes/data-entry/project"
	if _, err := os.Stat(protoPath); os.IsNotExist(err) {
		t.Skip("prototype project not found, skipping E2E test")
	}

	// Copy prototype to a temp directory so tests don't modify the original
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")

	// Use cp -r for simplicity
	cmd := exec.Command("cp", "-r", protoPath, projectDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("copying prototype: %v", err)
	}

	_ = os.MkdirAll(filepath.Join(projectDir, ".rela"), 0o755)

	svc, err := appbuild.Discover(projectDir, script.NewEngine())
	if err != nil {
		t.Fatalf("appbuild.Discover: %v", err)
	}
	app, err := NewApp(
		svc.FS(), svc.Paths(), svc.Meta(), svc.Store(),
		svc.EntityManager(), svc.Searcher(), svc.ACL(),
		NopFieldVerdictResolver{},
		svc.Audit(),
	)
	if err != nil {
		svc.Close()
		t.Fatalf("creating app: %v", err)
	}

	cleanup := func() {
		svc.Close()
	}

	return app, projectDir, cleanup
}

// TestE2E_LuaDocumentRenders verifies the full path for a Lua-rendered
// document in a headless browser: navigate to the entity detail page,
// wait for DocumentsPanel to fetch /api/v1/_documents/..., and assert
// the rendered HTML contains markers produced by the Lua script plus
// a rewritten form link. Exercises the full chain:
//
//	HTTP handler → documentService.Render → script.Engine.ExecuteDocument
//	→ Lua VM (with rela.document + rela.url bindings) → goldmark → link
//	rewriter (adds return_to to form routes) → DOMPurify in browser
//
// The prototype ships `scripts/docs/category_report.lua` wired as the
// `category_overview` document for entity_type `category`. The `backend`
// category has multiple belongs-to tickets, so the rendered output must
// contain both a ticket-table row and an /form/edit_ticket/... link
// produced via rela.url in the script.
func TestE2E_LuaDocumentRenders(t *testing.T) {
	app, _, cleanup := newE2ETestApp(t)
	defer cleanup()

	server := httptest.NewServer(app.NewRouter())
	defer server.Close()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var panelHTML string
	err := chromedp.Run(ctx,
		chromedp.Navigate(server.URL+"/entity/category/backend"),

		// DocumentsPanel renders asynchronously after the initial page
		// load, once it fetches the rendered HTML. Wait for the body
		// element specifically (not just the panel container) so we
		// know render has completed.
		chromedp.WaitVisible(`.documents-panel .document-body`, chromedp.ByQuery),

		// Small buffer for v-html + DOMPurify to actually paint.
		chromedp.Sleep(300*time.Millisecond),

		chromedp.OuterHTML(`.documents-panel .document-body`, &panelHTML),
	)
	if err != nil {
		t.Fatalf("browser automation failed: %v\n\nlast panel HTML:\n%s", err, panelHTML)
	}

	// The Lua script always prints "## Tickets (N)" as a section header,
	// independent of N — so this assertion survives prototype fixture
	// changes (adding/removing tickets or belongs-to relations for the
	// `backend` category). If the header stops appearing entirely, the
	// Lua render itself is broken.
	if !strings.Contains(panelHTML, "Tickets (") {
		t.Errorf("expected rendered doc to contain 'Tickets (' header, got:\n%s", panelHTML)
	}

	// The script builds /form/edit_ticket/<id> links via rela.url. The
	// data-entry layer then appends return_to so the submit navigation
	// returns to the category page. Finding an /form/edit_ticket/ href
	// with a return_to query proves the full chain (catalog-verified
	// URL in Lua → goldmark → link rewriter) worked end-to-end.
	if !strings.Contains(panelHTML, "/form/edit_ticket/") {
		t.Errorf("expected /form/edit_ticket/... link in panel, got:\n%s", panelHTML)
	}
	if !strings.Contains(panelHTML, "return_to=") {
		t.Errorf("expected return_to query on rewritten form link, got:\n%s", panelHTML)
	}
}
