package dataentry

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestRestish_DiscoversSpecAndRoundTripsAttachment drives a real restish
// binary against the router (TKT-XO9LQB). The in-repo spec tests check the
// spec's structure; only a real client shows that it finds the spec from the
// origin alone, turns it into commands, and picks the raw request body for an
// upload.
//
// Skipped when restish is not installed, except in CI, which sets
// RELA_TEST_RESTISH=1 after installing a pinned release so a missing binary
// fails instead of passing vacuously.
// stubSPA stands in for the frontend build, which the CI Test job does not
// produce. restish reads the root's Link header only on a 2xx, and production
// refuses to start without the build (CheckEmbeddedSPA), so a root that
// answers 200 is the real precondition.
var stubSPA = fstest.MapFS{"index.html": {Data: []byte("<!doctype html><html><head></head><body></body></html>")}}

func TestRestish_DiscoversSpecAndRoundTripsAttachment(t *testing.T) {
	bin, err := exec.LookPath("restish")
	if err != nil {
		if os.Getenv("RELA_TEST_RESTISH") == "1" {
			t.Fatal("RELA_TEST_RESTISH=1 but restish is not on PATH")
		}
		t.Skip("restish not installed")
	}

	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	srv := httptest.NewServer(app.NewRouter(withSPAFS(stubSPA)))
	t.Cleanup(srv.Close)

	// Keep restish's config and spec cache out of the developer's home.
	home := t.TempDir()
	env := append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+home+"/config",
		"XDG_CACHE_HOME="+home+"/cache",
		"RSH_CONFIG_DIR="+home+"/restish",
	)
	run := func(t *testing.T, stdin string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Env = env
		cmd.Stdin = strings.NewReader(stdin)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("restish %s: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout.String(), stderr.String())
		}
		return stdout.Bytes()
	}

	// Only the origin: the spec is found through the root's Link header.
	// connect exits 0 without a spec, so check the commands it produced.
	run(t, "", "api", "connect", "rela", srv.URL, "--yes")
	if help := run(t, "", "rela", "--help"); !bytes.Contains(help, []byte("put-ticket-attachment")) {
		t.Fatalf("discovery found no spec at %s; restish rela --help:\n%s", srv.URL, help)
	}

	const content = "evidence for TKT-001\nsecond line\n"
	run(t, content, "rela", "put-ticket-attachment", "TKT-001", "screenshot", "--filename", "shot.txt")

	got := run(t, "", "rela", "get-ticket-attachment", "TKT-001", "screenshot", "shot.txt")
	if string(got) != content {
		t.Fatalf("downloaded %q, uploaded %q", got, content)
	}
}
