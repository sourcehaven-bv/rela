package dataentry

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

// writeCustom drops a file inside the project's custom/ directory, creating
// intermediate dirs so nested entries ("fonts/b.woff2") work.
func writeCustom(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, project.CustomDir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeProjectRoot drops a file at the PROJECT ROOT, outside custom/. Used to
// prove the old root-level layout is gone.
func writeProjectRoot(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCustomEntry(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{color:red}")
	writeCustom(t, root, customJSFile, "console.log(1)")
	// A file that used to be refused by the two-name allowlist; now served.
	writeCustom(t, root, "secret.txt", "TOPSECRET")

	t.Run("css loads", func(t *testing.T) {
		b, err := openCustomEntry(root, customCSSFile)
		if err != nil || string(b) != ".a{color:red}" {
			t.Fatalf("got (%q, %v), want the stylesheet", b, err)
		}
	})
	t.Run("js loads", func(t *testing.T) {
		b, err := openCustomEntry(root, customJSFile)
		if err != nil || string(b) != "console.log(1)" {
			t.Fatalf("got (%q, %v), want the script", b, err)
		}
	})
	t.Run("missing file errors", func(t *testing.T) {
		if _, err := openCustomEntry(t.TempDir(), customCSSFile); err == nil {
			t.Error("expected error for missing custom.css")
		}
	})

	// Only malformed/traversal/dot spellings are refused now. A plain name like
	// "secret.txt" is NO LONGER a policy violation — under the folder layout any
	// file the operator puts in custom/ is meant to be served (AC7).
	bad := []string{
		"",
		".",
		"..",
		".env",
		".env.backup",
		".git/config",
		".DS_Store",
		"sub/.hidden",
		"sub/.git/config",
		"custom.css\x00",
	}
	for _, name := range bad {
		t.Run("rejects "+strconv.Quote(name), func(t *testing.T) {
			if _, err := openCustomEntry(root, name); err == nil {
				t.Errorf("openCustomEntry(%q) = nil error, want rejection", name)
			}
		})
	}

	// Traversal spellings are NEUTRALIZED, not rejected: path.Clean anchors
	// "../secret.txt" to "/secret.txt", i.e. custom/secret.txt. The property
	// that matters is containment — see TestOpenCustomEntry_NeverEscapes.

	// The inversion: these used to be refused by the two-name allowlist and
	// must now be SERVED, because they live inside custom/.
	t.Run("arbitrary name inside custom/ is served", func(t *testing.T) {
		writeCustom(t, root, "secret.txt", "operator content")
		b, err := openCustomEntry(root, "secret.txt")
		if err != nil || string(b) != "operator content" {
			t.Fatalf("got (%q, %v), want the file to be served", b, err)
		}
	})
	t.Run("nested asset is served", func(t *testing.T) {
		writeCustom(t, root, "fonts/brand.woff2", "FONT")
		b, err := openCustomEntry(root, "fonts/brand.woff2")
		if err != nil || string(b) != "FONT" {
			t.Fatalf("got (%q, %v), want the nested asset", b, err)
		}
	})
	t.Run("unknown extension is served, not 404 (AC7)", func(t *testing.T) {
		writeCustom(t, root, "data.avif", "AVIF")
		if _, err := openCustomEntry(root, "data.avif"); err != nil {
			t.Errorf("unknown extension must still serve: %v", err)
		}
		if got := appEntryContentType("data.avif"); got != "application/octet-stream" {
			t.Errorf("content type = %q, want application/octet-stream", got)
		}
	})
}

func TestOpenCustomEntry_Directory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, customCSSFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := openCustomEntry(root, customCSSFile); err == nil {
		t.Error("a directory named custom.css must not be served")
	}
}

func TestOpenCustomEntry_Oversize(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, strings.Repeat("a", maxCustomFileBytes+1))
	if _, err := openCustomEntry(root, customCSSFile); err == nil {
		t.Error("an oversize custom.css must be rejected")
	}
}

func TestOpenCustomEntry_EmptyFileIsServed(t *testing.T) {
	// Present-but-empty is a real state: it must serve (and, per
	// TestSelectShell, still inject). "Present" is not "non-empty".
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, "")
	b, err := openCustomEntry(root, customCSSFile)
	if err != nil || len(b) != 0 {
		t.Fatalf("got (%q, %v), want empty content and no error", b, err)
	}
}

func TestOpenCustomEntry_SymlinkEscape(t *testing.T) {
	// os.OpenRoot is now the PRIMARY containment boundary, not defense-in-depth
	// behind an allowlist: TKT-IWMETE removed the two-name check, so a symlink
	// escaping the project root must be refused by the roots alone.
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(secret, filepath.Join(root, customCSSFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := openCustomEntry(root, customCSSFile); err == nil {
		t.Error("a symlink escaping the project root must be rejected")
	}
}

func TestCustomEntryContentType(t *testing.T) {
	// Extension-based via the shared apps/ map. Unknown → octet-stream, which a
	// browser neither executes nor renders (AC7).
	tests := []struct{ name, want string }{
		{"custom.css", "text/css; charset=utf-8"},
		{"custom.js", "text/javascript; charset=utf-8"},
		{"logo.svg", "image/svg+xml"},
		{"fonts/brand.woff2", "font/woff2"},
		{"pic.png", "image/png"},
		{"data.avif", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appEntryContentType(tt.name); got != tt.want {
				t.Errorf("appEntryContentType(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// --- shell injection ---------------------------------------------------

const testShell = "<!DOCTYPE html>\n<html>\n  <head>\n    <title>rela</title>\n  </head>\n  <body>\n    <div id=\"app\"></div>\n  </body>\n</html>\n"

func TestBuildShellVariants_NoInjectionWhenAbsent(t *testing.T) {
	// The strongest form of "no injection": a stock deployment's HTML is
	// byte-identical to the embedded shell.
	v := buildShellVariants([]byte(testShell))
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
			custom := newCustomAssets(root, []byte(testShell), func() bool { return !tt.disableInject })
			got := string(custom.shell())

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
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{color:red}")
	writeCustom(t, root, "secret.txt", "TOPSECRET")
	writeCustom(t, root, ".env", "SECRET=1")
	custom := newCustomAssets(root, []byte(testShell), func() bool { return true })

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
		{
			// INVERTED by TKT-IWMETE: under the folder layout an arbitrary file
			// inside custom/ is meant to be served.
			name: "serves an arbitrary file inside custom/", path: customURLPrefix + "secret.txt",
			wantStatus: http.StatusOK, wantBody: "TOPSECRET",
			wantContentType: "text/plain; charset=utf-8",
		},
		{name: "404 for dot-prefixed", path: customURLPrefix + ".env", wantStatus: http.StatusNotFound},
		{
			// NOT 404: path.Clean anchors "../secret.txt" to custom/secret.txt,
			// which the fixture creates. Containment (never reaching a file
			// OUTSIDE custom/) is asserted by
			// TestHandleCustomAsset_TraversalNeverEscapes.
			name: "traversal is anchored inside custom/", path: customURLPrefix + "../secret.txt",
			wantStatus: http.StatusOK, wantBody: "TOPSECRET",
		},
		{name: "404 for empty name", path: customURLPrefix, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			custom.serveAsset(rec, req)

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
	t.Run("arbitrary name inside custom/ exists (inverted)", func(t *testing.T) {
		root := t.TempDir()
		writeCustom(t, root, "secret.txt", "x")
		if !customAssetExists(root, "secret.txt") {
			t.Error("exists=false for a real file inside custom/")
		}
	})
	t.Run("dot-prefixed name does not exist", func(t *testing.T) {
		root := t.TempDir()
		writeCustom(t, root, ".env", "x")
		if customAssetExists(root, ".env") {
			t.Error("exists=true for a dot-prefixed entry")
		}
	})
	t.Run("oversize file", func(t *testing.T) {
		// REGRESSION (code review): stat succeeded while open rejected on size,
		// so the shell injected a <link> to a URL that then 404'd. The comment
		// on this test claimed to pin exactly that and did not.
		root := t.TempDir()
		writeCustom(t, root, customCSSFile, strings.Repeat("a", maxCustomFileBytes+1))
		gotExists := customAssetExists(root, customCSSFile)
		_, openErr := openCustomEntry(root, customCSSFile)
		if gotExists != (openErr == nil) {
			t.Errorf("exists=%v but open-succeeds=%v — the shell would reference a 404",
				gotExists, openErr == nil)
		}
	})
	t.Run("unreadable file", func(t *testing.T) {
		// Same divergence via permissions rather than size.
		root := t.TempDir()
		writeCustom(t, root, customCSSFile, ".a{}")
		p := filepath.Join(root, project.CustomDir, customCSSFile)
		if err := os.Chmod(p, 0o000); err != nil {
			t.Skipf("chmod unavailable: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		gotExists := customAssetExists(root, customCSSFile)
		_, openErr := openCustomEntry(root, customCSSFile)
		if gotExists != (openErr == nil) {
			t.Errorf("exists=%v but open-succeeds=%v — the shell would reference a 404",
				gotExists, openErr == nil)
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
		_, openErr := openCustomEntry(root, customCSSFile)
		gotOpen := openErr == nil
		if gotExists != gotOpen {
			t.Errorf("exists=%v but open-succeeds=%v — the two checks must agree", gotExists, gotOpen)
		}
		if gotExists {
			t.Error("a symlink escaping the project root must not count as present")
		}
	})
}

// TestCustomAssets_EnabledIsReadPerRequest pins that disable_custom_injection
// is consulted live rather than snapshotted at construction. data-entry.yaml is
// reloadable, so a snapshot would leave a running server honoring a stale
// value — the operator flips the flag, reloads, and nothing changes.
func TestCustomAssets_EnabledIsReadPerRequest(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{}")

	enabled := true
	custom := newCustomAssets(root, []byte(testShell), func() bool { return enabled })

	if !strings.Contains(string(custom.shell()), customCSSTag) {
		t.Fatal("expected the stylesheet reference while enabled")
	}
	enabled = false
	if strings.Contains(string(custom.shell()), customCSSTag) {
		t.Error("shell still references custom.css after the flag flipped to disabled")
	}
	enabled = true
	if !strings.Contains(string(custom.shell()), customCSSTag) {
		t.Error("shell did not pick the reference back up after re-enabling")
	}
}

// TestCustomAssets_ServingIgnoresInjectionFlag pins that
// disable_custom_injection suppresses only the shell references — the files
// stay individually fetchable, which is what makes the flag usable for
// bisecting whether a customisation is causing a bug.
func TestCustomAssets_ServingIgnoresInjectionFlag(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{color:red}")
	custom := newCustomAssets(root, []byte(testShell), func() bool { return false })

	rec := httptest.NewRecorder()
	custom.serveAsset(rec, httptest.NewRequest(http.MethodGet, customURLPrefix+customCSSFile, http.NoBody))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200: the file must stay fetchable when injection is disabled", rec.Code)
	}
	if strings.Contains(string(custom.shell()), customCSSTag) {
		t.Error("shell must not reference custom.css while injection is disabled")
	}
}

// TestCustomAssets_UnreadableShellDegrades pins that an empty/unreadable
// embedded shell yields nil (caller falls back to the plain file server)
// rather than serving a corrupt or empty document.
func TestCustomAssets_UnreadableShellDegrades(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{}")
	custom := newCustomAssets(root, nil, func() bool { return true })

	if custom.shell() != nil {
		t.Error("expected nil shell so the caller delegates to the plain SPA handler")
	}
}

// TestOpenCustomEntry_SymlinkInsideProject pins the property that the NESTED
// os.OpenRoot exists for (AC10, RR-DR-SYMLINK).
//
// A symlink inside custom/ pointing at a project file OUTSIDE custom/ never
// leaves the project root, so a single os.OpenRoot(projectRoot) would happily
// follow it and serve the file. Only the second root scoped to custom/ refuses
// it. This is the case a "simplification" to one root would silently reopen —
// the ../secret.txt spellings would all still pass.
func TestOpenCustomEntry_SymlinkInsideProject(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, customCSSFile, ".a{}")
	writeProjectRoot(t, root, "metamodel.yaml", "SENSITIVE-PROJECT-FILE")

	link := filepath.Join(root, project.CustomDir, "leak.yaml")
	if err := os.Symlink(filepath.Join(root, "metamodel.yaml"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	b, err := openCustomEntry(root, "leak.yaml")
	if err == nil {
		t.Fatalf("symlink to an in-project file outside custom/ was SERVED: %q", b)
	}
	if strings.Contains(string(b), "SENSITIVE") {
		t.Fatal("leaked project file content")
	}
}

// TestRootLevelCustomNotServed pins AC4 by DISCRIMINATION, not absence.
//
// The obvious test — write custom.css at the project root, assert 404 — passes
// vacuously: after TKT-IWMETE a root-level file simply is not in the custom/
// tree, so it 404s whether or not a fallback exists. Writing BOTH copies and
// asserting which one wins is a test a resurrected fallback would actually fail.
func TestRootLevelCustomNotServed(t *testing.T) {
	root := t.TempDir()
	writeProjectRoot(t, root, customCSSFile, "ROOT-VERSION")
	writeCustom(t, root, customCSSFile, "FOLDER-VERSION")

	b, err := openCustomEntry(root, customCSSFile)
	if err != nil {
		t.Fatalf("custom/custom.css should serve: %v", err)
	}
	if got := string(b); got != "FOLDER-VERSION" {
		t.Errorf("served %q, want FOLDER-VERSION — a root-level fallback is live", got)
	}
	if strings.Contains(string(b), "ROOT-VERSION") {
		t.Error("root-level file content leaked into the response")
	}
}

// TestRootLevelCustomNotInjected is the injection half of AC4: with ONLY the
// root-level file present, the shell must be the plain variant.
func TestRootLevelCustomNotInjected(t *testing.T) {
	root := t.TempDir()
	writeProjectRoot(t, root, customCSSFile, "ROOT-VERSION")
	writeProjectRoot(t, root, customJSFile, "console.log(1)")

	custom := newCustomAssets(root, []byte(testShell), func() bool { return true })
	got := string(custom.shell())

	if strings.Contains(got, customCSSTag) || strings.Contains(got, customJSTag) {
		t.Error("root-level files were injected; the old layout is still live")
	}
	if got != testShell {
		t.Error("shell differs from the stock shell with no custom/ dir present")
	}
}

// TestOpenCustomEntry_Directories pins AC11: a directory request 404s and there
// is no index resolution.
//
// NOTE (corrected after code review): an earlier comment here claimed the two
// spellings take DIFFERENT guards — "fonts" via IsDir, "fonts/" via
// fs.ValidPath. That is false. path.Clean("/fonts/") yields "/fonts", so
// validCustomEntry returns ("fonts", true) for BOTH and they die at the same
// IsDir check; fs.ValidPath never sees a trailing slash. Both spellings are
// still worth asserting (a client may send either), but do not rely on a
// second guard that is not there. TestValidCustomEntry covers the predicate.
func TestOpenCustomEntry_Directories(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, "fonts/brand.woff2", "FONT")
	// An index.html inside the dir must NOT be resolved for a dir request.
	writeCustom(t, root, "fonts/index.html", "<html>index</html>")

	for _, entry := range []string{"fonts", "fonts/"} {
		t.Run("dir request "+strconv.Quote(entry), func(t *testing.T) {
			if _, err := openCustomEntry(root, entry); err == nil {
				t.Errorf("directory request %q must 404", entry)
			}
		})
	}
	t.Run("both spellings normalise identically", func(t *testing.T) {
		a, okA := validCustomEntry("fonts")
		b, okB := validCustomEntry("fonts/")
		if a != b || okA != okB {
			t.Errorf("validCustomEntry: %q/%v vs %q/%v — spellings must normalise the same",
				a, okA, b, okB)
		}
	})
	t.Run("no index resolution", func(t *testing.T) {
		// The file itself is still addressable by its explicit path...
		if _, err := openCustomEntry(root, "fonts/index.html"); err != nil {
			t.Errorf("explicit index.html path should serve: %v", err)
		}
	})
}

// TestValidCustomEntry covers the normalisation/rejection rule in isolation, so
// a regression is attributable to the predicate rather than to the filesystem.
func TestValidCustomEntry(t *testing.T) {
	tests := []struct {
		in      string
		wantRel string
		wantOK  bool
	}{
		{"custom.css", "custom.css", true},
		{"fonts/brand.woff2", "fonts/brand.woff2", true},
		{"a/b/c/d.png", "a/b/c/d.png", true},
		{"./custom.css", "custom.css", true},
		{"data.avif", "data.avif", true},
		{"custom.css~", "custom.css~", true}, // editor backup: NOT caught (documented)
		{"", "", false},
		{".", "", false},
		{"..", "", false},
		{"/", "", false},
		{".env", "", false},
		{".env.backup", "", false},
		{".git/config", "", false},
		{"sub/.hidden", "", false},
		{"sub/.git/config", "", false},
		{".well-known/acme", "", false}, // known false positive, documented
		{"../secret", "secret", true},   // NB: cleaned, then contained by OpenRoot
	}
	for _, tt := range tests {
		t.Run(strconv.Quote(tt.in), func(t *testing.T) {
			rel, ok := validCustomEntry(tt.in)
			if ok != tt.wantOK || (ok && rel != tt.wantRel) {
				t.Errorf("validCustomEntry(%q) = (%q, %v), want (%q, %v)",
					tt.in, rel, ok, tt.wantRel, tt.wantOK)
			}
		})
	}
}

// TestOpenCustomEntry_NeverEscapes is the primary containment test (AC3).
//
// It asserts the property that matters — no traversal spelling reaches a file
// OUTSIDE custom/ — rather than the weaker "the request errors". Those are not
// the same: path.Clean ANCHORS "../secret.txt" to "/secret.txt", so the request
// resolves to custom/secret.txt and legitimately succeeds if that file exists.
// An earlier version of this test asserted rejection, created a decoy inside
// custom/, and would have passed even against a genuinely leaky implementation.
//
// So: sensitive files outside custom/, with NO same-named decoy inside, and the
// assertion is on the CONTENT never appearing.
func TestOpenCustomEntry_NeverEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, project.CustomDir), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectRoot(t, root, "outside.txt", "LEAKED-OUTSIDE")
	writeProjectRoot(t, root, "metamodel.yaml", "LEAKED-METAMODEL")

	vectors := []string{
		"../outside.txt",
		"../../outside.txt",
		"../metamodel.yaml",
		"sub/../../outside.txt",
		"/outside.txt",
		"a/b/../../../outside.txt",
		"....//outside.txt",
		"..%2Foutside.txt",
		"../../../../../../etc/passwd",
	}
	for _, v := range vectors {
		t.Run(strconv.Quote(v), func(t *testing.T) {
			b, err := openCustomEntry(root, v)
			if err == nil && strings.Contains(string(b), "LEAKED") {
				t.Errorf("LEAK: %q served %q from outside custom/", v, b)
			}
		})
	}
}

// TestServeAsset_ConditionalRequests pins the payoff of moving to
// http.ServeContent: an unchanged asset revalidates to a bodiless 304 instead
// of re-transferring. Before this, a 200KB webfont was sent in full on every
// navigation (RR-CR2-SERVECONTENT).
func TestServeAsset_ConditionalRequests(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, "logo.svg", "<svg/>")
	custom := newCustomAssets(root, []byte(testShell), func() bool { return true })

	// First request: full body plus a validator.
	rec := httptest.NewRecorder()
	custom.serveAsset(rec, httptest.NewRequest(http.MethodGet, customURLPrefix+"logo.svg", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag — conditional requests cannot work without a validator")
	}
	if rec.Body.String() != "<svg/>" {
		t.Fatalf("body = %q, want the asset", rec.Body.String())
	}

	// Second request with the validator: 304, no body.
	req := httptest.NewRequest(http.MethodGet, customURLPrefix+"logo.svg", http.NoBody)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	custom.serveAsset(rec2, req)

	if rec2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304 for an unchanged asset", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("304 carried a %d-byte body; it must be empty", rec2.Body.Len())
	}
}

// TestServeAsset_ETagChangesOnEdit is the other half: a 304 is only correct if
// editing the file invalidates the validator. A constant ETag would serve stale
// content forever.
func TestServeAsset_ETagChangesOnEdit(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, "custom.css", ".a{color:red}")
	custom := newCustomAssets(root, []byte(testShell), func() bool { return true })

	get := func() (string, string) {
		rec := httptest.NewRecorder()
		custom.serveAsset(rec, httptest.NewRequest(http.MethodGet, customURLPrefix+"custom.css", http.NoBody))
		return rec.Header().Get("ETag"), rec.Body.String()
	}

	etag1, body1 := get()

	// Rewrite with different content AND a distinct modtime — the ETag derives
	// from modtime+size, so a same-size edit within one filesystem timestamp
	// tick is the known blind spot (documented on customEntryETag).
	writeCustom(t, root, "custom.css", ".a{color:blue}!!")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(root, project.CustomDir, "custom.css"), future, future); err != nil {
		t.Fatal(err)
	}

	etag2, body2 := get()
	if body1 == body2 {
		t.Fatal("fixture did not change the content")
	}
	if etag1 == etag2 {
		t.Error("ETag unchanged after an edit — clients would serve stale content indefinitely")
	}
}

// TestServeAsset_RangeAndHEAD pins the other two things ServeContent brings.
// Previously every method received a full body, including HEAD, and Range was
// ignored.
func TestServeAsset_RangeAndHEAD(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, "fonts/brand.woff2", "0123456789")
	custom := newCustomAssets(root, []byte(testShell), func() bool { return true })
	const path = customURLPrefix + "fonts/brand.woff2"

	t.Run("range yields 206 and the requested bytes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		req.Header.Set("Range", "bytes=2-4")
		rec := httptest.NewRecorder()
		custom.serveAsset(rec, req)

		if rec.Code != http.StatusPartialContent {
			t.Errorf("status = %d, want 206", rec.Code)
		}
		if got := rec.Body.String(); got != "234" {
			t.Errorf("body = %q, want %q", got, "234")
		}
	})

	t.Run("HEAD carries headers but no body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		custom.serveAsset(rec, httptest.NewRequest(http.MethodHead, path, http.NoBody))

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("HEAD returned a %d-byte body", rec.Body.Len())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "font/woff2" {
			t.Errorf("Content-Type = %q, want font/woff2", ct)
		}
	})
}

// TestServeAsset_HeadersSurviveServeContent guards the headers this route sets
// deliberately. ServeContent writes its own Content-Type when it can infer one,
// so nosniff and the explicit type must still be what leaves the handler —
// otherwise an unknown extension could be sniffed into something executable.
func TestServeAsset_HeadersSurviveServeContent(t *testing.T) {
	root := t.TempDir()
	// .avif is NOT in appContentTypes, so it must come back as octet-stream
	// rather than whatever content sniffing would guess.
	writeCustom(t, root, "data.avif", "<html>not really avif</html>")
	custom := newCustomAssets(root, []byte(testShell), func() bool { return true })

	rec := httptest.NewRecorder()
	custom.serveAsset(rec, httptest.NewRequest(http.MethodGet, customURLPrefix+"data.avif", http.NoBody))

	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream (content must not be sniffed)", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

// TestOpenCustomEntryFile_MatchesOpenCustomEntry pins that the two openers
// agree. They share a containment chain by construction, but a divergence would
// mean the shell references an asset the handler refuses (or vice versa).
func TestOpenCustomEntryFile_MatchesOpenCustomEntry(t *testing.T) {
	root := t.TempDir()
	writeCustom(t, root, "custom.css", ".a{}")
	writeCustom(t, root, "fonts/b.woff2", "F")
	writeCustom(t, root, ".env", "SECRET=1")
	writeCustom(t, root, "big.png", strings.Repeat("x", maxCustomFileBytes+1))
	writeProjectRoot(t, root, "outside.txt", "LEAKED")

	entries := []string{
		"custom.css", "fonts/b.woff2", ".env", "big.png",
		"../outside.txt", "..%2Foutside.txt", "", ".", "..",
		"fonts", "nope.css", "sub/.git/config",
	}
	for _, e := range entries {
		t.Run(strconv.Quote(e), func(t *testing.T) {
			_, byteErr := openCustomEntry(root, e)
			fh, fileErr := openCustomEntryFile(root, e)
			if fileErr == nil {
				fh.Close()
			}
			if (byteErr == nil) != (fileErr == nil) {
				t.Errorf("openCustomEntry err=%v but openCustomEntryFile err=%v — the two openers must agree",
					byteErr, fileErr)
			}
		})
	}
}

// TestOpenCustomEntryFile_NeverEscapes attacks the ServeContent read path
// directly. openCustomEntryFile duplicates the containment chain (it must hand
// back a live handle, so it cannot simply call openCustomEntry), and a
// duplicated security check is exactly the kind that drifts. Asserts the
// property, not the error: sensitive files outside custom/ with no decoy
// inside, and the content must never appear.
func TestOpenCustomEntryFile_NeverEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, project.CustomDir), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectRoot(t, root, "outside.txt", "LEAKED-OUTSIDE")
	writeProjectRoot(t, root, "schema.yaml", "LEAKED-SCHEMA")

	// A symlink INSIDE custom/ pointing at an in-project file outside it: the
	// case a single os.OpenRoot would follow, since the target never leaves the
	// project root.
	link := filepath.Join(root, project.CustomDir, "link.yaml")
	if err := os.Symlink(filepath.Join(root, "schema.yaml"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	vectors := []string{
		"../outside.txt", "../../outside.txt", "../schema.yaml", "link.yaml",
		"sub/../../outside.txt", "/outside.txt", "....//outside.txt",
		"a/b/../../../outside.txt", "..%2Foutside.txt",
		"../../../../../../etc/passwd",
	}
	for _, v := range vectors {
		t.Run(strconv.Quote(v), func(t *testing.T) {
			fh, err := openCustomEntryFile(root, v)
			if err != nil {
				return // refused outright
			}
			defer fh.Close()
			b, _ := io.ReadAll(fh.File)
			if strings.Contains(string(b), "LEAKED") {
				t.Errorf("LEAK: %q streamed %q from outside custom/", v, b)
			}
		})
	}
}
