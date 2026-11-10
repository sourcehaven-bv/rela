package tenant

import (
	"context"
	"sync"
	"time"
)

// RevalidateInterval is how often a process that holds a tenant open checks
// that the tenant is still served. A suspension or erasure takes effect in
// every such process within this time.
const RevalidateInterval = 30 * time.Second

// Watch resolves orgID through r every interval until stop is called, and
// calls onRefused once when r refuses it: the tenant was suspended, is being
// erased or is gone. It is for a process that holds one tenant open for
// long, such as `rela --tenant X scheduler` or stdio MCP, which has no
// [Registry] to revalidate it. A lookup that fails is not a refusal and
// changes nothing. The lookups carry parent's values but not its
// cancellation, so only stop ends the watcher.
//
// stop waits for the watcher to finish, so onRefused never runs after stop
// returns. onRefused must therefore not call stop itself.
//
// Nil: rejected — r and onRefused are required; Watch panics without them,
// as a programming error.
func Watch(parent context.Context, r Resolver, orgID string, interval time.Duration,
	onRefused func(error),
) (stop func()) {
	if r == nil || onRefused == nil {
		panic("tenant.Watch: resolver and onRefused are required")
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	var wg sync.WaitGroup
	wg.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if _, err := r.Resolve(ctx, orgID); err != nil && refused(err) && ctx.Err() == nil {
				onRefused(err)
				return
			}
		}
	})
	return func() {
		cancel()
		wg.Wait()
	}
}

// WatchDeployment runs [Watch] for orgID over a resolver of its own, built
// as [ForDeployment] builds it, so that a process holding orgID open learns
// when it is suspended or erased. It resolves orgID once before it starts,
// and returns that error when orgID is no longer served. stop ends the watcher and closes the
// resolver. Without tenant mode there is nothing to watch, and stop does
// nothing.
func WatchDeployment(ctx context.Context, getenv func(string) string, orgID string,
	interval time.Duration, onRefused func(error),
) (stop func(), err error) {
	r, err := ForDeployment(ctx, getenv)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return func() {}, nil
	}
	// Check once now: the caller has just taken the schema lock, and the
	// status it opened by was read before that.
	if _, err = r.Resolve(ctx, orgID); err != nil {
		_ = r.Close()
		return nil, err
	}
	stopWatch := Watch(ctx, r, orgID, interval, onRefused)
	return func() {
		stopWatch()
		_ = r.Close()
	}, nil
}
