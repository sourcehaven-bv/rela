package docs

import (
	"context"
	"strings"
	"testing"
	"time"
)

// slowCapturer takes longer than one Tier-A buildTimeout to return, the way a
// real capture does when it has to retry a cold-start failure.
type slowCapturer struct{ delay time.Duration }

func (s slowCapturer) Capture(ctx context.Context, _ CaptureSpec) (string, error) {
	select {
	case <-time.After(s.delay):
		return "x.png", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
func (slowCapturer) Close() error { return nil }

// A screenshot island must get the SCREENSHOT build's ceiling, not the bare
// Tier-A buildTimeout.
//
// gopher-lua's SetContext aborts an island on its own deadline, so passing a
// hardcoded buildTimeout to rlua.WithTimeout silently overrode the wider
// ceiling a screenshot build had already chosen: one screenshot{} got 30s no
// matter what screenshotBuildTimeout said. The symptom was
// `manual:NNN: lua: context deadline exceeded` — blamed on the manual, and
// impossible to fix from the capture layer, because the island was killed
// before Capture could return an error worth retrying.
//
// The delay here is deliberately just over buildTimeout: short enough to keep
// the test quick, long enough that the old wiring fails it.
func TestScreenshotIsland_GetsTheScreenshotCeiling(t *testing.T) {
	t.Parallel()

	src := "```rela\nscreenshot{view=\"form\", type=\"ticket\", entity=\"TICKET-1\", out=\"f.png\"}\n```\n"

	out, err := Build(context.Background(), src, Options{
		Meta:     fixtureMeta(t),
		Capturer: slowCapturer{delay: buildTimeout + 2*time.Second},
	})
	if err != nil {
		t.Fatalf("a capture slower than the Tier-A ceiling must still be allowed: %v", err)
	}
	if !strings.Contains(out, "x.png") {
		t.Errorf("expected the figure in the output, got: %q", out)
	}
}

// The ceiling still has to BITE. A screenshot build must not become unbounded:
// a capture that outlives the screenshot ceiling has to fail, or a hung browser
// would run until CI's job timeout with no attribution.
func TestScreenshotIsland_CeilingStillBounds(t *testing.T) {
	t.Parallel()

	src := "```rela\nscreenshot{view=\"form\", type=\"ticket\", entity=\"TICKET-1\", out=\"f.png\"}\n```\n"

	// An already-expired caller deadline stands in for "past the ceiling": Build
	// keeps the caller's earlier deadline, so this exercises the same bound
	// without a five-minute test.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := Build(ctx, src, Options{
		Meta:     fixtureMeta(t),
		Capturer: slowCapturer{delay: time.Minute},
	})
	if err == nil {
		t.Fatal("a capture past the ceiling must fail the build, not hang")
	}
}

// The tier constants must stay ordered, and the screenshot ceiling must leave
// room for docscapture's bounded retry. Without that headroom the retry cannot
// run, which is the defect above in constant form.
func TestBuildTimeoutTiers(t *testing.T) {
	t.Parallel()

	if apiBuildTimeout <= buildTimeout {
		t.Errorf("apiBuildTimeout (%s) must exceed buildTimeout (%s)", apiBuildTimeout, buildTimeout)
	}
	if screenshotBuildTimeout <= apiBuildTimeout {
		t.Errorf("screenshotBuildTimeout (%s) must exceed apiBuildTimeout (%s)", screenshotBuildTimeout, apiBuildTimeout)
	}
	// docscapture retries a capture up to 3 times at perCaptureTimeout (30s)
	// each. The ceiling must cover that, or the retry is dead code.
	if need := 3 * 30 * time.Second; screenshotBuildTimeout < need {
		t.Errorf("screenshotBuildTimeout (%s) leaves no room for docscapture's bounded retry (needs >= %s)",
			screenshotBuildTimeout, need)
	}
}

// Widening the per-island cap must not let N islands escape the build-wide
// ceiling. The total is bounded by Build's own child context, so many slow
// islands still stop at the ceiling rather than multiplying it.
func TestScreenshotIslands_TotalStillBounded(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	for range 5 {
		src.WriteString("```rela\nscreenshot{view=\"form\", type=\"ticket\", entity=\"TICKET-1\", out=\"f.png\"}\n```\n")
	}

	// A caller deadline well under 5 x the per-capture delay: if the per-island
	// cap were the only bound, this would run far past it.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := Build(ctx, src.String(), Options{
		Meta:     fixtureMeta(t),
		Capturer: slowCapturer{delay: 3 * time.Second},
	})
	if err == nil {
		t.Fatal("the build-wide ceiling must still bound the total")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("build ran %s: the per-island cap replaced the total bound instead of sitting under it", elapsed)
	}
}
