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
	anchor := "#field-" + firstFieldAnchor(spec)

	var png []byte
	actions := []chromedp.Action{
		// Deterministic viewport so form layout is stable across machines.
		deviceMetricsOverride(),
		// Thread the requested role to the per-request principal resolver.
		network.SetExtraHTTPHeaders(network.Headers(map[string]any{roleHeader: spec.As})),
		chromedp.Navigate(url),
		// Renderability gate: the entity must actually render (a real field
		// appears) AND the SPA's "failed to load" boundary must be absent (DR-S4).
		chromedp.WaitVisible(anchor, chromedp.ByQuery),
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
		p, err := standUp(ctx, spec.ProjectDir, spec.Seed)
		if err != nil {
			return err
		}
		c.proj = p
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

// captureAction returns the element-clip or bounded full-page screenshot action.
// Both produce PNG (lossless) — chromedp.Screenshot and page.CaptureScreenshot
// default to PNG; only FullScreenshot(_, quality) forces JPEG, which we avoid.
func captureAction(spec docs.CaptureSpec, out *[]byte) chromedp.Action {
	if spec.Clip != "" {
		return chromedp.Screenshot(spec.Clip, out, chromedp.ByQuery, chromedp.NodeVisible)
	}
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// Measure the full content; bound the height (DR-M2) — fail loud if too tall.
		var dims struct{ W, H int64 }
		if err := chromedp.Evaluate(
			`({W: Math.ceil(document.documentElement.scrollWidth),
			   H: Math.ceil(Math.max(document.body.scrollHeight, document.documentElement.scrollHeight))})`,
			&dims,
		).Do(ctx); err != nil {
			return err
		}
		if dims.H > maxFullHeight {
			return fmt.Errorf("page is %dpx tall (> %dpx cap); use a clip= selector to bound the capture", dims.H, maxFullHeight)
		}
		buf, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithClip(&page.Viewport{X: 0, Y: 0, Width: float64(dims.W), Height: float64(dims.H), Scale: 1}).
			Do(ctx)
		if err != nil {
			return err
		}
		*out = buf
		return nil
	})
}

// firstFieldAnchor picks a field to gate renderability on: the first arrow's
// field target, else a conventional "title"/"name". The chosen anchor must be a
// field the target form actually renders.
func firstFieldAnchor(spec docs.CaptureSpec) string {
	for _, a := range spec.Arrows {
		if f := fieldOf(a.At); f != "" {
			return f
		}
	}
	return "title"
}

// renderabilityGate fails the capture if the SPA surfaced an error toast instead
// of rendering the entity (DR-S4) — e.g. a bad entity id, a form field set the
// entity doesn't satisfy, or the `as` role lacking read access. The error toast
// carries a stable data-testid; its presence means we'd be capturing a broken
// form. (The prior WaitVisible on a real #field-<prop> already confirms the form
// mounted; this catches the load-error-after-mount case.)
func renderabilityGate() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var broken bool
		if err := chromedp.Evaluate(
			`!!document.querySelector('[data-testid="toast-error"]')`, &broken,
		).Do(ctx); err != nil {
			return err
		}
		if broken {
			return errors.New("the entity failed to render (the SPA surfaced an error) — check the entity id, the form's field set, and the `as` role's read access")
		}
		return nil
	})
}

// deviceMetrics gives the capture a stable, wide-enough viewport so form layout
// is deterministic across machines.
func deviceMetricsOverride() chromedp.Action {
	return emulation.SetDeviceMetricsOverride(viewportW, viewportH, 1.0, false)
}
