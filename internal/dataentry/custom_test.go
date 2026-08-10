package dataentry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeCustom drops a customisation file in a project root.
func writeCustom(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCustomAsset(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{color:red}")
	writeCustom(t, root, customJSFile, "console.log(1)")
	// A secret outside the allowlist we must never serve.
	writeCustom(t, root, "secret.txt", "TOPSECRET")

	t.Run("css loads", func(t *testing.T) {
		b, err := openCustomAsset(root, customCSSFile)
		if err != nil || string(b) != ".a{color:red}" {
			t.Fatalf("got (%q, %v), want the stylesheet", b, err)
		}
	})
	t.Run("js loads", func(t *testing.T) {
		b, err := openCustomAsset(root, customJSFile)
		if err != nil || string(b) != "console.log(1)" {
			t.Fatalf("got (%q, %v), want the script", b, err)
		}
	})
	t.Run("missing file errors", func(t *testing.T) {
		if _, err := openCustomAsset(t.TempDir(), customCSSFile); err == nil {
			t.Error("expected error for missing custom.css")
		}
	})

	// Every one of these must be refused by the allowlist before the
	// filesystem is touched.
	bad := []string{
		"../secret.txt",
		"../../etc/passwd",
		"/etc/passwd",
		"sub/../../secret.txt",
		"",
		".",
		"secret.txt",
		"custom.css/",
		"custom.css/.",
		"CUSTOM.CSS", // case-insensitive filesystems (APFS) must not match
		"custom.cs",
		"custom.css\x00",
		"./custom.css",
	}
	for _, name := range bad {
		t.Run("rejects "+strconv.Quote(name), func(t *testing.T) {
			if _, err := openCustomAsset(root, name); err == nil {
				t.Errorf("openCustomAsset(%q) = nil error, want rejection", name)
			}
		})
	}
}

func TestOpenCustomAsset_Directory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, customCSSFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := openCustomAsset(root, customCSSFile); err == nil {
		t.Error("a directory named custom.css must not be served")
	}
}

func TestOpenCustomAsset_Oversize(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, strings.Repeat("a", maxCustomFileBytes+1))
	if _, err := openCustomAsset(root, customCSSFile); err == nil {
		t.Error("an oversize custom.css must be rejected")
	}
}

func TestOpenCustomAsset_EmptyFileIsServed(t *testing.T) {
	// Present-but-empty is a real state: it must serve (and, per
	// TestSelectShell, still inject). "Present" is not "non-empty".
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, "")
	b, err := openCustomAsset(root, customCSSFile)
	if err != nil || len(b) != 0 {
		t.Fatalf("got (%q, %v), want empty content and no error", b, err)
	}
}

func TestOpenCustomAsset_SymlinkEscape(t *testing.T) {
	// os.OpenRoot is the defense-in-depth layer behind the allowlist: an
	// allowlisted NAME pointing outside the project must still be refused.
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(secret, filepath.Join(root, customCSSFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := openCustomAsset(root, customCSSFile); err == nil {
		t.Error("a symlink escaping the project root must be rejected")
	}
}

func TestCustomAssetContentType(t *testing.T) {
	tests := []struct{ name, want string }{
		{customCSSFile, "text/css; charset=utf-8"},
		{customJSFile, "text/javascript; charset=utf-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := customAssetContentType(tt.name); got != tt.want {
				t.Errorf("customAssetContentType(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// --- shell injection ---------------------------------------------------

const testShell = "<!DOCTYPE html>\n<html>\n  <head>\n    <title>rela</title>\n  </head>\n  <body>\n    <div id=\"app\"></div>\n  </body>\n</html>\n"

func TestBuildShellVariants_NoInjectionWhenAbsent(t *testing.T) {
	// The strongest form of "no injection": a stock deployment's HTML is
	// byte-identical to the embedded shell.
	v := buildShellVariants([]byte(testShell), true)
	got := v.selectShell(t.TempDir())
	if string(got) != testShell {
		t.Errorf("shell was modified with no customisation files present:\ngot  %q\nwant %q", got, testShell)
	}
}

func TestSelectShell(t *testing.T) {
	tests := []struct {
		name          string
		files         []string
		wantCSS       bool
		wantJS        bool
		disableInject bool
	}{
		{name: "neither"},
		{name: "css only", files: []string{customCSSFile}, wantCSS: true},
		{name: "js only", files: []string{customJSFile}, wantJS: true},
		{name: "both", files: []string{customCSSFile, customJSFile}, wantCSS: true, wantJS: true},
		{
			name:          "disabled suppresses both",
			files:         []string{customCSSFile, customJSFile},
			disableInject: true,
		},
		{
			// Present-but-empty still injects: "present" is not "non-empty".
			name: "empty file still injects", files: []string{customCSSFile}, wantCSS: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tt.files {
				writeCustom(t, root, f, "")
			}
			v := buildShellVariants([]byte(testShell), !tt.disableInject)
			got := string(v.selectShell(root))

			if gotCSS := strings.Contains(got, customCSSTag); gotCSS != tt.wantCSS {
				t.Errorf("css tag present = %v, want %v", gotCSS, tt.wantCSS)
			}
			if gotJS := strings.Contains(got, customJSTag); gotJS != tt.wantJS {
				t.Errorf("js tag present = %v, want %v", gotJS, tt.wantJS)
			}
			// Whatever the variant, each tag appears at most once.
			if n := strings.Count(got, customCSSTag); n > 1 {
				t.Errorf("css tag appears %d times, want <= 1", n)
			}
			if n := strings.Count(got, customJSTag); n > 1 {
				t.Errorf("js tag appears %d times, want <= 1", n)
			}
		})
	}
}

func TestInjectTags_Placement(t *testing.T) {
	out := string(injectTags([]byte(testShell), customCSSTag, customJSTag))

	cssAt := strings.Index(out, customCSSTag)
	headAt := strings.Index(out, "</head>")
	jsAt := strings.Index(out, customJSTag)
	bodyAt := strings.Index(out, "</body>")

	if cssAt < 0 || headAt < 0 || cssAt > headAt {
		t.Errorf("stylesheet must be injected before </head> (css=%d head=%d)", cssAt, headAt)
	}
	if jsAt < 0 || bodyAt < 0 || jsAt > bodyAt {
		t.Errorf("script must be injected before </body> (js=%d body=%d)", jsAt, bodyAt)
	}
	if jsAt < headAt {
		t.Error("script must come after </head>, not inside the head")
	}
}

func TestInjectTags_MissingMarkers(t *testing.T) {
	// A shell without the expected insertion points must be left alone rather
	// than corrupted.
	shell := []byte("<html><p>no head or body close</p>")
	got := injectTags(shell, customCSSTag, customJSTag)
	if !bytes.Equal(got, shell) {
		t.Errorf("shell without markers was modified:\ngot  %q\nwant %q", got, shell)
	}
}

// --- handler -----------------------------------------------------------

func TestHandleCustomAsset(t *testing.T) {
	app := newHandlerTestApp(t)
	root := t.TempDir()
	app.paths.Root = root
	writeCustom(t, root, customCSSFile, ".a{color:red}")
	writeCustom(t, root, "secret.txt", "TOPSECRET")

	tests := []struct {
		name            string
		path            string
		wantStatus      int
		wantBody        string
		wantContentType string
	}{
		{
			name:            "serves css",
			path:            customURLPrefix + customCSSFile,
			wantStatus:      http.StatusOK,
			wantBody:        ".a{color:red}",
			wantContentType: "text/css; charset=utf-8",
		},
		{name: "404 for absent js", path: customURLPrefix + customJSFile, wantStatus: http.StatusNotFound},
		{name: "404 for non-allowlisted", path: customURLPrefix + "secret.txt", wantStatus: http.StatusNotFound},
		{name: "404 for traversal", path: customURLPrefix + "../secret.txt", wantStatus: http.StatusNotFound},
		{name: "404 for empty name", path: customURLPrefix, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			app.handleCustomAsset(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantContentType != "" {
				if got := rec.Header().Get("Content-Type"); got != tt.wantContentType {
					t.Errorf("Content-Type = %q, want %q", got, tt.wantContentType)
				}
			}
			// A served asset must never be sniffable and must revalidate:
			// the URL is stable forever, so heuristic caching would strand
			// operators on a stale stylesheet.
			if tt.wantStatus == http.StatusOK {
				if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
					t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
				}
				if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
					t.Errorf("Cache-Control = %q, want no-cache", got)
				}
			}
		})
	}
}

// TestHandleCustomAsset_TraversalNeverEscapes is the security-critical case:
// a secret outside the project root must be unreachable however the request
// is spelled. Uses the real router so mux path-cleaning is exercised too.
func TestHandleCustomAsset_TraversalNeverEscapes(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := newHandlerTestApp(t)
	app.paths.Root = t.TempDir()
	app.broker = newEventBroker()
	handler := app.NewRouter()

	for _, path := range []string{
		customURLPrefix + "../secret.txt",
		customURLPrefix + "../../secret.txt",
		customURLPrefix + "..%2Fsecret.txt",
		customURLPrefix + "%2E%2E%2Fsecret.txt",
		customURLPrefix + "sub/../../secret.txt",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if strings.Contains(rec.Body.String(), "TOPSECRET") {
				t.Fatalf("LEAK: %s served content from outside the project root", path)
			}
		})
	}
}

// TestCustomAssetExists_MatchesOpen pins that the cheap stat-based existence
// check agrees with the authoritative read for every case that decides whether
// the shell references a file. A divergence would produce the confusing
// half-state where the shell links an asset that then 404s (or omits one that
// serves fine).
func TestCustomAssetExists_MatchesOpen(t *testing.T) {
	t.Run("present file", func(t *testing.T) {
		root := t.TempDir()
		writeCustom(t, root, customCSSFile, ".a{}")
		if !customAssetExists(root, customCSSFile) {
			t.Error("exists=false for a readable file")
		}
	})
	t.Run("absent file", func(t *testing.T) {
		if customAssetExists(t.TempDir(), customCSSFile) {
			t.Error("exists=true for a missing file")
		}
	})
	t.Run("directory", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, customCSSFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if customAssetExists(root, customCSSFile) {
			t.Error("exists=true for a directory")
		}
	})
	t.Run("non-allowlisted name", func(t *testing.T) {
		root := t.TempDir()
		writeCustom(t, root, "secret.txt", "x")
		if customAssetExists(root, "secret.txt") {
			t.Error("exists=true for a non-allowlisted name")
		}
	})
	t.Run("symlink escaping the root", func(t *testing.T) {
		// os.Root.Stat must refuse to follow outside the root, matching
		// openCustomAsset. If these diverged, the shell would reference a file
		// that the handler then refuses to serve.
		outside := t.TempDir()
		target := filepath.Join(outside, "evil.css")
		if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		if err := os.Symlink(target, filepath.Join(root, customCSSFile)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		gotExists := customAssetExists(root, customCSSFile)
		_, openErr := openCustomAsset(root, customCSSFile)
		gotOpen := openErr == nil
		if gotExists != gotOpen {
			t.Errorf("exists=%v but open-succeeds=%v — the two checks must agree", gotExists, gotOpen)
		}
		if gotExists {
			t.Error("a symlink escaping the project root must not count as present")
		}
	})
}
