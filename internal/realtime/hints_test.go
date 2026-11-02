package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

// hintBus connects the Locals of one test as a feed connects processes:
// every payload a Local sends reaches every follower, its own included.
type hintBus struct {
	mu        sync.Mutex
	sent      [][]byte
	followers map[int]func([]byte)
	gaps      map[int]func()
	next      int
}

func newHintBus() *hintBus {
	return &hintBus{followers: map[int]func([]byte){}, gaps: map[int]func(){}}
}

func (b *hintBus) SendHint(_ context.Context, p []byte) error {
	b.mu.Lock()
	b.sent = append(b.sent, bytes.Clone(p))
	fs := make([]func([]byte), 0, len(b.followers))
	for _, f := range b.followers {
		fs = append(fs, f)
	}
	b.mu.Unlock()
	for _, f := range fs {
		f(p)
	}
	return nil
}

func (b *hintBus) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	b.mu.Lock()
	id := b.next
	b.next++
	b.followers[id] = deliver
	b.gaps[id] = gap
	b.mu.Unlock()
	<-ctx.Done()
	b.mu.Lock()
	delete(b.followers, id)
	delete(b.gaps, id)
	b.mu.Unlock()
	return nil
}

func (b *hintBus) payloads() []hintPayload {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]hintPayload, 0, len(b.sent))
	for _, p := range b.sent {
		var h hintPayload
		if err := json.Unmarshal(p, &h); err != nil {
			panic(err)
		}
		out = append(out, h)
	}
	return out
}

func (b *hintBus) reset() {
	b.mu.Lock()
	b.sent = nil
	b.mu.Unlock()
}

func (b *hintBus) followerCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.followers)
}

func (b *hintBus) gap() {
	b.mu.Lock()
	gs := make([]func(), 0, len(b.gaps))
	for _, g := range b.gaps {
		gs = append(gs, g)
	}
	b.mu.Unlock()
	for _, g := range gs {
		g()
	}
}

func hintLocal(t *testing.T, sender HintSender) *Local {
	t.Helper()
	l, err := NewLocalWithHints(time.Now, sender)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close(context.Background()) })
	return l
}

// follow starts l's follower on src and waits until it has registered and
// made its start resync, which it takes from sub.
func follow(t *testing.T, l *Local, bus *hintBus, sub *Subscription) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	before := bus.followerCount()
	go func() { done <- l.FollowHints(ctx, bus) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("FollowHints: %v", err)
		}
	})
	eventually(t, func() bool { return bus.followerCount() > before && l.in.resyncs.Load() > 0 })
	if c := sub.TakeLive(); !c.Resync {
		t.Fatalf("start: want a resync, got %+v", c)
	}
}

func eventually(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// takeUntil takes sub's changes until one is not empty.
func takeUntil(t *testing.T, sub *Subscription) LiveChanges {
	t.Helper()
	var c LiveChanges
	eventually(t, func() bool { c = sub.TakeLive(); return !c.Empty() })
	return c
}

func flush(t *testing.T, l *Local) {
	t.Helper()
	if err := l.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHints_KindsReachOtherLocal(t *testing.T) {
	bus := newHintBus()
	a, b := hintLocal(t, bus), hintLocal(t, bus)
	sub := b.SubscribeLive()
	defer sub.Close()
	follow(t, b, bus, sub)
	k1, k2 := keyOf("TKT-1"), keyOf("TKT-2")
	alice := channels.Reader{User: "alice", OrgID: "acme"}

	t.Run("message", func(t *testing.T) {
		a.markMessage(k1)
		c := takeUntil(t, sub)
		if !hasKey(c.Keys, k1) || !hasKey(c.Durable, k1) || len(c.Moved) != 0 {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("moved", func(t *testing.T) {
		a.Mark(k2)
		c := takeUntil(t, sub)
		if !hasKey(c.Keys, k2) || !hasKey(c.Durable, k2) || !hasKey(c.Moved, k2) {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("addressed", func(t *testing.T) {
		a.markAddressed([]string{"integration:x"}, k1, markKindMessage, true)
		c := takeUntil(t, sub)
		if len(c.Addressed) != 1 || c.Addressed[0] != "integration:x" || !hasKey(c.Durable, k1) {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("read", func(t *testing.T) {
		a.markFor(alice, true, k2)
		c := takeUntil(t, sub)
		if len(c.Keys) != 0 || len(c.Durable) != 0 || len(c.Moved) != 0 {
			t.Fatalf("a read mark changed channels: %+v", c)
		}
		if got := c.Reads[alice]; len(got) != 1 || got[0] != k2 {
			t.Fatalf("reads = %+v", c.Reads)
		}
	})
}

func hasKey(keys []channels.Key, k channels.Key) bool {
	return slices.Contains(keys, k)
}

func TestHints_OwnOriginDroppedAndNoEcho(t *testing.T) {
	bus := newHintBus()
	a, b := hintLocal(t, bus), hintLocal(t, bus)
	subA, subB := a.SubscribeLive(), b.SubscribeLive()
	defer subA.Close()
	defer subB.Close()
	follow(t, a, bus, subA)
	follow(t, b, bus, subB)
	bus.reset()

	a.markMessage(keyOf("TKT-1"))
	if c := subA.TakeLive(); !hasKey(c.Keys, keyOf("TKT-1")) {
		t.Fatalf("local mark: %+v", c)
	}
	flush(t, a)
	takeUntil(t, subB)
	// b applied the hint; flushing b proves it queued nothing.
	flush(t, b)
	time.Sleep(10 * time.Millisecond)
	if c := subA.TakeLive(); !c.Empty() {
		t.Fatalf("a acted on its own hint: %+v", c)
	}
	ps := bus.payloads()
	if len(ps) != 1 || ps[0].Origin != a.origin {
		t.Fatalf("payloads = %+v; want one from a", ps)
	}
}

func TestHints_EphemeralSendsNothing(t *testing.T) {
	bus := newHintBus()
	l := hintLocal(t, bus)
	k := keyOf("TKT-1")
	l.markEphemeral(k)
	if _, _, err := l.typing.touch(k, "alice", channels.Author{User: "alice"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ephemeral.add(k, signal{id: "1", typ: "s", identity: "alice", at: time.Now(),
		expires: time.Now().Add(time.Minute)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := l.EntityRenamed(context.Background(), "TKT-1", "TKT-2"); err != nil {
		t.Fatal(err)
	}
	if err := l.EntityDeleted(context.Background(), "TKT-2"); err != nil {
		t.Fatal(err)
	}
	l.expire()
	flush(t, l)
	if ps := bus.payloads(); len(ps) != 0 {
		t.Fatalf("payloads = %+v", ps)
	}
}

func TestHints_OneSendPerPost(t *testing.T) {
	bus := newHintBus()
	l := hintLocal(t, bus)
	k := keyOf("TKT-1")
	// What notePosted marks: the addressed post and the poster's marker.
	l.markAddressed([]string{"bob"}, k, markKindMoved, true)
	l.markFor(channels.Reader{User: "alice"}, true, k)
	flush(t, l)
	ps := bus.payloads()
	if len(ps) != 1 || len(ps[0].Hints) != 2 {
		t.Fatalf("payloads = %+v", ps)
	}
	if h := ps[0].Hints[0]; h.Kind != hintAddressed || h.Of != hintMoved {
		t.Fatalf("first hint = %+v", h)
	}
}

func TestHints_CapsTurnIntoResync(t *testing.T) {
	cases := []struct {
		name string
		mark func(l *Local)
	}{
		{"keys", func(l *Local) {
			keys := make([]channels.Key, maxHintKeys+1)
			for i := range keys {
				keys[i] = keyOf(fmt.Sprint("TKT-", i))
			}
			l.Mark(keys...)
		}},
		{"targets", func(l *Local) {
			ts := make([]string, maxHintTargets+1)
			for i := range ts {
				ts[i] = fmt.Sprint("user-", i)
			}
			l.markAddressed(ts, keyOf("TKT-1"), markKindMessage, true)
		}},
		{"bytes", func(l *Local) {
			ts := make([]string, maxHintTargets)
			for i := range ts {
				ts[i] = strings.Repeat("x", 200) + strconv.Itoa(i)
			}
			l.markAddressed(ts, keyOf("TKT-1"), markKindMessage, true)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := newHintBus()
			l := hintLocal(t, bus)
			tc.mark(l)
			flush(t, l)
			ps := bus.payloads()
			if len(ps) != 1 || len(ps[0].Hints) != 1 || ps[0].Hints[0].Kind != hintResync {
				t.Fatalf("payloads = %+v", ps)
			}
		})
	}
}

func TestHints_SplitAcrossPayloads(t *testing.T) {
	bus := newHintBus()
	l := hintLocal(t, bus)
	// Many readers, each a hint of its own: more than one payload holds.
	for i := range 80 {
		l.markFor(channels.Reader{User: fmt.Sprint(strings.Repeat("u", 60), i)}, true, keyOf("TKT-1"))
	}
	flush(t, l)
	ps := bus.payloads()
	if len(ps) < 2 {
		t.Fatalf("want several payloads, got %d", len(ps))
	}
	n := 0
	for _, p := range ps {
		n += len(p.Hints)
	}
	if n != 80 {
		t.Fatalf("hints = %d, want 80", n)
	}
}

// blockingSender blocks every send until release is closed.
type blockingSender struct {
	release chan struct{}
	entered chan struct{}
	once    sync.Once
	inner   *hintBus
}

func (s *blockingSender) SendHint(ctx context.Context, p []byte) error {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.inner.SendHint(ctx, p)
}

func TestHints_MarkNeverWaitsAndOverflowResyncs(t *testing.T) {
	bus := newHintBus()
	s := &blockingSender{release: make(chan struct{}), entered: make(chan struct{}), inner: bus}
	l := hintLocal(t, s)
	l.markMessage(keyOf("TKT-first"))
	<-s.entered
	start := time.Now()
	for i := range hintQueueSize * 3 {
		l.markMessage(keyOf(fmt.Sprint("TKT-", i%50)))
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("marks waited %s on a blocked sender", d)
	}
	close(s.release)
	flush(t, l)
	var resyncs int
	for _, p := range bus.payloads() {
		for _, h := range p.Hints {
			if h.Kind == hintResync {
				resyncs++
			}
		}
	}
	if resyncs != 1 {
		t.Fatalf("resync hints = %d, want 1", resyncs)
	}
}

func TestHints_FlushAndClose(t *testing.T) {
	bus := newHintBus()
	l, err := NewLocalWithHints(time.Now, bus)
	if err != nil {
		t.Fatal(err)
	}
	l.markMessage(keyOf("TKT-1"))
	if cErr := l.Close(context.Background()); cErr != nil {
		t.Fatal(cErr)
	}
	if ps := bus.payloads(); len(ps) != 1 {
		t.Fatalf("payloads = %d, want 1", len(ps))
	}
	l.markMessage(keyOf("TKT-2"))
	if cErr := l.Close(context.Background()); cErr != nil {
		t.Fatalf("second Close: %v", cErr)
	}
	if fErr := l.Flush(context.Background()); !errors.Is(fErr, errHintsClosed) {
		t.Fatalf("Flush after Close = %v", fErr)
	}
	if ps := bus.payloads(); len(ps) != 1 {
		t.Fatalf("a mark after Close was sent")
	}
	plain, err := NewLocal(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := plain.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := plain.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalWithHints(time.Now, nil); err == nil {
		t.Fatal("nil sender accepted")
	}
}

type failingSender struct{}

func (failingSender) SendHint(context.Context, []byte) error { return errors.New("feed down") }

// recordHandler counts the warnings logged.
type recordHandler struct {
	mu   sync.Mutex
	msgs []string
}

func (h *recordHandler) Enabled(_ context.Context, lv slog.Level) bool { return lv >= slog.LevelWarn }
func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.msgs = append(h.msgs, r.Message)
	h.mu.Unlock()
	return nil
}
func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(string) slog.Handler      { return h }

func TestHints_SendErrorsLoggedOncePerMinute(t *testing.T) {
	h := &recordHandler{}
	logtest.SetDefault(t, h)
	l := hintLocal(t, failingSender{})
	for i := range 3 {
		l.markMessage(keyOf(fmt.Sprint("TKT-", i)))
		flush(t, l)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.msgs) != 1 {
		t.Fatalf("warnings = %q, want one", h.msgs)
	}
}

func TestHints_ResyncLimit(t *testing.T) {
	bus := newHintBus()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
	l, err := NewLocalWithHints(clock, bus)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close(context.Background()) })
	fire := make(chan time.Time)
	var waits []time.Duration
	var waitMu sync.Mutex
	l.in.after = func(d time.Duration) <-chan time.Time {
		waitMu.Lock()
		waits = append(waits, d)
		waitMu.Unlock()
		return fire
	}
	sub := l.Subscribe()
	defer sub.Close()
	follow(t, l, bus, sub) // the start resync: 1
	other, _ := json.Marshal(hintPayload{Origin: "other", Hints: []wireHint{{Kind: hintResync}}})
	for range 5 {
		bus.gap()
		_ = bus.SendHint(context.Background(), other)
		_ = bus.SendHint(context.Background(), []byte("{not json"))
	}
	eventually(t, func() bool { waitMu.Lock(); defer waitMu.Unlock(); return len(waits) == 1 })
	time.Sleep(20 * time.Millisecond)
	if n := l.in.resyncs.Load(); n != 1 {
		t.Fatalf("resyncs inside the window = %d, want 1", n)
	}
	clockMu.Lock()
	now = now.Add(hintResyncGap)
	clockMu.Unlock()
	fire <- now
	eventually(t, func() bool { return l.in.resyncs.Load() == 2 })
	if _, resync := sub.Take(); !resync {
		t.Fatal("deferred resync did not reach the subscriber")
	}
	waitMu.Lock()
	defer waitMu.Unlock()
	if len(waits) != 1 || waits[0] != hintResyncGap {
		t.Fatalf("deferred waits = %v", waits)
	}
}

func TestHints_FollowSingleFlight(t *testing.T) {
	bus := newHintBus()
	l := hintLocal(t, bus)
	sub := l.Subscribe()
	defer sub.Close()
	follow(t, l, bus, sub)
	done := make(chan error, 1)
	go func() { done <- l.FollowHints(context.Background(), bus) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("a second FollowHints did not return")
	}
	if n := bus.followerCount(); n != 1 {
		t.Fatalf("followers = %d", n)
	}
}

type brokenSource struct{}

func (brokenSource) FollowHints(context.Context, func([]byte), func()) error {
	return errors.New("no feed")
}

func TestHints_FollowReturnsSourceError(t *testing.T) {
	l := hintLocal(t, newHintBus())
	if err := l.FollowHints(context.Background(), brokenSource{}); err == nil {
		t.Fatal("want the source error")
	}
	// The guard is released: a later call runs again.
	if err := l.FollowHints(context.Background(), brokenSource{}); err == nil {
		t.Fatal("want the source error again")
	}
}

func TestDecodeHints(t *testing.T) {
	good := func(h wireHint) []byte {
		b, _ := json.Marshal(hintPayload{Origin: "o", Hints: []wireHint{h}})
		return b
	}
	k := keyOf("TKT-1")
	cases := []struct {
		name    string
		payload []byte
		ok      bool
	}{
		{"message", good(wireHint{Kind: hintMessage, Keys: []channels.Key{k}}), true},
		{"resync", good(wireHint{Kind: hintResync}), true},
		{"addressed", good(wireHint{Kind: hintAddressed, Keys: []channels.Key{k}, Of: hintMoved, Targets: []string{"x"}}), true},
		{"read", good(wireHint{Kind: hintRead, Keys: []channels.Key{k}, User: "alice"}), true},
		{"malformed", []byte("{"), false},
		{"no origin", []byte(`{"h":[]}`), false},
		{"unknown kind", good(wireHint{Kind: "z"}), false},
		{"invalid key", good(wireHint{Kind: hintMessage, Keys: []channels.Key{{Type: "x"}}}), false},
		{"no keys", good(wireHint{Kind: hintMoved}), false},
		{"addressed kind", good(wireHint{Kind: hintAddressed, Keys: []channels.Key{k}, Of: "z", Targets: []string{"x"}}), false},
		{"addressed no targets", good(wireHint{Kind: hintAddressed, Keys: []channels.Key{k}, Of: hintMessage}), false},
		{"addressed bad target", good(wireHint{Kind: hintAddressed, Keys: []channels.Key{k}, Of: hintMessage,
			Targets: []string{"a\x00"}}), false},
		{"addressed two keys", good(wireHint{Kind: hintAddressed, Keys: []channels.Key{k, k}, Of: hintMessage,
			Targets: []string{"x"}}), false},
		{"read no user", good(wireHint{Kind: hintRead, Keys: []channels.Key{k}}), false},
		{"oversized", append([]byte(`{"o":"o","h":[]`), bytes.Repeat([]byte(" "), maxHintBytes)...), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeHints(tc.payload)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok %v", err, tc.ok)
			}
		})
	}
}

func TestHints_MarkResyncReachesEverySubscriber(t *testing.T) {
	l, err := NewLocal(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	live, plain := l.SubscribeLive(), l.Subscribe()
	defer live.Close()
	defer plain.Close()
	l.markFor(channels.Reader{User: "a"}, false, keyOf("TKT-1"))
	l.markResync()
	if c := live.TakeLive(); !c.Resync || len(c.Reads) != 0 {
		t.Fatalf("live = %+v", c)
	}
	if _, resync := plain.Take(); !resync {
		t.Fatal("plain subscriber not resynced")
	}
}
