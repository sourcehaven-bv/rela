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
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
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

	ws, err := workspace.Discover(projectDir, script.NewEngine())
	if err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	app, err := NewApp(
		ws.FS(), ws.Paths(), ws.Meta(), ws.Store(),
		ws.EntityManager(), ws.Searcher(),
		ws.StartWatching,
	)
	if err != nil {
		t.Fatalf("creating app: %v", err)
	}

	cleanup := func() {
		// TempDir cleanup is automatic
	}

	return app, projectDir, cleanup
}

// TestE2E_MarkdownEditorSave tests that content typed in the EasyMDE editor
// is correctly saved when the form is submitted.
func TestE2E_MarkdownEditorSave(t *testing.T) {
	app, projectDir, cleanup := newE2ETestApp(t)
	defer cleanup()

	// Start test server
	server := httptest.NewServer(app.NewRouter())
	defer server.Close()

	// Set up headless Chrome
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout for the whole test
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Test content with unique marker
	testMarker := fmt.Sprintf("E2E-TEST-%d", time.Now().UnixNano())
	testContent := fmt.Sprintf("\n\n## Test Section\n\n%s", testMarker)

	var pageHTML string

	// Navigate to edit form for TKT-001, type in editor, save, verify
	err := chromedp.Run(ctx,
		// Navigate to the edit form
		chromedp.Navigate(server.URL+"/form/create_ticket/TKT-001"),

		// Wait for page to load and capture HTML for debugging
		chromedp.Sleep(1*time.Second),
		chromedp.OuterHTML("html", &pageHTML),
	)
	if err != nil {
		t.Fatalf("navigation failed: %v", err)
	}

	// Check if the editor element exists
	if !strings.Contains(pageHTML, "body-editor") {
		t.Logf("Page HTML (first 2000 chars):\n%s", pageHTML[:min(2000, len(pageHTML))])
		t.Fatal("body-editor element not found in page")
	}

	err = chromedp.Run(ctx,
		// Wait for EasyMDE to initialize (CodeMirror element appears)
		chromedp.WaitVisible(`.CodeMirror`, chromedp.ByQuery),

		// Type test content into CodeMirror
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				var editor = document.querySelector('.CodeMirror').CodeMirror;
				var content = editor.getValue();
				editor.setValue(content + %q);
				return true;
			})()
		`, testContent), nil),

		// Small delay to ensure change event fires
		chromedp.Sleep(100*time.Millisecond),

		// Click save button
		chromedp.Click(`button.btn-primary`, chromedp.ByQuery),

		// Wait for navigation/response
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("browser automation failed: %v", err)
	}

	// Verify the content was saved to the file
	savedData, err := os.ReadFile(filepath.Join(projectDir, "entities/tickets/TKT-001.md"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	if !strings.Contains(string(savedData), testMarker) {
		t.Errorf("saved content does not contain test marker %q\n\nSaved content:\n%s", testMarker, savedData)
	}
}

// TestE2E_FormFieldSubmit tests that regular form fields are correctly submitted.
func TestE2E_FormFieldSubmit(t *testing.T) {
	app, projectDir, cleanup := newE2ETestApp(t)
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

	// Update the title of an existing ticket
	newTitle := fmt.Sprintf("Updated Title %d", time.Now().UnixNano())

	err := chromedp.Run(ctx,
		// Navigate to edit form for TKT-001
		chromedp.Navigate(server.URL+"/form/create_ticket/TKT-001"),
		chromedp.Sleep(1*time.Second),

		// Wait for title input
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),

		// Clear and set new title
		chromedp.Clear(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="title"]`, newTitle, chromedp.ByQuery),

		// Submit form
		chromedp.Click(`button.btn-primary`, chromedp.ByQuery),

		// Wait for response
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		t.Fatalf("browser automation failed: %v", err)
	}

	// Verify the title was saved
	savedData, err := os.ReadFile(filepath.Join(projectDir, "entities/tickets/TKT-001.md"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	if !strings.Contains(string(savedData), newTitle) {
		t.Errorf("saved content does not contain new title %q\n\nSaved content:\n%s", newTitle, savedData)
	}
}

// TestE2E_LuaDocumentRenders verifies the full path for a Lua-rendered
// document in a headless browser: navigate to the entity detail page,
// wait for DocumentsPanel to fetch /api/v1/_documents/..., and assert
// the rendered HTML contains markers produced by the Lua script plus
// a rewritten form link. Exercises the full chain:
//
//   HTTP handler → documentService.Render → script.Engine.ExecuteDocument
//   → Lua VM (with rela.document + rela.url bindings) → goldmark → link
//   rewriter (adds return_to to form routes) → DOMPurify in browser
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
