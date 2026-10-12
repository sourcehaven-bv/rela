package changefeed

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fakeSource is a store.Watcher whose events the test sends by hand.
type fakeSource struct {
	ch       chan store.Event
	canceled chan struct{}
	once     sync.Once
}

func newFakeSource() *fakeSource {
	return &fakeSource{ch: make(chan store.Event, 64), canceled: make(chan struct{})}
}

func (f *fakeSource) Subscribe(int) (events <-chan store.Event, cancel func()) {
	return f.ch, func() { f.once.Do(func() { close(f.canceled) }) }
}

func entity(typ string) store.Event {
	return store.Event{Op: store.EventEntityUpdated, EntityType: typ, EntityID: typ + "-1"}
}

func relation() store.Event {
	return store.Event{Op: store.EventRelationCreated, RelationType: "member-of", From: "a", To: "b"}
}

// started returns a running Bus fed by a fake source, closed at cleanup.
func started(t *testing.T) (*Bus, *fakeSource) {
	t.Helper()
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	src := newFakeSource()
	if err := b.Start(src); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b, src
}

// send publishes events and waits until the bus has counted them.
func send(t *testing.T, b *Bus, src *fakeSource, evs ...store.Event) {
	t.Helper()
	b.mu.Lock()
	want := b.head
	b.mu.Unlock()
	for _, ev := range evs {
		src.ch <- ev
		switch ev.Op {
		case store.EventEntityCreated, store.EventEntityUpdated, store.EventEntityDeleted,
			store.EventRelationCreated, store.EventRelationUpdated, store.EventRelationDeleted:
			want++
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.mu.Lock()
		head := b.head
		b.mu.Unlock()
		if head == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("bus head = %d, want %d", head, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func woke(s *Subscription) bool {
	select {
	case <-s.Wake():
		return true
	default:
		return false
	}
}

func TestBus_SequenceAdvancesPerCountedEvent(t *testing.T) {
	b, src := started(t)
	send(t, b, src, entity("ticket"), relation(), store.Event{Op: store.EventOp(99), EntityType: "ignored"})
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.head != 2 {
		t.Fatalf("head = %d, want 2 (unknown ops are not counted)", b.head)
	}
	if b.typeSeq["ticket"] != 1 || b.relSeq != 2 {
		t.Fatalf("typeSeq = %v, relSeq = %d", b.typeSeq, b.relSeq)
	}
	if _, ok := b.typeSeq["ignored"]; ok {
		t.Fatal("an unknown op recorded a type")
	}
}

func TestBus_SubscribeAndTake(t *testing.T) {
	b, src := started(t)
	send(t, b, src, entity("ticket"), entity("bug"))
	mid := b.Subscribe("")
	defer mid.Close()
	midToken := mid.Token(mid.Start())
	send(t, b, src, entity("feature"), entity("ticket"), relation())

	other, _ := New()
	foreign := other.Subscribe("")
	defer foreign.Close()

	tests := []struct {
		name       string
		token      string
		wantResync bool
		wantWake   bool
		wantTypes  []string
		wantRel    bool
	}{
		{name: "empty token starts at head", token: ""},
		{
			name: "older token yields the types changed since", token: midToken,
			wantWake: true, wantTypes: []string{"feature", "ticket"}, wantRel: true,
		},
		{name: "token at zero yields every type", token: mid.Token(0), wantWake: true,
			wantTypes: []string{"bug", "feature", "ticket"}, wantRel: true},
		{name: "token at head yields nothing", token: mid.Token(5)},
		{name: "token past head resyncs", token: mid.Token(6), wantResync: true},
		{name: "foreign token resyncs", token: foreign.Token(1), wantResync: true},
		{name: "garbage resyncs", token: "not-a-token", wantResync: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := b.Subscribe(tc.token)
			defer s.Close()
			if s.Resync() != tc.wantResync {
				t.Fatalf("Resync = %v, want %v", s.Resync(), tc.wantResync)
			}
			if woke(s) != tc.wantWake {
				t.Fatalf("pre-armed wake = %v, want %v", !tc.wantWake, tc.wantWake)
			}
			got := s.Take()
			if !slices.Equal(got.Types, tc.wantTypes) || got.Relation != tc.wantRel {
				t.Fatalf("Take = %+v, want types %v relation %v", got, tc.wantTypes, tc.wantRel)
			}
			if got.Seq != 5 {
				t.Fatalf("Take.Seq = %d, want head 5", got.Seq)
			}
			if again := s.Take(); len(again.Types) != 0 || again.Relation {
				t.Fatalf("second Take = %+v, want empty", again)
			}
		})
	}
}

func TestBus_StartIsResumedPositionNotHead(t *testing.T) {
	b, src := started(t)
	send(t, b, src, entity("ticket"))
	s := b.Subscribe("")
	tok := s.Token(s.Start())
	s.Close()
	send(t, b, src, entity("bug"))

	r := b.Subscribe(tok)
	defer r.Close()
	if r.Start() != 1 {
		t.Fatalf("Start = %d, want the presented position 1", r.Start())
	}
	// Re-subscribing with a token for Start replays the same changes, so a
	// client that drops before its first flush loses nothing.
	again := b.Subscribe(r.Token(r.Start()))
	defer again.Close()
	if got := again.Take(); !slices.Equal(got.Types, []string{"bug"}) {
		t.Fatalf("replay from Start = %v, want [bug]", got.Types)
	}
}

func TestBus_WakeCoalesces(t *testing.T) {
	b, src := started(t)
	s := b.Subscribe("")
	defer s.Close()
	send(t, b, src, entity("ticket"), entity("ticket"), entity("ticket"))
	if !woke(s) {
		t.Fatal("no wake after writes")
	}
	if woke(s) {
		t.Fatal("wake did not coalesce")
	}
	if got := s.Take(); !slices.Equal(got.Types, []string{"ticket"}) {
		t.Fatalf("Take = %v, want [ticket] once", got.Types)
	}
}

func TestBus_ClosedSubscriptionIsNotWoken(t *testing.T) {
	b, src := started(t)
	s := b.Subscribe("")
	s.Close()
	send(t, b, src, entity("ticket"))
	if woke(s) {
		t.Fatal("closed subscription was woken")
	}
}

func TestBus_Lifecycle(t *testing.T) {
	t.Run("start rejects nil source", func(t *testing.T) {
		b, _ := New()
		if err := b.Start(nil); err == nil {
			t.Fatal("Start(nil) succeeded")
		}
	})
	t.Run("start twice fails", func(t *testing.T) {
		b, _ := started(t)
		if err := b.Start(newFakeSource()); err == nil {
			t.Fatal("second Start succeeded")
		}
	})
	t.Run("start after close fails", func(t *testing.T) {
		b, _ := New()
		b.Close()
		if err := b.Start(newFakeSource()); !errors.Is(err, ErrClosed) {
			t.Fatalf("Start after Close = %v, want ErrClosed", err)
		}
	})
	t.Run("close is idempotent and safe before start", func(_ *testing.T) {
		b, _ := New()
		b.Close()
		b.Close()
	})
	t.Run("close cancels the source and keeps subscriptions usable", func(t *testing.T) {
		b, src := started(t)
		s := b.Subscribe("")
		defer s.Close()
		send(t, b, src, entity("ticket"))
		b.Close()
		b.Close()
		select {
		case <-src.canceled:
		default:
			t.Fatal("Close did not cancel the source")
		}
		if got := s.Take(); !slices.Equal(got.Types, []string{"ticket"}) {
			t.Fatalf("Take after Close = %v", got.Types)
		}
		if late := b.Subscribe(""); late.Resync() {
			t.Fatal("Subscribe after Close resynced")
		}
	})
	t.Run("source closing its channel stops the pump", func(t *testing.T) {
		b, err := New()
		if err != nil {
			t.Fatal(err)
		}
		src := newFakeSource()
		if err := b.Start(src); err != nil {
			t.Fatal(err)
		}
		close(src.ch)
		select {
		case <-b.done:
		case <-time.After(2 * time.Second):
			t.Fatal("pump did not return when the source closed")
		}
		b.Close()
	})
	t.Run("never started bus serves subscriptions", func(t *testing.T) {
		b, _ := New()
		s := b.Subscribe("")
		defer s.Close()
		if s.Resync() || woke(s) || len(s.Take().Types) != 0 {
			t.Fatal("unstarted bus reported changes")
		}
	})
}

// TestBus_ConcurrentSubscribe checks under -race that subscriptions made
// concurrently all see a later write.
func TestBus_ConcurrentSubscribe(t *testing.T) {
	b, src := started(t)
	const n = 50
	subs := make([]*Subscription, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			subs[i] = b.Subscribe("")
			subs[i].Take()
		})
	}
	wg.Wait()
	send(t, b, src, entity("ticket"))
	for i, s := range subs {
		if got := s.Take(); !slices.Equal(got.Types, []string{"ticket"}) {
			t.Fatalf("sub %d: Take = %v", i, got.Types)
		}
		s.Close()
	}
}
