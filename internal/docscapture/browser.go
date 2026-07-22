package docscapture

import (
	"context"

	"github.com/chromedp/chromedp"
)

// browser owns the chromedp allocator + context lifecycle. The chromedp context
// IS the browser handle (a tab), so it is held on the struct by design.
type browser struct {
	allocCancel context.CancelFunc
	ctxCancel   context.CancelFunc
	ctx         context.Context //nolint:containedctx // the chromedp context is the browser handle
}

// newBrowser launches headless Chrome. It tries WITH the sandbox first and only
// falls back to --no-sandbox if the sandboxed launch fails (DR-M1) — the page
// content is our own localhost fixture, but default-secure is preferred.
func newBrowser(parent context.Context) (*browser, error) {
	b, err := launch(parent, false)
	if err == nil {
		return b, nil
	}
	// Retry without the sandbox (root/container environments).
	return launch(parent, true)
}

func launch(parent context.Context, noSandbox bool) (*browser, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
	)
	if path, ok := hasChrome(); ok {
		opts = append(opts, chromedp.ExecPath(path))
	}
	if noSandbox {
		opts = append(opts, chromedp.Flag("no-sandbox", true))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(parent, opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	// Force the browser to actually start so a launch failure surfaces here
	// (and we can fall back to --no-sandbox) rather than on the first navigate.
	if err := chromedp.Run(ctx); err != nil {
		ctxCancel()
		allocCancel()
		return nil, err
	}
	return &browser{allocCancel: allocCancel, ctxCancel: ctxCancel, ctx: ctx}, nil
}

func (b *browser) close() {
	if b == nil {
		return
	}
	if b.ctxCancel != nil {
		b.ctxCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}
