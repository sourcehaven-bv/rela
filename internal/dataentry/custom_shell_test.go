package dataentry

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestTokensCSSNeverLayered pins DECISION 1 of TKT-3DBK6I: the `:root` token
// declarations must NEVER be wrapped in `@layer`, on either side of the
// SPA/app boundary.
//
// tokens.css is served into two different cascade environments: the SPA
// (alongside the rest of rela's CSS, which IS layered) and custom-app iframes
// as `_rela.css` (where there is no other rela CSS at all). Layering it would
// not order the tokens against anything inside an iframe — it would merely
// demote them beneath every unlayered rule the app author writes, weakening
// the contract in exactly the place it exists to serve.
//
// TestAppTokensCSSInSyncWithFrontend asserts the two copies are byte-identical;
// this asserts that neither is layered, so "identical" also means "behaves
// identically". Without this, wrapping the SPA copy at build time would leave
// that test green while silently voiding its guarantee.
func TestTokensCSSNeverLayered(t *testing.T) {
	frontend, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "styles", "tokens.css"))
	if err != nil {
		t.Fatalf("read frontend tokens.css: %v", err)
	}
	for _, tc := range []struct{ name, css string }{
		{"frontend/src/styles/tokens.css", string(frontend)},
		{"internal/dataentry/apps_tokens.css", appTokensCSS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.css, "@layer") {
				t.Errorf("%s contains @layer — token declarations must stay unlayered "+
					"so they behave identically in the SPA and in custom-app iframes "+
					"(see TKT-3DBK6I DECISION 1 / RR-XOTMPN)", tc.name)
			}
		})
	}
}

// embeddedSPAShell reads the embedded index.html. Tests skip when the SPA is
// not built (matching TestAppEditorBundleEmbedded's convention).
func embeddedSPAShell() ([]byte, error) {
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(spaFS, spaIndexFile)
}

// TestSPAShellInjection exercises the real router: a stock deployment's shell
// must be byte-identical to the embedded one, and an injected shell must carry
// a correct Content-Length (http.FileServer's headers describe the ORIGINAL
// bytes, which we no longer serve).
func TestSPAShellInjection(t *testing.T) {
	embedded, err := embeddedSPAShell()
	if err != nil {
		t.Skipf("embedded SPA not built: %v", err)
	}

	tests := []struct {
		name     string
		files    []string
		wantCSS  bool
		wantJS   bool
		disabled bool
	}{
		{name: "stock deployment is byte-identical"},
		{name: "css only", files: []string{customCSSFile}, wantCSS: true},
		{name: "js only", files: []string{customJSFile}, wantJS: true},
		{name: "both", files: []string{customCSSFile, customJSFile}, wantCSS: true, wantJS: true},
		{name: "disabled serves stock shell", files: []string{customCSSFile, customJSFile}, disabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			app.broker = newEventBroker()
			root := t.TempDir()
			app.paths.Root = root
			for _, f := range tt.files {
				writeCustom(t, root, f, "/* x */")
			}
			if tt.disabled {
				app.State().Cfg.App.DisableCustomInjection = true
			}

			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()

			if gotCSS := strings.Contains(body, customCSSTag); gotCSS != tt.wantCSS {
				t.Errorf("css tag present = %v, want %v", gotCSS, tt.wantCSS)
			}
			if gotJS := strings.Contains(body, customJSTag); gotJS != tt.wantJS {
				t.Errorf("js tag present = %v, want %v", gotJS, tt.wantJS)
			}

			// The strongest form of "no injection".
			if !tt.wantCSS && !tt.wantJS && body != string(embedded) {
				t.Errorf("shell differs from the embedded original with no injection expected")
			}

			// Content-Length must describe what we actually wrote.
			if cl := rec.Header().Get("Content-Length"); cl != "" {
				if cl != strconv.Itoa(len(body)) {
					t.Errorf("Content-Length = %s, want %d", cl, len(body))
				}
			}
		})
	}
}

// TestSPAShellInjection_ClientRouteAlsoInjected pins that a deep client-side
// route (which falls through to the shell) gets the same treatment as "/".
// Without this, a customisation would apply on the dashboard but vanish on a
// hard refresh of any other route.
func TestSPAShellInjection_ClientRouteAlsoInjected(t *testing.T) {
	if _, err := embeddedSPAShell(); err != nil {
		t.Skipf("embedded SPA not built: %v", err)
	}
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()
	root := t.TempDir()
	app.paths.Root = root
	writeCustom(t, root, customCSSFile, "/* x */")

	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list/tickets", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), customCSSTag) {
		t.Error("client-side route did not receive the injected stylesheet")
	}
}

// TestSPAAssetsNotInjected pins that real embedded assets are delegated
// untouched — only the shell is rewritten.
func TestSPAAssetsNotInjected(t *testing.T) {
	if _, err := embeddedSPAShell(); err != nil {
		t.Skipf("embedded SPA not built: %v", err)
	}
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()
	root := t.TempDir()
	app.paths.Root = root
	writeCustom(t, root, customCSSFile, "/* x */")

	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/favicon.svg", http.NoBody))

	if strings.Contains(rec.Body.String(), customCSSTag) {
		t.Error("a real asset response was rewritten; only the SPA shell may be injected")
	}
}
