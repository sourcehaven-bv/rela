package dataentry

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Sourcehaven-BV/rela/internal/rootfs"
)

// TestStaticShell_ServesOnlyEmbeddedBytes pins that the tenant-free shell
// answers every path from the binary: the shell for a client route and for an
// API path alike, never a handler that could read a store.
func TestStaticShell_ServesOnlyEmbeddedBytes(t *testing.T) {
	if err := CheckEmbeddedSPA(); err != nil {
		t.Skipf("embedded SPA not built: %v", err)
	}
	h := StaticShell(ShellBoot{})
	shell := httptest.NewRecorder()
	h.ServeHTTP(shell, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if shell.Code != http.StatusOK || shell.Body.Len() == 0 {
		t.Fatalf("GET / = %d with %d bytes, want the shell", shell.Code, shell.Body.Len())
	}
	for _, p := range []string{"/tickets/TKT-1", "/api/v1/tickets"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, http.NoBody))
		if w.Code != http.StatusOK || w.Body.String() != shell.Body.String() {
			t.Errorf("GET %s = %d, want the shell bytes", p, w.Code)
		}
	}
}

// TestShellBootFrom reads app.name and app.language, and drops a language
// that is not a tag (BUG-21N3J4N).
func TestShellBootFrom(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want ShellBoot
	}{
		{"both", "app:\n  name: Sourcehaven\n  language: en\n", ShellBoot{AppName: "Sourcehaven", Language: "en"}},
		{"region", "app:\n  language: nl-BE\n", ShellBoot{Language: "nl-BE"}},
		{"no app block", "lists: {}\n", ShellBoot{}},
		{"bad language", "app:\n  name: X\n  language: '\"><script>'\n", ShellBoot{AppName: "X"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ShellBootFrom([]byte(tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
	if _, err := ShellBootFrom([]byte("app: [")); err == nil {
		t.Error("invalid YAML: want an error")
	}
}

const bootTestShell = `<!DOCTYPE html>
<html lang="en">
  <head>
    <title>rela</title>
  </head>
  <body></body>
</html>`

// TestBootShell pins the title, lang and meta rewrite, and that the app name
// is escaped.
func TestBootShell(t *testing.T) {
	got := string(bootShell([]byte(bootTestShell), ShellBoot{AppName: `A & <b>"B"</b>`, Language: "nl"}))
	for _, want := range []string{
		`<html lang="nl">`,
		`<title>A &amp; &lt;b&gt;&#34;B&#34;&lt;/b&gt;</title>`,
		`<meta name="rela-app-name" content="A &amp; &lt;b&gt;&#34;B&#34;&lt;/b&gt;" />`,
		`<meta name="rela-language" content="nl" />`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("shell lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<b>") {
		t.Errorf("app name not escaped:\n%s", got)
	}
	if plain := string(bootShell([]byte(bootTestShell), ShellBoot{})); plain != bootTestShell {
		t.Errorf("zero boot changed the shell:\n%s", plain)
	}
}

// TestBootShellHandler serves the rewritten shell for client routes and
// leaves real files alone.
func TestBootShellHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte(bootTestShell)},
		"app.js":     {Data: []byte("js")},
	}
	h := bootShellHandler(fsys, ShellBoot{AppName: "Sourcehaven"})
	for _, p := range []string{"/", "/s/portal"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, http.NoBody))
		if !strings.Contains(w.Body.String(), "<title>Sourcehaven</title>") {
			t.Errorf("GET %s: %d %q, want the named shell", p, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/", http.NoBody))
	if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Errorf("HEAD / = %d with %d bytes", w.Code, w.Body.Len())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/app.js", http.NoBody))
	if w.Body.String() != "js" {
		t.Errorf("GET /app.js = %q, want the file", w.Body.String())
	}
}

// TestLoadShellBoot treats a missing data-entry.yaml as no boot values and
// passes other read errors on.
func TestLoadShellBoot(t *testing.T) {
	load := func(data []byte, err error) func(context.Context, string) ([]byte, error) {
		return func(_ context.Context, name string) ([]byte, error) {
			if name != "data-entry.yaml" {
				t.Errorf("read %q, want data-entry.yaml", name)
			}
			return data, err
		}
	}
	got, err := LoadShellBoot(t.Context(), load([]byte("app:\n  name: N\n"), nil))
	if err != nil || got != (ShellBoot{AppName: "N"}) {
		t.Errorf("file: %+v, %v", got, err)
	}
	got, err = LoadShellBoot(t.Context(), load(nil, fs.ErrNotExist))
	if err != nil || got != (ShellBoot{}) {
		t.Errorf("missing: %+v, %v", got, err)
	}
	if _, err = LoadShellBoot(t.Context(), load(nil, errors.New("boom"))); err == nil {
		t.Error("read error: want an error")
	}
}

// TestSPAHandlerWithCustom_WritesBootPerRequest pins that a single-tenant
// server's shell names the app and its language from the current config, read
// per request. A screen shown before `/_config` loads, such as a refused
// request, then follows `app.language` and not the browser.
func TestSPAHandlerWithCustom_WritesBootPerRequest(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{"index.html": {Data: []byte(testShell)}, "app.js": {Data: []byte("js")}}
	custom := newCustomAssets(rootfs.New(t.TempDir()), []byte(testShell), func() bool { return true })
	boot := ShellBoot{AppName: "Workspace", Language: "en"}
	h := spaHandlerWithCustom(fsys, custom, func() ShellBoot { return boot })

	get := func(path string) string {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		return w.Body.String()
	}
	if got := get("/s/portal"); !strings.Contains(got, `<meta name="rela-language" content="en" />`) ||
		!strings.Contains(got, `<meta name="rela-app-name" content="Workspace" />`) {

		t.Errorf("shell does not carry the boot values:\n%s", got)
	}
	boot = ShellBoot{Language: "nl"}
	if got := get("/"); !strings.Contains(got, `content="nl"`) || strings.Contains(got, "Workspace") {
		t.Errorf("a config reload did not reach the shell:\n%s", got)
	}
	if got := get("/app.js"); got != "js" {
		t.Errorf("an asset was rewritten: %q", got)
	}
}
