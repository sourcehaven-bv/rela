package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

func TestParseContextTarget(t *testing.T) {
	encode := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return " " + url.PathEscape(string(b))
	}
	tests := []struct {
		name string
		data string
		want contextTarget
		ok   bool
	}{
		{
			name: "link with base and title",
			data: encode(contextTarget{Path: "/entity/doc/DOC-1", Base: "/p/abc/", Title: "First"}),
			want: contextTarget{Path: "/entity/doc/DOC-1", Base: "/p/abc/", Title: "First"},
			ok:   true,
		},
		{name: "empty", data: "", ok: false},
		{name: "not json", data: url.PathEscape("nope"), ok: false},
		{name: "bad escape", data: "%zz", ok: false},
		{name: "no path", data: encode(map[string]string{"title": "x"}), ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseContextTarget(tc.data)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseContextTarget = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestEntityIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/entity/doc/DOC-1", "DOC-1", true},
		{"/s/crm/entity/doc/DOC-1", "DOC-1", true},
		{"/p/abc/s/crm/entity/doc/DOC-1?tab=x", "DOC-1", true},
		{"/entity/doc/A%20B", "A B", true},
		{"/entity/doc", "", false},
		{"/entity/doc/DOC-1/extra", "", false},
		{"/list/docs", "", false},
		{"/entity/doc/%zz", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got, ok := entityIDFromPath(tc.path)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("entityIDFromPath(%q) = %q, %v; want %q, %v", tc.path, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestShellCommandJS_EncodesArgument(t *testing.T) {
	arg := `"});alert(1);//`
	js := shellCommandJS(shellCommand{Command: "space", Arg: arg})
	const prefix, suffix = `window.dispatchEvent(new CustomEvent("rela:shell-command",{detail:`, `}))`
	if !strings.HasPrefix(js, prefix) || !strings.HasSuffix(js, suffix) {
		t.Fatalf("unexpected script: %s", js)
	}
	// The detail must be one JSON value that carries the argument intact.
	var got shellCommand
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(js, prefix), suffix)), &got); err != nil {
		t.Fatalf("detail is not one JSON value: %v (%s)", err, js)
	}
	if got.Arg != arg || got.Command != "space" {
		t.Fatalf("detail = %+v", got)
	}
}

type fakeLoader map[string]string

func (f fakeLoader) Load(_ context.Context, name string) ([]byte, error) {
	if v, ok := f[name]; ok {
		return []byte(v), nil
	}
	return nil, fs.ErrNotExist
}

type failingLoader struct{}

func (failingLoader) Load(context.Context, string) ([]byte, error) { return nil, errors.New("boom") }

func TestProjectSpaces(t *testing.T) {
	tests := []struct {
		name   string
		loader configLoader
		want   []menuSpace
	}{
		{name: "no loader", loader: nil},
		{name: "no file", loader: fakeLoader{}},
		{name: "read error", loader: failingLoader{}},
		{name: "bad yaml", loader: fakeLoader{"data-entry.yaml": "spaces: ["}},
		{
			name:   "spaces in order, unnamed dropped",
			loader: fakeLoader{"data-entry.yaml": "spaces:\n  - id: a\n    label: Alpha\n  - label: none\n  - id: b\n"},
			want:   []menuSpace{{ID: "a", Label: "Alpha"}, {ID: "b"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := projectSpaces(context.Background(), tc.loader)
			if len(got) != len(tc.want) {
				t.Fatalf("projectSpaces = %+v; want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("projectSpaces[%d] = %+v; want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSpaceLabel(t *testing.T) {
	if got := spaceLabel(menuSpace{ID: "crm", Label: "  "}); got != "crm" {
		t.Fatalf("spaceLabel fallback = %q", got)
	}
	if got := spaceLabel(menuSpace{ID: "crm", Label: "CRM"}); got != "CRM" {
		t.Fatalf("spaceLabel = %q", got)
	}
}

func TestProjectOfBase(t *testing.T) {
	if got := projectOfBase("/p/abc123/entity/doc/DOC-1"); got != "abc123" {
		t.Fatalf("projectOfBase = %q", got)
	}
	if got := projectOfBase("/entity/doc/DOC-1"); got != "" {
		t.Fatalf("projectOfBase root = %q", got)
	}
}

// The chrome style targets the component library's class names. A rename
// there would leave windows that cannot be dragged, with nothing failing, so
// this checks each class against the stylesheets the binary actually embeds.
func TestChromeStyle_TargetsShippedClasses(t *testing.T) {
	if goruntime.GOOS != "darwin" {
		if chromeStyle != "" {
			t.Fatal("chrome style must be empty off macOS")
		}
		t.Skip("chrome style is macOS-only")
	}
	css := embeddedCSS(t)
	for _, class := range []string{"rl-sidebar__header", "rl-page-header", "rl-page-header__actions", "rl-sidebar"} {
		if !strings.Contains(chromeStyle, "."+class) {
			t.Fatalf("chrome style no longer targets .%s; update this list", class)
		}
		if !strings.Contains(css, "."+class) {
			t.Errorf("embedded SPA has no .%s; the desktop chrome style targets it", class)
		}
	}
	if !strings.Contains(css, "--rl-sidebar-safe-top") {
		t.Error("embedded SPA no longer reads --rl-sidebar-safe-top; traffic lights would cover the sidebar")
	}
}

func embeddedCSS(t *testing.T) string {
	t.Helper()
	// The directory internal/dataentry embeds; the build fails without it.
	assets := os.DirFS(filepath.Join("..", "..", "internal", "dataentry", "static"))
	var b strings.Builder
	err := fs.WalkDir(assets, ".", func(p string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() || !strings.HasSuffix(p, ".css") {
			return err
		}
		data, err := fs.ReadFile(assets, p)
		b.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Len() == 0 {
		t.Fatal("no stylesheets in the embedded SPA")
	}
	return b.String()
}
