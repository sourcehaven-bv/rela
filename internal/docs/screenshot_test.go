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
