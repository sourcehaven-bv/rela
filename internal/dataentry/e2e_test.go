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

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
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

	ctx := &project.Context{
		Root:          projectDir,
		MetamodelPath: filepath.Join(projectDir, "metamodel.yaml"),
		EntitiesDir:   filepath.Join(projectDir, "entities"),
		RelationsDir:  filepath.Join(projectDir, "relations"),
		CacheDir:      filepath.Join(projectDir, ".rela"),
	}
	_ = os.MkdirAll(ctx.CacheDir, 0o755)

	fs := storage.NewSafeFS(storage.NewOsFS())
	repo := repository.New(fs, ctx)
	ws, err := workspace.New(repo, workspace.NopScriptExecutor)
	if err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	app, err := NewApp(ws)
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
