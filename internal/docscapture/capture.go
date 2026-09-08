package docscapture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

const (
	// perCaptureTimeout bounds one navigate+render+capture.
	perCaptureTimeout = 30 * time.Second
	// maxFullHeight caps a full-page capture; a taller page is a build error, not
	// a silent truncation (DR-M2).
	maxFullHeight = 4000
	// settleDelay lets the SPA finish its post-mount render before capture.
	settleDelay = 400 * time.Millisecond
	// pollInterval is how often the renderability gate checks the form's load state.
	pollInterval = 100 * time.Millisecond
	// viewportW/H give the capture a stable, wide-enough viewport so form layout
	// is deterministic across machines.
	viewportW = 1280
	// 16:9 — the ratio a reader's own window is closest to, so a figure looks
	// like the app rather than like a screenshot tool's idea of a page. It was
	// 1280x1600 (4:5, portrait), which made every capture of a short screen
	// mostly empty: a three-row list rendered as a tall strip with the content
	// at the top. Full-page capture still extends BEYOND this when a page is
	// genuinely longer, so nothing is cut — this only sets the floor.
	viewportH = 720
)

// Capturer implements docs.Capturer using chromedp against a data-entry SPA
// served over a seeded temp project. The temp project + server + browser are
// created lazily on the first Capture and reused across islands; Close tears
// them all down.
type Capturer struct {
	// shared is the DOCUMENT's temp project, shared with the api{} client so a
	// figure can photograph what an earlier api{} island wrote. The Capturer
	// does not own it: the wiring site creates one per document and closes it,
	// which is what keeps a scratch schema per document rather than per
	// consumer. See SharedProject.
	shared *SharedProject
	// proj is the acquired project for the current capture, cached so the rest
	// of this file reads it directly. It is the SAME pointer the shared holder
	// owns — the Capturer must never close it.
	proj    *project
	browser *browser
}

// New returns a Capturer. It does NOT launch a browser yet (that happens on the
// first Capture) so a manual with no screenshot{} pays nothing. It DOES verify a
// Chrome binary is resolvable, so a screenshot-bearing manual fails loud early
// rather than after standing up a server.
func New(shared *SharedProject) (*Capturer, error) {
	if _, ok := hasChrome(); !ok {
		return nil, errors.New("no Chrome/Chromium browser found on PATH — screenshot{} requires a browser")
	}
	if shared == nil {
		return nil, errors.New("docscapture.New: a SharedProject is required — it is the " +
			"document-scoped temp project the api{} client also uses")
	}
	return &Capturer{shared: shared}, nil
}

// Capture renders one screenshot and writes the PNG, returning its path.
func (c *Capturer) Capture(ctx context.Context, spec docs.CaptureSpec) (string, error) {
	if err := c.ensure(ctx, spec); err != nil {
		return "", err
	}

	// Derive the per-capture deadline from the browser's chromedp context (its
	// tab), not the incoming ctx — chromedp actions must run against the browser
	// context.
	cctx, cancel := context.WithTimeout(c.browser.ctx, perCaptureTimeout)
	defer cancel()

	if err := c.proj.requireKnownRole(spec.As); err != nil {
		return "", err
	}
	url := captureURL(trimSlash(c.proj.server.URL), spec)

	// Wait for the version rows BEFORE opening the page, so the single render
	// the capture photographs already has them. Waiting afterwards would mean
	// re-navigating a live SPA mid-capture, which races Chrome's execution
	// context; waiting first keeps the capture itself a plain one-shot load.
	if err := c.awaitVersions(ctx, spec); err != nil {
		return "", err
	}

	var png []byte
	actions := []chromedp.Action{
		// Start every capture from a blank page.
		//
		// One browser tab is reused for the whole build, so without this each
		// navigation is a CLIENT-SIDE route transition and SPA state carries
		// over between figures. That is invisible for a few captures and then
		// bites: a screen whose load is driven by an `immediate` watcher on the
		// route query does not re-run when the query it is mounted with equals
		// the one already in the store, so the page never leaves `pending` and
		// the capture waits out its timeout. Reproduced at the sixth capture of
		// a six-figure manual, independent of which screens preceded it.
		//
		// about:blank forces a real document teardown, so the next Navigate is
		// a full load with a fresh Vue app — the same state a reader's browser
		// would be in when they open the link.
		chromedp.Navigate("about:blank"),
		// Deterministic viewport so form layout is stable across machines.
		deviceMetricsOverride(),
		// Thread the requested role to the per-request principal resolver.
		network.SetExtraHTTPHeaders(network.Headers(map[string]any{roleHeader: spec.As})),
		chromedp.Navigate(url),
		// Renderability gate (DR-S4): wait until the form stamps a terminal
		// load state, then fail loud if it was an error. This races load vs
		// error in one poll — no dependency on which schema fields render or on
		// a timing-sensitive toast, and a load failure short-circuits instead of
		// eating the whole capture timeout.
		renderabilityGate(),
		chromedp.Sleep(settleDelay),
		// Grow the VIEWPORT to the content before capturing.
		//
		// CaptureBeyondViewport photographs the full scroll height, but the app
		// shell is laid out against the viewport: a sidebar with height:100% and
		// a position:fixed footer stop at the viewport edge. On a page taller
		// than 720px that produced a figure whose lower half had no sidebar and
		// a footer bar cutting across the middle of the form.
		//
		// Resizing first makes the layout itself full-height, so what is
		// photographed is a page the app would actually render at that size.
		fitViewportToContent(),
		chromedp.Sleep(settleDelay),
	}
	if len(spec.Arrows) > 0 {
		actions = append(actions, annotateAction(spec.Arrows))
	}
	actions = append(actions, captureAction(spec, &png))

	//nolint:contextcheck // chromedp actions bind to the browser context (cctx), not the caller ctx
	if err := chromedp.Run(cctx, actions...); err != nil {
		return "", fmt.Errorf("capture: %w", err)
	}
	if dir := filepath.Dir(spec.OutPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create image dir: %w", err)
		}
	}
	if err := os.WriteFile(spec.OutPath, png, 0o644); err != nil {
		return "", fmt.Errorf("write png: %w", err)
	}
	return spec.OutPath, nil
}

// ensure acquires the document's shared temp project (standing it up on first
// use, and applying any seed ops added since the last island) and launches the
// browser once.
//
// The project comes from the SHARED holder rather than a private standUp, so a
// capture renders the same store an earlier api{} island wrote to. The browser
// stays private: it is this Capturer's, and nothing else needs it.
func (c *Capturer) ensure(ctx context.Context, spec docs.CaptureSpec) error {
	p, err := c.shared.acquire(ctx, spec.ProjectDir, spec.Seed, true)
	if err != nil {
		return err
	}
	c.proj = p
	if c.browser == nil {
		b, berr := newBrowser(ctx)
		if berr != nil {
			return berr
		}
		c.browser = b
	}
	return nil
}

// Close tears down the browser only.
//
// The temp project and server are NOT closed here: they belong to the
// document's SharedProject, which the api{} client is still using and which
// the wiring site closes when the document finishes. Closing them here would
// tear the store out from under a later api{} island — the mirror of the bug
// that made sharing necessary in the first place.
func (c *Capturer) Close() error {
	if c.browser != nil {
		c.browser.close()
	}
	c.proj = nil
	return nil
}

// rect is a page-coordinate bounding box (CSS px).
type rect struct{ X, Y, W, H float64 }

// captureAction produces a PNG (lossless): the full page, or a clipped region
// (an element / a keyword-computed region) expanded by pad and clamped to the
// page. All paths go through page.CaptureScreenshot (PNG + CaptureBeyondViewport),
// so a clip taller than the viewport still renders and padding is honored —
// chromedp.Screenshot cannot pad.
func captureAction(spec docs.CaptureSpec, out *[]byte) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		page0, err := pageSize(ctx)
		if err != nil {
			return err
		}
		clip := rect{X: 0, Y: 0, W: page0.W, H: page0.H}
		if spec.Clip != "" {
			region, rerr := resolveClip(ctx, spec.Clip, spec.Arrows)
			if rerr != nil {
				return rerr
			}
			clip = padAndClamp(region, float64(spec.Pad), page0)
		}
		if clip.H > maxFullHeight {
			return fmt.Errorf("capture region is %.0fpx tall (> %dpx cap); use a tighter clip= selector or clip=\"focus\"", clip.H, maxFullHeight)
		}
		buf, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithClip(&page.Viewport{X: clip.X, Y: clip.Y, Width: clip.W, Height: clip.H, Scale: 1}).
			WithCaptureBeyondViewport(true).
			Do(ctx)
		if err != nil {
			return err
		}
		*out = buf
		return nil
	})
}

// renderabilityGate blocks until the form reaches a terminal load state and
// fails loud if that state is an error (DR-S4). The form root stamps
// data-testid="form-state-{pending|loaded|error}" off loadEntity's outcome, so
// this is unambiguous: it distinguishes a form rendered WITH its entity's data
// from an empty schema-only shell left after a failed load — the fail-OPEN hole
// the spike hit. Polling races load vs error, so a failure short-circuits rather
// than eating the capture timeout.
func renderabilityGate() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var state string
		// Two marker families, one contract. `form-state-*` is DynamicForm's;
		// `page-state-*` is the same signal on the list, detail and search
		// screens. Matching either keeps one gate for every view rather than a
		// per-view poll that could disagree about what "loaded" means.
		if err := chromedp.Poll(
			`(function(){`+
				`var e=document.querySelector('[data-testid^="form-state-"],[data-testid^="page-state-"]');`+
				`if(!e)return null;`+
				`var v=e.getAttribute('data-testid');`+
				`var s=v.slice(v.indexOf('-state-')+7);`+
				`return s==='pending'?null:s;})()`,
			&state,
			chromedp.WithPollingInterval(pollInterval),
		).Do(ctx); err != nil {
			return fmt.Errorf("waiting for the page to load: %w", err)
		}
		if state == "error" {
			return errors.New("the page failed to load — check the entity id, the form's field set, and the `as` role's read access")
		}
		return nil
	})
}

// deviceMetrics gives the capture a stable, wide-enough viewport so form layout
// is deterministic across machines.
func deviceMetricsOverride() chromedp.Action {
	return emulation.SetDeviceMetricsOverride(viewportW, viewportH, 1.0, false)
}

// fitViewportToContent grows the emulated viewport to the page's full scroll
// height, so viewport-relative chrome (a full-height sidebar, a fixed footer)
// lays out against the whole page rather than the first screenful.
//
// Width is left alone: only height varies with content, and changing width
// would alter the responsive breakpoint a figure is meant to show. The height
// is capped at maxFullHeight — the same ceiling the capture itself enforces, so
// an unbounded page fails there with its actionable message rather than here.
func fitViewportToContent() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		page0, err := pageSize(ctx)
		if err != nil {
			return err
		}
		h := int64(page0.H)
		if h <= viewportH {
			return nil // already tall enough; leave the 16:9 floor alone
		}
		if h > maxFullHeight {
			h = maxFullHeight
		}
		return emulation.SetDeviceMetricsOverride(viewportW, h, 1.0, false).Do(ctx)
	})
}

// versionPollInterval is how often awaitVersions re-asks the history API while
// the debounced sweep catches up; versionAwaitTimeout is how long it keeps
// asking before failing the island. The bound is the wait's own, not the
// caller's: a docs build runs under a signal context with no deadline, so a
// manual claiming more versions than will ever exist would otherwise spin
// until CI's job timeout.
const (
	versionPollInterval = 500 * time.Millisecond
	versionAwaitTimeout = 60 * time.Second
)

// awaitVersions blocks until the entity's history timeline holds at least
// spec.AwaitVersions versions. A zero count is a no-op.
//
// # Why this waits at all
//
// On the postgres backend create/update versions are written by a DEBOUNCED
// reconciliation sweep, not synchronously with the write. So a history page
// opened immediately after an edit legitimately renders an empty timeline: the
// versions exist in the future, not yet in the database. Capturing then
// photographs "No versions recorded yet" under a caption promising a history —
// the manual would be wrong, and wrong only on some machines, which is worse
// than failing.
//
// # Why it asks the API rather than counting rows on the page
//
// The history view fetches its timeline once on mount and never re-fetches, so
// a DOM poll would spin against a page that was never going to change, and
// re-navigating to refresh it races Chrome's execution context mid-capture.
// Asking the server settles the question before the browser is involved at all,
// which leaves the capture a plain one-shot load.
//
// # Why falling short is an ERROR
//
// The count is the manual's own claim about the figure. Capturing fewer rows
// would publish an image contradicting the prose beside it, so the wait is
// bounded by versionAwaitTimeout (and the caller's context) and running out
// fails the island.
func (c *Capturer) awaitVersions(ctx context.Context, spec docs.CaptureSpec) error {
	if spec.AwaitVersions <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, versionAwaitTimeout)
	defer cancel()
	for {
		n, err := c.proj.countVersions(ctx, spec)
		if err != nil {
			return err
		}
		if n >= spec.AwaitVersions {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"waited for %d version rows on %s but the history API reported %d: the "+
					"version sweep had not captured them in time. On the postgres backend "+
					"create/update versions are captured by a debounced sweep — set "+
					"RELA_VERSION_SWEEP_INTERVAL/_IDLE low (e.g. 500ms/200ms) for a docs "+
					"build, as `just docs-visual-postgres` does",
				spec.AwaitVersions, spec.Entity, n)
		case <-time.After(versionPollInterval):
		}
	}
}
