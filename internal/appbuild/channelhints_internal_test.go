package appbuild

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeEndpoint records sends and runs followers until their ctx ends.
type fakeEndpoint struct {
	mu        sync.Mutex
	sent      [][]byte
	following atomic.Int32
	followErr error
}

func (f *fakeEndpoint) SendHint(_ context.Context, p []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, p)
	return nil
}

func (f *fakeEndpoint) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	if f.followErr != nil {
		return f.followErr
	}
	f.following.Add(1)
	defer f.following.Add(-1)
	gap() // positioned
	deliver([]byte("hello"))
	<-ctx.Done()
	return nil
}

func (f *fakeEndpoint) sends() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func TestHintRelay_AttachDetachAndSwap(t *testing.T) {
	ctx := context.Background()
	r := newHintRelay()
	require.ErrorIs(t, r.SendHint(ctx, []byte("x")), errNoHintEndpoint)

	one, two := &fakeEndpoint{}, &fakeEndpoint{}
	a1 := r.attach(one)
	require.NoError(t, r.SendHint(ctx, []byte("to one")))

	var mu sync.Mutex
	var delivered int
	var gaps atomic.Int32
	fctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- r.FollowHints(fctx, func([]byte) {
			mu.Lock()
			delivered++
			mu.Unlock()
		}, func() { gaps.Add(1) })
	}()
	require.Eventually(t, func() bool { return one.following.Load() == 1 && gaps.Load() == 1 },
		5*time.Second, time.Millisecond, "the endpoint reports its start as a gap")

	a2 := r.attach(two)
	require.Eventually(t, func() bool { return two.following.Load() == 1 && one.following.Load() == 0 },
		5*time.Second, time.Millisecond, "the follower moves to the new endpoint")
	require.Eventually(t, func() bool { return gaps.Load() == 2 }, 5*time.Second, time.Millisecond)
	require.NoError(t, r.SendHint(ctx, []byte("to two")))
	require.Equal(t, 1, two.sends())

	// Detaching the newest, as a rejected reload does, falls back to the
	// one before.
	r.detach(a2)
	require.NoError(t, r.SendHint(ctx, []byte("back to one")))
	require.Equal(t, 2, one.sends())
	require.Eventually(t, func() bool { return one.following.Load() == 1 && two.following.Load() == 0 },
		5*time.Second, time.Millisecond, "the follower moves back")

	// Detaching one that is not current changes nothing.
	a3 := r.attach(two)
	require.Eventually(t, func() bool { return two.following.Load() == 1 }, 5*time.Second, time.Millisecond)
	r.detach(a1)
	r.detach(nil)
	r.detach(a1) // twice: no effect
	require.NoError(t, r.SendHint(ctx, []byte("still two")))
	require.Equal(t, 2, two.sends())
	require.Equal(t, int32(1), two.following.Load())

	r.detach(a3)
	require.ErrorIs(t, r.SendHint(ctx, []byte("x")), errNoHintEndpoint)
	require.Eventually(t, func() bool { return two.following.Load() == 0 }, 5*time.Second, time.Millisecond)

	// An endpoint that cannot follow is logged; the relay waits for the next.
	r.attach(&fakeEndpoint{followErr: errors.New("no listener")})
	three := &fakeEndpoint{}
	r.attach(three)
	require.Eventually(t, func() bool { return three.following.Load() == 1 }, 5*time.Second, time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, delivered, 3)
}

func TestFollowChannels_NoHints(t *testing.T) {
	done := FollowChannels(context.Background(), &Services{})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a Services without hints must return a closed channel")
	}
	retireChannelHints(&Services{}) // no Local: nothing to flush
}
