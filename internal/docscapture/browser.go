package docscapture

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// launchTimeout bounds ONE attempt to start Chrome and dial its devtools
// websocket.
//
// Without it a launch inherits the docs build's root context, which is a signal
// context with no deadline, so the only bound is chromedp's own internal dial
// timeout — and newBrowser then retries WITHOUT the sandbox, paying that wait a
// second time before the island fails. CI saw exactly that:
// `could not dial "ws://127.0.0.1:.../devtools/browser": context deadline
// exceeded`. An explicit bound makes the sandboxed attempt give up promptly so
// the fallback still has room to succeed.
const launchTimeout = 25 * time.Second

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
	b, sandboxErr := launch(parent, false)
	if sandboxErr == nil {
		return b, nil
	}
	// Retry without the sandbox (root/container environments).
	b, err := launch(parent, true)
	if err != nil {
		// Report BOTH reasons. The sandboxed attempt is the one that usually
		// explains the failure, and reporting only the fallback's error sent
		// readers looking at --no-sandbox for a problem that was a slow or
		// missing Chrome. errors.Join keeps both inspectable, not just printable.
		return nil, fmt.Errorf("launch chrome: %w", errors.Join(sandboxErr, err))
	}
	return b, nil
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
	//
	// Run on ctx itself, NOT on a child with a timeout. This first Run is what
	// allocates the tab, and chromedp binds the tab's lifetime to the context
	// it is allocated under — so a `context.WithTimeout(ctx, ...)` here cancels
	// the tab the moment this function returns, and every later action fails
	// with "context canceled". The bound is applied with a timer instead: it
	// stops us WAITING past launchTimeout without owning what we waited for.
	done := make(chan error, 1)
	go func() { done <- chromedp.Run(ctx) }()

	timer := time.NewTimer(launchTimeout)
	defer timer.Stop()

	var err error
	select {
	case err = <-done:
	case <-timer.C:
		err = fmt.Errorf("chrome did not start within %s", launchTimeout)
	case <-parent.Done():
		err = parent.Err()
	}
	if err != nil {
		// Canceling here also unblocks the goroutine above if it is still
		// dialing, so the timeout path leaves nothing running.
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
