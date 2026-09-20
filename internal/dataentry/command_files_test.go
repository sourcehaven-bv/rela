package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// --- Token table (TKT-93FUCV) ---

func TestCommandFileStore_MintLookup(t *testing.T) {
	s := newCommandFileStore()
	cmd := CommandConfig{Label: "Export", Context: "entity", Permission: "command:allowed"}

	token, err := s.mint("run-1", "/tmp/report.pdf", "report.pdf", cmd)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if token == "" {
		t.Fatal("mint returned an empty token")
	}

	entry, ok := s.lookup(token)
	if !ok {
		t.Fatal("lookup failed for a freshly minted token")
	}
	if entry.path != "/tmp/report.pdf" {
		t.Errorf("path = %q, want /tmp/report.pdf", entry.path)
	}
	if entry.label != "report.pdf" {
		t.Errorf("label = %q, want report.pdf", entry.label)
	}
	// The stored command is what the download re-authorizes against, so it
	// must survive the round trip intact.
	if entry.cmd.Permission != "command:allowed" {
		t.Errorf("cmd.Permission = %q, want command:allowed", entry.cmd.Permission)
	}
}

func TestCommandFileStore_UnknownToken(t *testing.T) {
	s := newCommandFileStore()
	if _, ok := s.lookup("nope"); ok {
		t.Error("lookup succeeded for a token that was never minted")
	}
}

// TestCommandFileStore_TokensAreUnguessable pins that tokens are distinct and
// long. The table has no rate limit, so unguessability is what keeps the route
// from being enumerable.
func TestCommandFileStore_TokensAreUnguessable(t *testing.T) {
	s := newCommandFileStore()
	seen := make(map[string]bool)
	for range 100 {
		token, err := s.mint("run-1", "/tmp/x", "x", CommandConfig{})
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		if seen[token] {
			t.Fatalf("duplicate token minted: %q", token)
		}
		if len(token) < 40 {
			t.Fatalf("token too short to be unguessable: %q", token)
		}
		seen[token] = true
	}
}

// TestCommandFileStore_LiveRunDoesNotExpire pins that expiry starts at run
// completion, not at mint. A long-running script that emits a file early must
// keep it downloadable for the whole run.
func TestCommandFileStore_LiveRunDoesNotExpire(t *testing.T) {
	s := newCommandFileStore()
	base := time.Now()
	s.now = func() time.Time { return base }

	token, err := s.mint("run-1", "/tmp/x", "x", CommandConfig{})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// Far past the TTL, but the run never completed.
	s.now = func() time.Time { return base.Add(100 * commandFileTTL) }
	if _, ok := s.lookup(token); !ok {
		t.Error("token expired while its run was still in flight")
	}
}

// TestCommandFileStore_ExpiresAfterRelease is AC-5: a token stops resolving
// once its run has completed and the TTL has elapsed.
func TestCommandFileStore_ExpiresAfterRelease(t *testing.T) {
	s := newCommandFileStore()
	base := time.Now()
	s.now = func() time.Time { return base }

	token, err := s.mint("run-1", "/tmp/x", "x", CommandConfig{})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	s.release("run-1")

	// Still inside the window: the user has not clicked Download yet.
	s.now = func() time.Time { return base.Add(commandFileTTL - time.Second) }
	if _, ok := s.lookup(token); !ok {
		t.Error("token expired before its TTL elapsed; the Download button would be dead on arrival")
	}

	s.now = func() time.Time { return base.Add(commandFileTTL + time.Second) }
	if _, ok := s.lookup(token); ok {
		t.Error("token still resolves after run completion + TTL")
	}
}

// TestCommandFileStore_ReleaseKeepsTokensUsable pins the reason release sets a
// deadline instead of deleting: the user clicks Download AFTER the run ends.
func TestCommandFileStore_ReleaseKeepsTokensUsable(t *testing.T) {
	s := newCommandFileStore()
	token, err := s.mint("run-1", "/tmp/x", "x", CommandConfig{})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	s.release("run-1")
	if _, ok := s.lookup(token); !ok {
		t.Error("release deleted the token, breaking every button the run just rendered")
	}
}

// TestCommandFileStore_SweepDrainsExpired pins that the table does not grow
// without bound: there is no background goroutine, so minting sweeps.
func TestCommandFileStore_SweepDrainsExpired(t *testing.T) {
	s := newCommandFileStore()
	base := time.Now()
	s.now = func() time.Time { return base }

	for range 10 {
		if _, err := s.mint("old-run", "/tmp/x", "x", CommandConfig{}); err != nil {
			t.Fatalf("mint: %v", err)
		}
	}
	s.release("old-run")

	s.now = func() time.Time { return base.Add(commandFileTTL + time.Second) }
	if _, err := s.mint("new-run", "/tmp/y", "y", CommandConfig{}); err != nil {
		t.Fatalf("mint: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.byToken) != 1 {
		t.Errorf("byToken holds %d entries, want 1 — expired entries were not swept", len(s.byToken))
	}
	if _, ok := s.byRun["old-run"]; ok {
		t.Error("byRun still tracks a fully-expired run")
	}
}

// --- mintFileToken ---

// TestMintFileToken_StripsPath is AC-6: the raw server path must not reach the
// browser.
func TestMintFileToken_StripsPath(t *testing.T) {
	app := newHandlerTestApp(t)
	root := t.TempDir()
	app.paths.Root = root

	target := filepath.Join(root, "report.pdf")
	if err := os.WriteFile(target, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := app.commands.mintFileToken("run-1",
		CommandConfig{Label: "Export"},
		CommandMessage{Type: "file", Path: target, Label: "report.pdf"})

	if out.Path != "" {
		t.Errorf("outbound message still carries the server path: %q", out.Path)
	}
	if out.Token == "" {
		t.Fatal("no token minted for a file inside the project root")
	}
	if out.Label != "report.pdf" {
		t.Errorf("label = %q, want report.pdf", out.Label)
	}
}

// TestMintFileToken_LabelFallsBackToBasename pins that a script that omits
// `label` still gets a sensible filename — derived server-side, since the
// client no longer sees the path to derive it from.
func TestMintFileToken_LabelFallsBackToBasename(t *testing.T) {
	app := newHandlerTestApp(t)
	root := t.TempDir()
	app.paths.Root = root

	target := filepath.Join(root, "report.pdf")
	if err := os.WriteFile(target, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := app.commands.mintFileToken("run-1", CommandConfig{},
		CommandMessage{Type: "file", Path: target})
	if out.Label != "report.pdf" {
		t.Errorf("label = %q, want the basename report.pdf", out.Label)
	}
}

// TestMintFileToken_RejectsOutsideProject pins containment at mint time: a
// path outside the project root yields no token, so no download handle for it
// ever exists.
func TestMintFileToken_RejectsOutsideProject(t *testing.T) {
	app := newHandlerTestApp(t)
	app.paths.Root = t.TempDir()

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{"absolute path outside root", outside},
		{"traversal", "../../etc/passwd"},
		{"nonexistent inside root", "no-such-file.pdf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := app.commands.mintFileToken("run-1", CommandConfig{},
				CommandMessage{Type: "file", Path: tc.path, Label: "x"})
			if out.Token != "" {
				t.Errorf("minted a token for %q", tc.path)
			}
			if out.Path != "" {
				t.Errorf("leaked path for %q", tc.path)
			}
			// Still reported, so the user sees the script named something.
			if out.Type != "file" {
				t.Errorf("type = %q, want file", out.Type)
			}
		})
	}
}

// --- Download handler ---

// downloadFile issues a GET against the download route with the ACL request +
// read gate attached, mirroring execCommandAs.
func downloadFile(
	ctx context.Context, t *testing.T, app *App, d *acl.Declarative, token string,
) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/command-file/"+token, http.NoBody).
		WithContext(gateCtxFor(ctx, t, d))
	w := httptest.NewRecorder()
	app.commands.handleCommandFile(w, r)
	return w
}

func TestHandleCommandFile_MethodNotAllowed(t *testing.T) {
	app := newHandlerTestApp(t)
	r := httptest.NewRequest(http.MethodPost, "/api/command-file/tok", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandFile(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleCommandFile_UnknownToken(t *testing.T) {
	app := newHandlerTestApp(t)
	r := httptest.NewRequest(http.MethodGet, "/api/command-file/nope", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandFile(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for an unknown token, got %d", w.Code)
	}
}

// TestHandleCommandFile_ServesHardenedDownload pins that the bytes come back
// with the same hardening attachment and export downloads use. Command output
// is script-authored, so it is exactly the user-influenced content that block
// exists for.
func TestHandleCommandFile_ServesHardenedDownload(t *testing.T) {
	app := newHandlerTestApp(t)
	root := t.TempDir()
	app.paths.Root = root

	target := filepath.Join(root, "report.txt")
	if err := os.WriteFile(target, []byte("hello from the script"), 0o600); err != nil {
		t.Fatal(err)
	}

	token, err := app.commands.files.mint("run-1", target, "report.txt", CommandConfig{})
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/command-file/"+token, http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandFile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "hello from the script" {
		t.Errorf("body = %q", got)
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "sandbox; default-src 'none'",
		"Cache-Control":           "no-store",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("Content-Disposition = %q, want a forced attachment", cd)
	}
}

// TestHandleCommandFile_MissingFileIs404 covers a script that cleaned up after
// itself: the token is valid but the bytes are gone.
func TestHandleCommandFile_MissingFileIs404(t *testing.T) {
	app := newHandlerTestApp(t)
	root := t.TempDir()
	app.paths.Root = root

	token, err := app.commands.files.mint("run-1", filepath.Join(root, "gone.txt"), "gone.txt", CommandConfig{})
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/command-file/"+token, http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandFile(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when the file is gone, got %d", w.Code)
	}
}

// TestHandleCommandFile_ReauthorizesPerDownload is AC-4, the reason the route
// is shaped this way: a token is a capability, and holding one is not
// authorization. The SAME token is minted once under a grant, then presented
// again after the grant is gone.
func TestHandleCommandFile_ReauthorizesPerDownload(t *testing.T) {
	mintUnder := func(t *testing.T) (*App, *acl.Declarative, string) {
		t.Helper()
		app := newHandlerTestApp(t)
		bindRepo(app, t.TempDir())
		root := t.TempDir()
		app.paths.Root = root

		target := filepath.Join(root, "report.txt")
		if err := os.WriteFile(target, []byte("secret report"), 0o600); err != nil {
			t.Fatal(err)
		}

		d := commandPolicyACL(t, app)
		app.acl = d
		cmd := CommandConfig{Label: "Export", Context: "entity", Permission: "command:allowed"}
		token, err := app.commands.files.mint("run-1", target, "report.txt", cmd)
		if err != nil {
			t.Fatal(err)
		}
		return app, d, token
	}

	t.Run("holder of the permission downloads", func(t *testing.T) {
		app, d, token := mintUnder(t)
		w := downloadFile(principalCtx("alice"), t, app, d, token)
		if w.Code != http.StatusOK {
			t.Fatalf("alice holds command:allowed, expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != "secret report" {
			t.Errorf("body = %q", w.Body.String())
		}
	})

	t.Run("principal without the permission is refused", func(t *testing.T) {
		app, d, token := mintUnder(t)
		w := downloadFile(principalCtx("bob"), t, app, d, token)
		if w.Code != http.StatusNotFound {
			t.Fatalf("bob holds nothing, expected 404, got %d", w.Code)
		}
		if strings.Contains(w.Body.String(), "secret report") {
			t.Error("refused download still streamed the file")
		}
	})

	t.Run("read-only restart kills a valid token", func(t *testing.T) {
		app, d, token := mintUnder(t)
		// The grant that minted the token is gone. The token itself is
		// untouched and unexpired — only the live policy changed.
		app.acl = acl.ReadOnlyACL{}
		w := downloadFile(principalCtx("alice"), t, app, d, token)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 under --read-only, got %d", w.Code)
		}
		if strings.Contains(w.Body.String(), "secret report") {
			t.Error("--read-only download still streamed the file")
		}
	})
}

// TestCommandExecEmitsTokenNotPath is the integration case: a real run emits a
// `file` message, and what reaches the SSE stream is a token with no path.
func TestCommandExecEmitsTokenNotPath(t *testing.T) {
	app := newHandlerTestApp(t)
	root := t.TempDir()
	bindRepo(app, root)
	app.paths.Root = root

	target := filepath.Join(root, "out.txt")
	if err := os.WriteFile(target, []byte("generated"), 0o600); err != nil {
		t.Fatal(err)
	}

	app.Cfg().Commands = map[string]CommandConfig{
		"gen": {
			Label:   "Gen",
			Script:  `echo '::rela::{"type":"file","path":"` + target + `","label":"out.txt"}'`,
			Context: "global",
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/api/command/gen", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandExec(w, r)

	body := w.Body.String()
	if !strings.Contains(body, `"token"`) {
		t.Fatalf("SSE stream carries no download token:\n%s", body)
	}
	if strings.Contains(body, target) {
		t.Errorf("SSE stream leaks the server path:\n%s", body)
	}

	// The token the stream handed out must actually resolve to the bytes.
	token := tokenFromSSE(t, body)
	dr := httptest.NewRequest(http.MethodGet, "/api/command-file/"+token, http.NoBody)
	dw := httptest.NewRecorder()
	app.commands.handleCommandFile(dw, dr)
	if dw.Code != http.StatusOK {
		t.Fatalf("download of the emitted token failed: %d", dw.Code)
	}
	if dw.Body.String() != "generated" {
		t.Errorf("downloaded %q, want the file's contents", dw.Body.String())
	}
}

// tokenFromSSE pulls the token out of the first `file` data frame.
func tokenFromSSE(t *testing.T, body string) string {
	t.Helper()
	for line := range strings.SplitSeq(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || !strings.Contains(data, `"token"`) {
			continue
		}
		var msg CommandMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			t.Fatalf("parsing SSE frame %q: %v", data, err)
		}
		return msg.Token
	}
	t.Fatalf("no token frame in SSE body:\n%s", body)
	return ""
}
