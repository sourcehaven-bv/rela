package docscapture

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// Capturer implements docs.PageInspector as an OPTIONAL capability alongside
// docs.Capturer. Asserted at compile time here rather than left to the
// type-assertion in page{}, because that assertion fails at BUILD time of a
// manual — long after this file compiled — and its failure mode is a verb that
// refuses for a reason no one can act on.
var _ docs.PageInspector = (*Capturer)(nil)

// perInspectTimeout bounds one navigate+render+read-back. Shorter than
// perCaptureTimeout: there is no screenshot encoding or viewport refit, just a
// load and a DOM read.
const perInspectTimeout = 20 * time.Second

// Inspect implements docs.PageInspector: it renders the page spec describes and
// returns the visible text of every element matching selector.
//
// # Why this rides the capture path rather than a second browser path
//
// The expensive, subtle part of a capture is everything BEFORE the screenshot:
// the seeded temp project, the SPA server, the role header, the about:blank
// teardown that stops SPA state carrying between islands, and the renderability
// gate that distinguishes "loaded" from "rendered an empty shell after a failed
// load". An assertion that skipped any of those would be asserting about a
// different page than the figure beside it shows — the exact drift page{}
// exists to prevent. So this is the same sequence, minus the pixels.
func (c *Capturer) Inspect(ctx context.Context, spec docs.CaptureSpec, selector string) ([]string, error) {
	if err := c.ensure(ctx, spec); err != nil {
		return nil, err
	}

	// Bind the deadline to the browser's chromedp context (its tab), not the
	// caller's — chromedp actions must run against the browser context.
	cctx, cancel := context.WithTimeout(c.browser.ctx, perInspectTimeout)
	defer cancel()

	if err := c.proj.requireKnownRole(spec.As); err != nil {
		return nil, err
	}
	url := captureURL(trimSlash(c.proj.server.URL), spec)

	var raw string
	actions := []chromedp.Action{
		// Same full-document teardown a capture does: one tab is reused for the
		// whole build, and without this a navigation is a client-side route
		// transition that can leave a watcher un-re-run and the page stuck
		// pending. See the note in Capture.
		chromedp.Navigate("about:blank"),
		deviceMetricsOverride(),
		network.SetExtraHTTPHeaders(network.Headers(map[string]any{roleHeader: spec.As})),
		chromedp.Navigate(url),
		renderabilityGate(),
		chromedp.Sleep(settleDelay),
		readRegionText(selector, &raw),
	}

	//nolint:contextcheck // chromedp actions bind to the browser context (cctx), not the caller ctx
	if err := chromedp.Run(cctx, actions...); err != nil {
		return nil, fmt.Errorf("inspect: %w", err)
	}

	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("inspect: decoding region text: %w", err)
	}
	return out, nil
}

// readRegionText evaluates the selector in the page and yields the trimmed,
// whitespace-collapsed text of each match as a JSON array.
//
// # Why innerText and not textContent
//
// `textContent` returns the text of hidden nodes too — a collapsed column, a
// `display:none` template, a screen-reader-only string. A manual asserting
// "the reader's board does not show POL-2" means what a reader can SEE, so a
// claim satisfied by invisible markup would be false in the only sense that
// matters. `innerText` is the rendered text, which is the same thing the
// screenshot beside it photographs.
//
// aria-label is folded in because several regions in the vocabulary carry their
// accessible name there rather than as child text — a kanban card's title is
// its `aria-label`, and the card's own innerText is the truncated body. The
// name is what a manual claims about.
func readRegionText(selector string, out *string) chromedp.Action {
	js := `(function(){
  var els = document.querySelectorAll(` + jsString(selector) + `);
  var out = [];
  for (var i = 0; i < els.length; i++) {
    var e = els[i];
    var parts = [];
    var label = e.getAttribute('aria-label');
    if (label) parts.push(label);
    var t = e.innerText || '';
    if (t) parts.push(t);
    var s = parts.join(' ').replace(/\s+/g, ' ').trim();
    out.push(s);
  }
  return JSON.stringify(out);
})()`
	return chromedp.Evaluate(js, out)
}

// jsString renders s as a JavaScript string literal. The selectors it quotes
// come from the docs package's closed region table, never from a manual, but
// quoting properly keeps that a property of the code rather than of the current
// contents of the table.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil { // coverage-ignore: json.Marshal of a string cannot fail
		return `""`
	}
	return string(b)
}
