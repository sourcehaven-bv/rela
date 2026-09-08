package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The injector appends before </body>. If a future frontend build ships an
// index without one, injection silently no-ops and multi-window stops working
// with no error anywhere — so pin it against the built bundle. The file is
// gitignored, so this skips when the frontend has not been built.
func TestBuiltSPAHasBodyTag(t *testing.T) {
	index := filepath.Join("..", "..", "internal", "dataentry", "static", "v2", "index.html")
	data, err := os.ReadFile(index)
	if err != nil {
		t.Skip("frontend not built: " + err.Error())
	}
	assert.Contains(t, string(data), "</body>",
		"the injector needs a </body> to append before")
}

// Wails v3 serves its runtime at /wails/runtime.js but, unlike v2, does not
// inject it into the page. A page that never requests it has no window.wails,
// so every bound call fails with "Wails runtime unavailable" and multi-window
// silently does nothing. Both injected surfaces must load it.
func TestInjectedScriptLoadsWailsRuntime(t *testing.T) {
	out := string(injectMultiWindow([]byte("<html><body>app</body></html>"), ""))
	require.Contains(t, out, `src="/wails/runtime.js"`,
		"the runtime must be loaded or window.wails is undefined")
	assert.Less(t, strings.Index(out, "/wails/runtime.js"), strings.Index(out, "main.Desktop.OpenWindow"),
		"the runtime must load before the script that uses it")
}

func TestSafeRoute(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"simple route", "/dashboard", false},
		{"entity route", "/entity/ticket/TKT-1", false},
		{"route with query", "/list/tickets?sort=due", false},
		{"empty", "", true},
		{"relative", "dashboard", true},
		{"absolute url", "http://evil.example/x", true},
		{"protocol-relative escapes the origin", "//evil.example/x", true},
		{"carriage return", "/list\r/x", true},
		{"newline", "/list\n/x", true},
		{"null byte", "/list\x00", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := safeRoute(tc.path)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.path, got)
		})
	}
}

func TestInjectMultiWindow(t *testing.T) {
	t.Run("script goes before the closing body tag", func(t *testing.T) {
		out := string(injectMultiWindow([]byte("<html><body><p>hi</p></body></html>"), ""))
		assert.Contains(t, out, "main.Desktop.OpenWindow")
		assert.Less(t, strings.Index(out, "OpenWindow"), strings.Index(out, "</body>"),
			"script must be inside body")
		assert.True(t, strings.HasSuffix(out, "</body></html>"))
	})

	t.Run("document without a body tag is unchanged", func(t *testing.T) {
		in := []byte(`{"json":true}`)
		assert.Equal(t, in, injectMultiWindow(in, ""))
	})

	t.Run("uses the last body tag", func(t *testing.T) {
		out := string(injectMultiWindow([]byte("<body>a</body><!-- </body> -->"), ""))
		assert.Equal(t, 1, strings.Count(out, "main.Desktop.OpenWindow"))
	})
}

// The asset server carries the SSE live-reload stream. Buffering it would hang
// every connected client, so the injector must pass non-HTML straight through
// and honor Flush.
func TestHTMLInjector_DoesNotBufferStreams(t *testing.T) {
	rec := httptest.NewRecorder()
	inj := &htmlInjector{ResponseWriter: rec}

	inj.Header().Set("Content-Type", "text/event-stream")
	inj.WriteHeader(http.StatusOK)
	_, err := inj.Write([]byte("event: tick\ndata: 1\n\n"))
	require.NoError(t, err)
	inj.Flush()

	assert.Contains(t, rec.Body.String(), "data: 1",
		"stream bytes must reach the client before finish()")
	assert.False(t, inj.isHTML)
}

func TestHTMLInjector_InjectsHTML(t *testing.T) {
	rec := httptest.NewRecorder()
	inj := &htmlInjector{ResponseWriter: rec}

	inj.Header().Set("Content-Type", "text/html; charset=utf-8")
	inj.WriteHeader(http.StatusOK)
	_, err := inj.Write([]byte("<html><body>app</body></html>"))
	require.NoError(t, err)

	assert.Empty(t, rec.Body.String(), "HTML is held until finish()")
	inj.finish()
	assert.Contains(t, rec.Body.String(), "main.Desktop.OpenWindow")
}

// A stale Content-Length would truncate the injected document.
func TestHTMLInjector_DropsContentLength(t *testing.T) {
	rec := httptest.NewRecorder()
	inj := &htmlInjector{ResponseWriter: rec}

	inj.Header().Set("Content-Type", "text/html")
	inj.Header().Set("Content-Length", "29")
	inj.WriteHeader(http.StatusOK)
	_, _ = inj.Write([]byte("<html><body>app</body></html>"))
	inj.finish()

	assert.Empty(t, rec.Header().Get("Content-Length"))
}

// Non-HTML responses must keep their status and body untouched.
func TestHTMLInjector_PassesThroughJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	inj := &htmlInjector{ResponseWriter: rec}

	inj.Header().Set("Content-Type", "application/json")
	inj.WriteHeader(http.StatusCreated)
	_, _ = inj.Write([]byte(`{"ok":true}`))
	inj.finish()

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"ok":true}`, rec.Body.String())
}

// OpenWindow must reject a bad route before touching the window manager, and
// must not panic when called with no application (the frontend can call a
// bound method at any time).
func TestOpenWindow_ValidatesBeforeOpening(t *testing.T) {
	d := &Desktop{} // no wails app

	t.Run("no application yet", func(t *testing.T) {
		assert.Equal(t, "application not ready", d.OpenWindow("/dashboard", ""))
	})

	t.Run("bad routes are rejected", func(t *testing.T) {
		for _, bad := range []string{"", "relative", "//evil.example", "http://evil.example"} {
			got := d.OpenWindow(bad, "")
			assert.NotEmpty(t, got, "route %q must be rejected", bad)
		}
	})
}

// Window names must be unique: Wails looks windows up by name, so a collision
// would silently return the wrong window.
func TestWindowNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		n := windowSeq.Add(1)
		name := "secondary-" + strconv.FormatUint(n, 10)
		require.False(t, seen[name], "duplicate window name %s", name)
		seen[name] = true
	}
}

// A project served under /p/<id>/ must tell the SPA its base, or every
// root-absolute API call lands on the active project instead.
func TestInjectMultiWindow_DeclaresBase(t *testing.T) {
	t.Run("prefixed project emits a base tag", func(t *testing.T) {
		out := string(injectMultiWindow([]byte("<html><body>app</body></html>"), "/p/abc123"))
		assert.Contains(t, out, `<meta name="rela-base" content="/p/abc123/">`,
			"base must carry a trailing slash so relative joins work")
	})

	t.Run("active project at the root emits no base tag", func(t *testing.T) {
		out := string(injectMultiWindow([]byte("<html><body>app</body></html>"), ""))
		assert.NotContains(t, out, "rela-base")
	})

	t.Run("base is escaped", func(t *testing.T) {
		out := string(injectMultiWindow([]byte("<html><body>a</body></html>"), `/p/a"><script>x`))
		assert.NotContains(t, out, `"><script>x`, "a crafted id must not break out of the attribute")
	})
}
