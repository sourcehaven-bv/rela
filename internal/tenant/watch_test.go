package tenant_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestWatch pins that a long-running tenant process learns of a suspension
// (RR-7EKSK1I), and that a failed lookup is not taken for one.
func TestWatch(t *testing.T) {
	t.Parallel()

	fail := &failingResolver{inner: mapResolver(t, "a")}
	fail.set(errors.New("connection refused"))
	counted := &countingResolver{inner: fail}
	refusedCh := make(chan error, 1)
	stop := tenant.Watch(t.Context(), counted, "a", time.Millisecond, func(err error) { refusedCh <- err })
	defer stop()

	// Several failed lookups run before the refusal is set.
	for counted.n.Load() < 3 {
		select {
		case err := <-refusedCh:
			t.Fatalf("a failed lookup was taken for a refusal: %v", err)
		case <-time.After(time.Millisecond):
		}
	}

	fail.set(fmt.Errorf("suspended: %w", tenant.ErrUnknownTenant))
	select {
	case err := <-refusedCh:
		if !errors.Is(err, tenant.ErrUnknownTenant) {
			t.Fatalf("onRefused(%v), want ErrUnknownTenant", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher never reported the suspension")
	}
}

type countingResolver struct {
	inner tenant.Resolver
	n     atomic.Int32
}

func (c *countingResolver) Resolve(ctx context.Context, org string) (tenant.Tenant, error) {
	c.n.Add(1)
	return c.inner.Resolve(ctx, org)
}

// TestWatch_StopEndsTheWatcher pins that stop returns and nothing is
// reported after it.
func TestWatch_StopEndsTheWatcher(t *testing.T) {
	t.Parallel()

	called := make(chan struct{}, 1)
	stop := tenant.Watch(t.Context(), mapResolver(t, "a"), "a", time.Millisecond, func(error) { called <- struct{}{} })
	time.Sleep(5 * time.Millisecond)
	stop()
	select {
	case <-called:
		t.Fatal("onRefused ran for a served tenant")
	default:
	}
}
