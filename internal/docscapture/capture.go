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
	viewportH = 1600
)

// Capturer implements docs.Capturer using chromedp against a data-entry SPA
// served over a seeded temp project. The temp project + server + browser are
// created lazily on the first Capture and reused across islands; Close tears
// them all down.
type Capturer struct {
	proj    *project
	browser *browser
}

// New returns a Capturer. It does NOT launch a browser yet (that happens on the
// first Capture) so a manual with no screenshot{} pays nothing. It DOES verify a
// Chrome binary is resolvable, so a screenshot-bearing manual fails loud early
// rather than after standing up a server.
func New() (*Capturer, error) {
	if _, ok := hasChrome(); !ok {
		return nil, errors.New("no Chrome/Chromium browser found on PATH — screenshot{} requires a browser")
	}
	return &Capturer{}, nil
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

	url := formURL(trimSlash(c.proj.server.URL), spec)

	var png []byte
	actions := []chromedp.Action{
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

// ensure lazily stands up the temp-project server (once) and the browser (once).
func (c *Capturer) ensure(ctx context.Context, spec docs.CaptureSpec) error {
	if c.proj == nil {
		p, err := standUp(ctx, spec.ProjectDir, spec.Seed, true)
		if err != nil {
			return err
		}
		c.proj = p
	} else if err := c.proj.syncSeed(ctx, spec.Seed); err != nil {
		// A later island may have create()d entities after standUp; apply the new
		// tail so this island's entity actually exists in the running server.
		return err
	}
	if c.browser == nil {
		b, err := newBrowser(ctx)
		if err != nil {
			return err
		}
		c.browser = b
	}
	return nil
}

// Close tears down the browser, server, and temp project.
func (c *Capturer) Close() error {
	if c.browser != nil {
		c.browser.close()
	}
	if c.proj != nil {
		c.proj.close()
	}
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
		if err := chromedp.Poll(
			`(function(){var e=document.querySelector('[data-testid^="form-state-"]');`+
				`if(!e)return null;var s=e.getAttribute('data-testid').slice(11);`+
				`return s==='pending'?null:s;})()`,
			&state,
			chromedp.WithPollingInterval(pollInterval),
		).Do(ctx); err != nil {
			return fmt.Errorf("waiting for the form to load: %w", err)
		}
		if state == "error" {
			return errors.New("the entity failed to load in the form — check the entity id, the form's field set, and the `as` role's read access")
		}
		return nil
	})
}

// deviceMetrics gives the capture a stable, wide-enough viewport so form layout
// is deterministic across machines.
func deviceMetricsOverride() chromedp.Action {
	return emulation.SetDeviceMetricsOverride(viewportW, viewportH, 1.0, false)
}
