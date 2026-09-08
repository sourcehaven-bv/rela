package projectsetup_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// newProjectFS returns a memfs with an initialized rela project rooted at the
// returned root dir, so project.Discover finds a root.
func newProjectFS(t *testing.T) (fs storage.FS, root string) {
	t.Helper()
	fs = storage.NewMemFS()
	root = "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init project: %v", err)
	}
	return fs, root
}

func TestScaffoldApp_CreatesWiredUpApp(t *testing.T) {
	fs, root := newProjectFS(t)

	res, err := projectsetup.ScaffoldAppWithFS(root, "my-app", fs)
	if err != nil {
		t.Fatalf("ScaffoldApp: %v", err)
	}
	if res.ID != "my-app" {
		t.Errorf("ID = %q", res.ID)
	}
	wantIndex := filepath.Join(root, project.AppsDir, "my-app", "index.html")
	if res.IndexAbs != wantIndex {
		t.Errorf("IndexAbs = %q, want %q", res.IndexAbs, wantIndex)
	}

	html, err := fs.ReadFile(wantIndex)
	if err != nil {
		t.Fatalf("index.html not written: %v", err)
	}
	body := string(html)
	for _, want := range []string{
		`name="rela-app:bridge-version" content="1"`, // required version (server rejects without it)
		`<script src="_rela.js">`,                    // bridge SDK wired
		`href="_rela.css"`,                           // theme opt-in
		`name="rela-app:label"`,                      // metadata stub
		`href="app.css"`,                             // own styles, as a file
		`<script src="app.js">`,                      // own code, as a file
	} {
		if !strings.Contains(body, want) {
			t.Errorf("starter index.html missing %q", want)
		}
	}

	// The two asset files exist and carry the behavior that used to be inline.
	css, err := fs.ReadFile(res.CSSAbs)
	if err != nil {
		t.Fatalf("app.css not written: %v", err)
	}
	if !strings.Contains(string(css), "var(--text-color)") {
		t.Error("starter app.css should use rela's theme tokens")
	}
	js, err := fs.ReadFile(res.JSAbs)
	if err != nil {
		t.Fatalf("app.js not written: %v", err)
	}
	for _, want := range []string{`rela.list(`, `window.addEventListener('rela:ready'`} {
		if !strings.Contains(string(js), want) {
			t.Errorf("starter app.js missing %q", want)
		}
	}
}

// TestScaffoldApp_NoInlineCodeOrStyles is the guard that keeps `rela apps new`
// working out of the box. The app CSP carries no 'unsafe-inline'
// (dataentry.appCSP), so an inline <script>, <style> or style="" attribute in
// the scaffold would be BLOCKED — the generated app would render unstyled and
// do nothing, before the author writes a line of their own code. Nothing else
// fails in that case: the files are written, the server serves them, and the
// breakage only shows as a console violation in a browser.
var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

func TestScaffoldApp_NoInlineCodeOrStyles(t *testing.T) {
	fs, root := newProjectFS(t)

	res, err := projectsetup.ScaffoldAppWithFS(root, "inline-check", fs)
	if err != nil {
		t.Fatalf("ScaffoldApp: %v", err)
	}
	html, err := fs.ReadFile(res.IndexAbs)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	body := string(html)

	// An opening <style>/<script> tag with no src= is an inline block. Strip
	// HTML comments first: the template deliberately EXPLAINS this rule in a
	// comment that names both tags, and a raw substring check would match that
	// prose and fail on the very text telling authors to avoid inlining.
	markup := htmlCommentRe.ReplaceAllString(body, "")
	for _, bad := range []string{"<style", "<script>"} {
		if strings.Contains(markup, bad) {
			t.Errorf("scaffolded index.html must not contain an inline %s block: the app CSP blocks it", bad)
		}
	}
	if strings.Contains(body, `style="`) {
		t.Error(`scaffolded index.html must not use style="" attributes: blocked without 'unsafe-hashes'; use a class in app.css`)
	}
	// on*= inline event handlers are blocked by script-src-attr for the same reason.
	for _, bad := range []string{"onclick=", "onload=", "onerror="} {
		if strings.Contains(body, bad) {
			t.Errorf("scaffolded index.html must not use an inline %s handler: the app CSP blocks it", bad)
		}
	}
}

func TestScaffoldApp_RejectsInvalidID(t *testing.T) {
	fs, root := newProjectFS(t)
	for _, bad := range []string{"Bad Id", "UPPER", "has/slash", "has.dot", "", strings.Repeat("a", 65)} {
		if _, err := projectsetup.ScaffoldAppWithFS(root, bad, fs); err == nil {
			t.Errorf("ScaffoldApp(%q) = nil error, want rejection", bad)
		}
	}
}

func TestScaffoldApp_RejectsDuplicate(t *testing.T) {
	fs, root := newProjectFS(t)
	if _, err := projectsetup.ScaffoldAppWithFS(root, "dash", fs); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	_, err := projectsetup.ScaffoldAppWithFS(root, "dash", fs)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate scaffold err = %v, want 'already exists'", err)
	}
}

func TestScaffoldApp_RejectsNoProject(t *testing.T) {
	fs := storage.NewMemFS() // no project initialized
	if _, err := projectsetup.ScaffoldAppWithFS("/nowhere", "dash", fs); err == nil {
		t.Error("expected error when no rela project is found")
	}
}
