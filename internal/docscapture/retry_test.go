package docscapture

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// retryableCapture decides whether a failed capture gets another attempt. Its
// two jobs pull in opposite directions, so both are pinned here: a transient
// cold-start failure must be retried (that is the whole point), and a
// deterministic one must NOT be, or a genuinely broken figure costs three
// attempts and reports the same error three times slower.
func TestRetryableCapture(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil is not a failure", nil, false},
		{
			"the renderability gate's error state is the document's fault",
			fmt.Errorf("capture: %w", errPageLoadFailed),
			false,
		},
		{
			"a region over the height cap needs a tighter clip, not a retry",
			fmt.Errorf("capture: %w", errRegionTooTall),
			false,
		},
		{
			"an unknown as= role is a typo in the manual",
			fmt.Errorf("resolve: %w", errUnknownRole),
			false,
		},
		{
			"the observed CI dial timeout",
			errors.New(`could not dial "ws://127.0.0.1:35025/devtools/browser/af23": context deadline exceeded`),
			true,
		},
		{
			"the observed CI capture timeout",
			fmt.Errorf("capture: %w", context.DeadlineExceeded),
			true,
		},
		{
			"an unrecognized failure is retried, since the deny list is the closed set",
			errors.New("websocket: close 1006 (abnormal closure)"),
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryableCapture(tc.err); got != tc.want {
				t.Errorf("retryableCapture(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// The sentinels must survive the wrapping each one actually gets in the capture
// path. A sentinel that stops matching once wrapped silently turns a
// deterministic failure back into a retried one.
func TestCaptureSentinelsSurviveWrapping(t *testing.T) {
	for _, target := range []error{errPageLoadFailed, errRegionTooTall, errUnknownRole} {
		wrapped := fmt.Errorf("capture: %w", fmt.Errorf("outer: %w", target))
		if !errors.Is(wrapped, target) {
			t.Errorf("errors.Is lost %v through two levels of wrapping", target)
		}
	}
}

// withFastRetries removes the inter-attempt pause for the duration of a test,
// so the retry policy is tested at full speed.
func withFastRetries(t *testing.T) {
	t.Helper()
	prev := captureRetryPause
	captureRetryPause = time.Millisecond
	t.Cleanup(func() { captureRetryPause = prev })
}

// The loop's contract, stated as attempt counts. These are what actually fix
// the CI flake — the classifier above only decides, this is what acts on it.
func TestCapture_RetryPolicy(t *testing.T) {
	withFastRetries(t)

	tests := []struct {
		name string
		// fail returns the error for attempt n (1-based); nil means success.
		fail         func(n int) error
		wantAttempts int
		wantErr      bool
	}{
		{
			name:         "a first-attempt success does not retry",
			fail:         func(int) error { return nil },
			wantAttempts: 1,
		},
		{
			name: "a transient failure that clears is invisible to the caller",
			fail: func(n int) error {
				if n == 1 {
					return context.DeadlineExceeded
				}
				return nil
			},
			wantAttempts: 2,
		},
		{
			name: "the cold-start case: two transient failures still succeed",
			fail: func(n int) error {
				if n <= 2 {
					return errors.New("could not dial ws://127.0.0.1:1/devtools/browser")
				}
				return nil
			},
			wantAttempts: 3,
		},
		{
			name:         "a persistently transient failure exhausts the budget and fails",
			fail:         func(int) error { return context.DeadlineExceeded },
			wantAttempts: captureAttempts,
			wantErr:      true,
		},
		{
			name:         "a deterministic failure fails on the FIRST attempt",
			fail:         func(int) error { return fmt.Errorf("capture: %w", errPageLoadFailed) },
			wantAttempts: 1,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got int
			c := &Capturer{}
			c.attempt = func(context.Context, docs.CaptureSpec) (string, error) {
				got++
				if err := tc.fail(got); err != nil {
					return "", err
				}
				return "out.png", nil
			}

			path, err := c.Capture(context.Background(), docs.CaptureSpec{})
			if got != tc.wantAttempts {
				t.Errorf("ran %d attempts, want %d", got, tc.wantAttempts)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			if path != "out.png" {
				t.Errorf("path = %q, want out.png", path)
			}
		})
	}
}

// A failing capture must surface the REASON, not a generic "gave up after 3
// attempts" — the error is what a doc author reads to fix their figure.
func TestCapture_ExhaustedReportsLastError(t *testing.T) {
	withFastRetries(t)

	sentinel := errors.New("could not dial ws://127.0.0.1:1/devtools/browser")
	c := &Capturer{attempt: func(context.Context, docs.CaptureSpec) (string, error) {
		return "", sentinel
	}}

	_, err := c.Capture(context.Background(), docs.CaptureSpec{})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want it to wrap %v", err, sentinel)
	}
}

// A cancelled build must stop retrying immediately rather than sleeping out its
// remaining attempts: ctrl-C during a docs build should not take seconds per
// figure to take effect.
func TestCapture_CancelledContextStopsRetrying(t *testing.T) {
	prev := captureRetryPause
	captureRetryPause = time.Hour // a retry that waits would hang the test
	t.Cleanup(func() { captureRetryPause = prev })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var got int
	c := &Capturer{attempt: func(context.Context, docs.CaptureSpec) (string, error) {
		got++
		return "", context.DeadlineExceeded
	}}

	done := make(chan struct{})
	go func() {
		_, _ = c.Capture(ctx, docs.CaptureSpec{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Capture kept waiting on a cancelled context")
	}
	if got != 1 {
		t.Errorf("ran %d attempts on a cancelled context, want 1", got)
	}
}

// The retry path must work against a REAL browser, not just the mocked seam.
//
// The seam tests above pin the control flow, but they cannot catch the thing
// that actually broke during this fix: resetBrowser cancels the chromedp
// context, so if the relaunch is wired wrong every retry fails with "context
// canceled" instead of retrying. That is invisible to a mock and fatal in CI.
//
// This forces one genuine transient failure by tearing the browser down
// mid-flight, then asserts the capture still produces a PNG.
func TestCapture_RetriesAgainstRealBrowser(t *testing.T) {
	requireBrowser(t)
	withFastRetries(t)

	capr, err := New(NewSharedProject(""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer capr.Close()

	spec := docs.CaptureSpec{
		ProjectDir: protoDir(t),
		Seed: []docs.SeedOp{{
			Kind: "create", Type: "ticket", ID: "TICKET-retry",
			Properties: map[string]any{
				"title": "Retried capture", "status": "in-progress",
				"priority": "high", "reporter": "retry@example.com",
			},
		}},
		View: "form", Type: "ticket", Entity: "TICKET-retry",
		OutPath: filepath.Join(t.TempDir(), "retry.png"),
	}

	// Fail the first attempt the way a loaded runner does: run the real capture
	// but kill the browser out from under it, so the error is a genuine
	// chromedp failure rather than a synthetic one.
	var attempts int
	capr.attempt = func(ctx context.Context, s docs.CaptureSpec) (string, error) {
		attempts++
		if attempts == 1 {
			if ensureErr := capr.ensure(ctx, s); ensureErr != nil {
				return "", ensureErr
			}
			capr.browser.close() // the tab dies mid-capture
			return capr.captureOnce(ctx, s)
		}
		return capr.captureOnce(ctx, s)
	}

	png, err := capr.Capture(context.Background(), spec)
	if err != nil {
		t.Fatalf("Capture did not recover from a transient browser failure: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected the first attempt to fail and retry, but ran %d attempt(s)", attempts)
	}
	assertPNG(t, png)
}
