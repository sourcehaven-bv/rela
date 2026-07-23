package docscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// fakeProject is a minimal docscli.Project: a metamodel plus a Paths.Root
// pointing at a temp dir (where acl.yaml would live, if any).
type fakeProject struct {
	meta  *metamodel.Metamodel
	paths *project.Context
}

func (p fakeProject) Meta() *metamodel.Metamodel { return p.meta }
func (p fakeProject) Paths() *project.Context    { return p.paths }

// newFakeProject builds a tiny metamodel with one entity type so typeref{}
// resolves, rooted at a temp dir with no acl.yaml (roles_matrix degrades).
func newFakeProject(t *testing.T) fakeProject {
	t.Helper()
	return fakeProject{
		meta: &metamodel.Metamodel{
			Description: "A test deployment.",
			Entities: map[string]metamodel.EntityDef{
				"risico": {
					Label:         "risico",
					PropertyOrder: []string{"titel", "kans"},
					Properties: map[string]metamodel.PropertyDef{
						"titel": {Type: "string", Required: true},
						"kans":  {Type: "integer", Required: true},
					},
				},
			},
		},
		paths: &project.Context{Root: t.TempDir()},
	}
}

func writeManual(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "manual.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write manual: %v", err)
	}
	return p
}

// stubNoCapturer forces the "no browser" path deterministically, independent of
// whether Chrome is installed on the host: newCapturer returns an error, so
// docs.Options.CapturerErr is set and a screenshot{} manual fails loud.
func stubNoCapturer(t *testing.T, reason string) {
	t.Helper()
	orig := newCapturer
	newCapturer = func() (docs.Capturer, error) { return nil, errors.New(reason) }
	t.Cleanup(func() { newCapturer = orig })
}

// stubCapturer installs a no-op capturer so a screenshot-free manual never
// touches a real browser regardless of the host.
type stubCapturer struct{}

func (stubCapturer) Capture(context.Context, docs.CaptureSpec) (string, error) { return "x.png", nil }
func (stubCapturer) Close() error                                              { return nil }

func stubOKCapturer(t *testing.T) {
	t.Helper()
	orig := newCapturer
	newCapturer = func() (docs.Capturer, error) { return stubCapturer{}, nil }
	t.Cleanup(func() { newCapturer = orig })
}

// A screenshot-free manual resolves to Markdown on stdout without ever
// constructing a browser capturer.
func TestBuild_ToStdout(t *testing.T) {
	stubOKCapturer(t)
	proj := newFakeProject(t)
	manual := writeManual(t, t.TempDir(),
		"# Manual\n\n```rela\ntyperef{ type = \"risico\", fields = \"required\" }\n```\n")

	cmd := &BuildCmd{Manual: manual}
	if err := cmd.Run(context.Background(), proj); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// acl.yaml present and valid → the policy flows into docs.Options and
// roles_matrix{} renders a role × verb table (not the "no policy" note).
func TestBuild_ACLPresent_RolesMatrixRenders(t *testing.T) {
	stubOKCapturer(t)
	proj := newFakeProject(t)
	acl := "roles:\n  editor:\n    read: [\"*\"]\n    create: [\"*\"]\n  viewer:\n    read: [\"*\"]\n"
	if err := os.WriteFile(filepath.Join(proj.Paths().Root, "acl.yaml"), []byte(acl), 0o644); err != nil {
		t.Fatalf("write acl.yaml: %v", err)
	}
	dir := t.TempDir()
	manual := writeManual(t, dir, "# Manual\n\n```rela\nroles_matrix{ type = \"risico\" }\n```\n")
	out := filepath.Join(dir, "out.md")

	cmd := &BuildCmd{Manual: manual, Output: out}
	if err := cmd.Run(context.Background(), proj); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(got), "No access policy defined") {
		t.Errorf("acl.yaml present but roles_matrix rendered the no-policy note:\n%s", got)
	}
	if !strings.Contains(string(got), "editor") || !strings.Contains(string(got), "viewer") {
		t.Errorf("expected both roles in the matrix, got:\n%s", got)
	}
}

// acl.yaml present but malformed → the build fails loud with a wrapped error,
// never silently ignoring the policy.
func TestBuild_ACLMalformed_FailsLoud(t *testing.T) {
	stubOKCapturer(t)
	proj := newFakeProject(t)
	// Structurally broken YAML (a mapping value where a key is expected).
	if err := os.WriteFile(filepath.Join(proj.Paths().Root, "acl.yaml"),
		[]byte("roles:\n  editor:\n  : : :\n"), 0o644); err != nil {
		t.Fatalf("write acl.yaml: %v", err)
	}
	manual := writeManual(t, t.TempDir(), "# Manual\n")
	cmd := &BuildCmd{Manual: manual}
	err := cmd.Run(context.Background(), proj)
	if err == nil {
		t.Fatal("expected a fail-loud error for malformed acl.yaml")
	}
	if !strings.Contains(err.Error(), "acl.yaml") {
		t.Errorf("error should mention acl.yaml, got: %v", err)
	}
}

// A screenshot{} manual with no browser available fails loud with the
// actionable reason — the "no graceful degradation" contract. Deterministic:
// the capturer seam forces the no-browser path regardless of the host.
func TestBuild_ScreenshotNoBrowser_FailsLoud(t *testing.T) {
	stubNoCapturer(t, "no Chrome/Chromium browser found on PATH")
	proj := newFakeProject(t)
	manual := writeManual(t, t.TempDir(),
		"# Manual\n\n```rela\nscreenshot{ type = \"risico\", entity = \"r1\", out = \"f.png\" }\n```\n")
	cmd := &BuildCmd{Manual: manual}
	err := cmd.Run(context.Background(), proj)
	if err == nil {
		t.Fatal("expected a fail-loud error for screenshot{} without a browser")
	}
	if !strings.Contains(err.Error(), "browser") {
		t.Errorf("error should carry the no-browser reason, got: %v", err)
	}
}

// The --out path writes resolved Markdown to a file, creating parent dirs.
func TestBuild_ToFile(t *testing.T) {
	stubOKCapturer(t)
	proj := newFakeProject(t)
	dir := t.TempDir()
	manual := writeManual(t, dir, "# Manual\n\n`rela description()`\n")
	out := filepath.Join(dir, "nested", "out.md")

	cmd := &BuildCmd{Manual: manual, Output: out}
	if err := cmd.Run(context.Background(), proj); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) == 0 {
		t.Error("output file is empty")
	}
}

// A missing manual path fails loud.
func TestBuild_MissingManual(t *testing.T) {
	t.Parallel()
	proj := newFakeProject(t)
	cmd := &BuildCmd{Manual: filepath.Join(t.TempDir(), "nope.md")}
	if err := cmd.Run(context.Background(), proj); err == nil {
		t.Fatal("expected an error for a missing manual")
	}
}

// --out pointing at an existing directory is rejected.
func TestBuild_OutputIsDir(t *testing.T) {
	t.Parallel()
	proj := newFakeProject(t)
	dir := t.TempDir()
	manual := writeManual(t, dir, "# Manual\n")
	cmd := &BuildCmd{Manual: manual, Output: dir}
	if err := cmd.Run(context.Background(), proj); err == nil {
		t.Fatal("expected an error when --out is a directory")
	}
}

func TestOutputDir(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                ".",
		"out.md":          ".",
		"site/manual.md":  "site",
		"a/b/c/manual.md": "a/b/c",
	}
	for in, want := range cases {
		if got := outputDir(in); got != want {
			t.Errorf("outputDir(%q) = %q, want %q", in, got, want)
		}
	}
}
