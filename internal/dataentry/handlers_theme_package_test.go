package dataentry

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// uploadThemePackage is a test helper that POSTs a multipart body with
// the given raw zip bytes to the import handler.
func uploadThemePackage(t *testing.T, app *App, fieldName string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, "theme.relatheme")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_theme/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	app.handleAPIThemeImport(w, req)
	return w
}

// readZipBody parses an export response and returns the entries map.
func readZipBody(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	out := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %q: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %q: %v", f.Name, err)
		}
		out[f.Name] = data
	}
	return out
}

func TestThemeExport_PaletteOnly(t *testing.T) {
	app := newHandlerTestApp(t)
	// Set a palette via the existing API to mirror real flow.
	body := `{"accent":"#abcdef"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/_palette", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleAPISavePalette(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("save palette: %d %s", w.Code, w.Body.String())
	}

	exReq := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/export", http.NoBody)
	exW := httptest.NewRecorder()
	app.handleAPIThemeExport(exW, exReq)
	if exW.Code != http.StatusOK {
		t.Fatalf("export: %d %s", exW.Code, exW.Body.String())
	}
	if ct := exW.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type: %q", ct)
	}
	if cd := exW.Header().Get("Content-Disposition"); !strings.Contains(cd, ".relatheme") {
		t.Errorf("Content-Disposition: %q", cd)
	}

	entries := readZipBody(t, exW.Body.Bytes())
	if _, ok := entries["theme.yaml"]; !ok {
		t.Fatal("missing theme.yaml")
	}
	for name := range entries {
		if strings.HasPrefix(name, "logo.") {
			t.Errorf("expected no logo entry, got %q", name)
		}
	}

	var manifest dataentryconfig.ThemeManifest
	if err := yaml.Unmarshal(entries["theme.yaml"], &manifest); err != nil {
		t.Fatalf("manifest unmarshal: %v", err)
	}
	if manifest.Accent != "#abcdef" {
		t.Errorf("accent: %q", manifest.Accent)
	}
	if manifest.Logo != "" {
		t.Errorf("expected no logo ref, got %q", manifest.Logo)
	}
}

func TestThemeExport_WithLogo(t *testing.T) {
	app := newHandlerTestApp(t)
	pngBytes := makeTinyPNG(t)
	if w := uploadLogo(t, app, "logo", pngBytes); w.Code != http.StatusOK {
		t.Fatalf("upload logo: %d %s", w.Code, w.Body.String())
	}

	exReq := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/export", http.NoBody)
	exW := httptest.NewRecorder()
	app.handleAPIThemeExport(exW, exReq)
	if exW.Code != http.StatusOK {
		t.Fatalf("export: %d", exW.Code)
	}

	entries := readZipBody(t, exW.Body.Bytes())
	logoEntry, ok := entries["logo.png"]
	if !ok {
		t.Fatal("missing logo.png in zip")
	}
	if !bytes.Equal(logoEntry, pngBytes) {
		t.Error("logo bytes do not round-trip")
	}

	var manifest dataentryconfig.ThemeManifest
	if err := yaml.Unmarshal(entries["theme.yaml"], &manifest); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if manifest.Logo != "logo.png" {
		t.Errorf("manifest.logo: %q", manifest.Logo)
	}
}

func TestThemeImport_RoundTrip(t *testing.T) {
	source := newHandlerTestApp(t)
	pngBytes := makeTinyPNG(t)
	if w := uploadLogo(t, source, "logo", pngBytes); w.Code != http.StatusOK {
		t.Fatalf("source upload logo: %d %s", w.Code, w.Body.String())
	}
	body := `{"accent":"#aabbcc","badges":{"blue":"#1e40af"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/_palette", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	source.handleAPISavePalette(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("source palette: %d %s", w.Code, w.Body.String())
	}

	exReq := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/export", http.NoBody)
	exW := httptest.NewRecorder()
	source.handleAPIThemeExport(exW, exReq)
	if exW.Code != http.StatusOK {
		t.Fatalf("export: %d", exW.Code)
	}

	dest := newHandlerTestApp(t)
	if dest.State().UserLogoExt != "" {
		t.Fatal("dest should not start with a logo")
	}

	imW := uploadThemePackage(t, dest, "file", exW.Body.Bytes())
	if imW.Code != http.StatusOK {
		t.Fatalf("import: %d %s", imW.Code, imW.Body.String())
	}

	var resp struct {
		Palette dataentryconfig.PaletteConfig `json:"palette"`
		LogoURL string                        `json:"logoUrl"`
	}
	if err := json.NewDecoder(imW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if resp.Palette.Accent != "#aabbcc" {
		t.Errorf("palette.accent: %q", resp.Palette.Accent)
	}
	if resp.Palette.Badges["blue"] != "#1e40af" {
		t.Errorf("badges.blue: %q", resp.Palette.Badges["blue"])
	}
	if !strings.HasPrefix(resp.LogoURL, "/api/v1/_theme/logo?v=") {
		t.Errorf("logoUrl: %q", resp.LogoURL)
	}

	if !bytes.Equal(dest.State().UserLogoBytes, pngBytes) {
		t.Error("dest UserLogoBytes does not match round-tripped bytes")
	}
	if dest.State().UserPalette != nil {
		t.Error("dest UserPalette should not be auto-saved on import")
	}
}

func TestThemeImport_Errors(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		body       []byte
		wantStatus int
		wantSubstr string
	}{
		{"not a zip", "file", []byte("hello"), http.StatusBadRequest, "not a valid zip"},
		{"missing field", "wrong", []byte("hello"), http.StatusBadRequest, "missing form field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			w := uploadThemePackage(t, app, tt.fieldName, tt.body)
			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d (body=%s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.wantSubstr) {
				t.Errorf("body: %q does not contain %q", w.Body.String(), tt.wantSubstr)
			}
			if app.State().UserLogoExt != "" {
				t.Error("rejected import must not have mutated state")
			}
		})
	}
}

func TestThemeExport_MethodNotAllowed(t *testing.T) {
	app := newHandlerTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_theme/export", http.NoBody)
	w := httptest.NewRecorder()
	app.handleAPIThemeExport(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestThemeImport_MethodNotAllowed(t *testing.T) {
	app := newHandlerTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/import", http.NoBody)
	w := httptest.NewRecorder()
	app.handleAPIThemeImport(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSafeThemeFilename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"My Theme", "My_Theme"},
		{"my-cool-theme", "my-cool-theme"},
		{"weird/path:name", "weird_path_name"},
		{"", "theme"},
		{strings.Repeat("a", 100), strings.Repeat("a", 64)},
		{"___only_underscores___", "only_underscores"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := safeThemeFilename(tt.in)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
