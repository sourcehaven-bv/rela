package dataentry

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// makeTinyPNG returns a small valid PNG so http.DetectContentType
// classifies the upload as image/png. The exact pixel values don't
// matter — only the magic bytes and structural validity do.
func makeTinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// jpegMagicBytes is enough of a JFIF stream for http.DetectContentType
// to identify the body as image/jpeg.
func jpegMagicBytes() []byte {
	return []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xd9,
	}
}

// gifMagicBytes triggers detection as image/gif (not in our allowlist).
func gifMagicBytes() []byte {
	return []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02L\x01\x00;")
}

// webpMagicBytes triggers detection as image/webp.
func webpMagicBytes() []byte {
	return []byte("RIFF\x24\x00\x00\x00WEBPVP8 \x18\x00\x00\x00\x30\x01\x00\x9d\x01\x2a\x01\x00\x01\x00\x02\x00\x34\x25\xa4\x00\x03\x70\x00\xfe\xfb\x94\x00\x00")
}

// hostileSVGBytes embeds the patterns we want the browser sandbox to
// neutralize so the e2e tests can confirm none of them activate.
func hostileSVGBytes() []byte {
	return []byte(`<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
  <script>alert("xss")</script>
  <image xlink:href="https://example.invalid/leak" width="10" height="10"/>
  <rect width="10" height="10" fill="green" onload="alert('onload')"/>
</svg>`)
}

// uploadLogo is a test helper that POSTs a multipart body to the logo
// PUT handler and returns the recorder.
func uploadLogo(t *testing.T, app *App, fieldName string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, "logo")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/_theme/logo", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	app.handleAPIThemeLogo(w, req)
	return w
}

func TestThemeLogo_RoundTrip(t *testing.T) {
	app := newHandlerTestApp(t)
	pngBytes := makeTinyPNG(t)

	// PUT
	w := uploadLogo(t, app, "logo", pngBytes)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var put struct {
		Ok      bool   `json:"ok"`
		LogoURL string `json:"logoUrl"`
	}
	if err := decodeJSON(w.Body, &put); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if !put.Ok {
		t.Error("expected ok:true in PUT response")
	}
	if !strings.HasPrefix(put.LogoURL, "/api/v1/_theme/logo?v=") {
		t.Errorf("expected hashed URL, got %q", put.LogoURL)
	}

	// State should reflect the upload.
	s := app.State()
	if s.UserLogoExt != "png" {
		t.Errorf("expected ext=png, got %q", s.UserLogoExt)
	}
	if !bytes.Equal(s.UserLogoBytes, pngBytes) {
		t.Error("UserLogoBytes does not match upload")
	}
	if s.UserLogoHash == "" {
		t.Error("UserLogoHash empty after upload")
	}

	// GET should return the bytes byte-for-byte.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/logo", http.NoBody)
	getW := httptest.NewRecorder()
	app.handleAPIThemeLogo(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d", getW.Code)
	}
	if got := getW.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type: expected image/png, got %q", got)
	}
	if got := getW.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: expected nosniff, got %q", got)
	}
	if got := getW.Header().Get("Content-Security-Policy"); got != "sandbox; frame-ancestors 'none'" {
		t.Errorf("CSP: expected `sandbox; frame-ancestors 'none'`, got %q", got)
	}
	if got := getW.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options: expected DENY, got %q", got)
	}
	if !bytes.Equal(getW.Body.Bytes(), pngBytes) {
		t.Error("GET body does not match uploaded bytes")
	}

	// DELETE should clear state and 204.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/_theme/logo", http.NoBody)
	delW := httptest.NewRecorder()
	app.handleAPIThemeLogo(delW, delReq)
	if delW.Code != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d", delW.Code)
	}
	if app.State().UserLogoExt != "" {
		t.Error("UserLogoExt not cleared after DELETE")
	}

	// Subsequent GET should 404.
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/logo", http.NoBody)
	getW2 := httptest.NewRecorder()
	app.handleAPIThemeLogo(getW2, getReq2)
	if getW2.Code != http.StatusNotFound {
		t.Errorf("GET after DELETE: expected 404, got %d", getW2.Code)
	}
}

func TestThemeLogo_Validation(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		fieldName   string
		wantStatus  int
		wantContent string // substring match on response body
	}{
		{"GIF rejected", gifMagicBytes(), "logo", http.StatusBadRequest, "unsupported format"},
		{"plain text rejected", []byte("hello world"), "logo", http.StatusBadRequest, "unsupported format"},
		{"wrong field name", []byte("ignored"), "image", http.StatusBadRequest, "missing form field"},
		{"empty body rejected", []byte{}, "logo", http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			w := uploadLogo(t, app, tt.fieldName, tt.body)
			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d (body=%s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantContent != "" && !strings.Contains(w.Body.String(), tt.wantContent) {
				t.Errorf("body: %q does not contain %q", w.Body.String(), tt.wantContent)
			}
			if app.State().UserLogoExt != "" {
				t.Error("rejected upload should not have mutated state")
			}
		})
	}
}

func TestThemeLogo_AcceptedFormats(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		ext  string
	}{
		{"png", makeTinyPNG(t), "png"},
		{"jpeg", jpegMagicBytes(), "jpeg"},
		{"webp", webpMagicBytes(), "webp"},
		{"svg", hostileSVGBytes(), "svg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			w := uploadLogo(t, app, "logo", tt.body)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
			}
			if app.State().UserLogoExt != tt.ext {
				t.Errorf("ext: got %q, want %q", app.State().UserLogoExt, tt.ext)
			}
		})
	}
}

func TestThemeLogo_TooLarge(t *testing.T) {
	app := newHandlerTestApp(t)
	// PNG header followed by enough padding to exceed the limit.
	tooBig := make([]byte, MaxUserLogoBytes+512)
	copy(tooBig, makeTinyPNG(t))
	w := uploadLogo(t, app, "logo", tooBig)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d body=%s", w.Code, w.Body.String())
	}
	if app.State().UserLogoExt != "" {
		t.Error("oversized upload must not mutate state")
	}
}

func TestThemeLogo_ExactlyAtLimit(t *testing.T) {
	app := newHandlerTestApp(t)
	// PNG header padded out with a custom chunk to reach exactly the
	// limit. The chunk-vs-chunk consistency doesn't matter — only
	// http.DetectContentType behavior on the prefix.
	atLimit := make([]byte, MaxUserLogoBytes)
	copy(atLimit, makeTinyPNG(t))
	w := uploadLogo(t, app, "logo", atLimit)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 at exact limit, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestThemeLogo_SVGGetHeaders(t *testing.T) {
	app := newHandlerTestApp(t)
	if w := uploadLogo(t, app, "logo", hostileSVGBytes()); w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	getW := httptest.NewRecorder()
	app.handleAPIThemeLogo(getW, httptest.NewRequest(http.MethodGet, "/api/v1/_theme/logo", http.NoBody))
	if getW.Code != http.StatusOK {
		t.Fatalf("GET: %d", getW.Code)
	}
	want := map[string]string{
		"Content-Type":            "image/svg+xml",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "sandbox; frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
	}
	for k, v := range want {
		if got := getW.Header().Get(k); got != v {
			t.Errorf("%s: got %q, want %q", k, got, v)
		}
	}
}

func TestSniffLogoMime_RejectsSVGPolyglots(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		// wantSVG is true when the body should be classified as SVG.
		wantSVG bool
	}{
		{"plain svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), true},
		{"with xml prologue", []byte(`<?xml version="1.0"?><svg></svg>`), true},
		{"with doctype", []byte(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" ""><svg></svg>`), true},
		{"with comment", []byte(`<!-- comment --><svg></svg>`), true},
		{"with leading whitespace", []byte("\n\t  <svg></svg>"), true},
		// Polyglots: <svg appears, but it's not the first real element.
		{"html wrapper around svg", []byte(`<html><body><svg></svg></body></html>`), false},
		{"div before svg", []byte(`<div><svg></svg></div>`), false},
		{"text before svg", []byte(`hello <svg></svg>`), false},
		// Edge: prefix that's not the SVG element.
		{"svgfoo (not svg tag)", []byte(`<svgfoo></svgfoo>`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeSVG(tt.body)
			if got != tt.wantSVG {
				t.Errorf("looksLikeSVG: got %v, want %v", got, tt.wantSVG)
			}
		})
	}
}

func TestThemeLogo_DeleteIdempotent(t *testing.T) {
	app := newHandlerTestApp(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/_theme/logo", http.NoBody)
	w := httptest.NewRecorder()
	app.handleAPIThemeLogo(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE on missing logo: expected 204, got %d", w.Code)
	}
}

func TestThemeLogo_GetWhenUnset(t *testing.T) {
	app := newHandlerTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_theme/logo", http.NoBody)
	w := httptest.NewRecorder()
	app.handleAPIThemeLogo(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET when unset: expected 404, got %d", w.Code)
	}
}

func TestThemeLogo_MethodNotAllowed(t *testing.T) {
	app := newHandlerTestApp(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/_theme/logo", http.NoBody)
	w := httptest.NewRecorder()
	app.handleAPIThemeLogo(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PATCH: expected 405, got %d", w.Code)
	}
}

func TestSettings_ExposesLogoURL(t *testing.T) {
	app := newHandlerTestApp(t)
	pngBytes := makeTinyPNG(t)
	w := uploadLogo(t, app, "logo", pngBytes)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_settings", http.NoBody)
	sw := httptest.NewRecorder()
	app.handleAPIGetSettings(sw, req)
	if sw.Code != http.StatusOK {
		t.Fatalf("get settings: %d", sw.Code)
	}
	var data APISettingsData
	if err := decodeJSON(sw.Body, &data); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if data.LogoURL == nil {
		t.Fatal("expected non-nil LogoURL after upload")
	}
	if !strings.HasPrefix(*data.LogoURL, "/api/v1/_theme/logo?v=") {
		t.Errorf("LogoURL: %q", *data.LogoURL)
	}
}

// decodeJSON is a tiny helper to keep the assertions concise.
func decodeJSON(r io.Reader, out interface{}) error {
	return json.NewDecoder(r).Decode(out)
}
