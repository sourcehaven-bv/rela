package projectsetup_test

import (
	"path/filepath"
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
		`<script src="_rela.js">`, // bridge SDK wired
		`href="_rela.css"`,        // theme opt-in
		`name="rela-app:label"`,   // metadata stub
		`rela.list(`,              // a working bridge call
		`window.addEventListener('rela:ready'`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("starter index.html missing %q", want)
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
