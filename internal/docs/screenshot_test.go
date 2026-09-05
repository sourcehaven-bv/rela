package docs

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A screenshot{} island with no Capturer wired fails loud (browser support
// absent) — testable without any browser (DR-S3: the fail-loud paths run in
// standard CI).
func TestScreenshot_NoCapturer_FailsLoud(t *testing.T) {
	t.Parallel()
	src := "```rela\nscreenshot{ type=\"risico\", entity=\"r1\", out=\"f.png\" }\n```\n"
	_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t)})
	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("want *BuildError, got %T: %v", err, err)
	}
	if !strings.Contains(be.Msg, "browser") {
		t.Errorf("error should mention the missing browser: %q", be.Msg)
	}
}

// A view with no readiness marker fails loud rather than hanging.
//
// The supported set is an ALLOWLIST, not a passthrough: each entry routes in
// the SPA and stamps a `form-state-*` / `page-state-*` marker the capture
// polls. A view without one gives the gate nothing to wait for, so it could
// only time out — a slow, confusing failure in place of an immediate, clear
// one. (Before the list/entity/create/search views grew markers, this test
// asserted that only `form` was allowed.)
func TestScreenshot_UnsupportedView_FailsLoud(t *testing.T) {
	t.Parallel()
	for _, view := range []string{"nonsense", "settings", "conflicts", "relation-history"} {
		src := "```rela\nscreenshot{ view=\"" + view + "\", type=\"risico\", entity=\"r1\", out=\"f.png\" }\n```\n"
		_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), Capturer: stubCapturer{}})
		if err == nil || !strings.Contains(err.Error(), "unknown view") {
			t.Errorf("view=%q should fail loud, got: %v", view, err)
		}
	}
}

// Each supported view demands the arguments it actually needs, and says which.
func TestScreenshot_PerViewRequiredArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		island  string
		wantErr string
	}{
		{
			name:    "a list needs a list id, not an entity",
			island:  `screenshot{ view="list", out="f.png" }`,
			wantErr: "`list` is required",
		},
		{
			// An idle search screen shows nothing for a reason unrelated to
			// whatever the figure is illustrating.
			name:    "a search needs a query",
			island:  `screenshot{ view="search", out="f.png" }`,
			wantErr: "`q` is required",
		},
		{
			name:    "a create form needs a type or a form id",
			island:  `screenshot{ view="create", out="f.png" }`,
			wantErr: "`type` or `form` is required",
		},
		{
			name:    "a detail view needs an entity",
			island:  `screenshot{ view="entity", type="risico", out="f.png" }`,
			wantErr: "`entity` is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "```rela\n" + tc.island + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), Capturer: stubCapturer{}})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// Missing required args fail loud (also without a browser).
func TestScreenshot_MissingArgs_FailLoud(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"no type":   "```rela\nscreenshot{ entity=\"r1\", out=\"f.png\" }\n```\n",
		"no entity": "```rela\nscreenshot{ type=\"risico\", out=\"f.png\" }\n```\n",
		"no out":    "```rela\nscreenshot{ type=\"risico\", entity=\"r1\" }\n```\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// A stub capturer so we get past the nil check to the arg checks.
			_, err := Build(context.Background(), src, Options{
				Meta: fixtureMeta(t), Capturer: stubCapturer{},
			})
			if err == nil {
				t.Fatalf("%s: expected a BuildError", name)
			}
		})
	}
}

// A manual with NO screenshot{} never constructs/consults a Capturer (AC9).
func TestBuild_NoScreenshot_CapturerUntouched(t *testing.T) {
	t.Parallel()
	stub := &countingCapturer{}
	src := "```rela\ntyperef{type=\"risico\", fields=\"required\"}\n```\n"
	if _, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), Capturer: stub}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if stub.calls != 0 {
		t.Errorf("Capturer.Capture called %d times for a screenshot-free manual", stub.calls)
	}
}

// stubCapturer satisfies Capturer without a browser (arg-validation tests).
type stubCapturer struct{}

func (stubCapturer) Capture(context.Context, CaptureSpec) (string, error) { return "x.png", nil }
func (stubCapturer) Close() error                                         { return nil }

type countingCapturer struct{ calls int }

func (c *countingCapturer) Capture(context.Context, CaptureSpec) (string, error) {
	c.calls++
	return "x.png", nil
}
func (c *countingCapturer) Close() error { return nil }
