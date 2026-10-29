package realtime

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// busyRealtime declares the session types the busy tests seed:
//
//   - run: one per ticket, waits for the owner in queued and asking, and
//     works in planning;
//   - instant: one per ticket, starts in a working state;
//   - private: like run, but only the owner may read it;
//   - runs: many per ticket, like run;
//   - pagerun: one per page family, like run;
//   - facerun: one per page face, like run;
//   - chat: no session;
//   - idle: a session without a working state.
const busyRealtime = `
version: 1
channel_types:
  run:
    label: Agent run
    attach_to: [ticket]
    read: { from: attached }
    roles: &roles
      owner:  { from: entity.owner }
      member: { from: attached }
    message_types: &types
      chat:   { properties: { body: { type: string } }, post: [member] }
      claim:  { properties: { body: { type: string } }, post: [owner, member] }
      ask:    { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { body: { type: string } }, post: [owner] }
    lifecycle: &lifecycle
      close:   { roles: [member] }
      archive: { roles: [member], from: closed }
    session: &session
      initial: queued
      states:
        queued:   { label: Queued, waiting_for: [owner], on: { claim: { to: planning } } }
        planning: { label: Planning, status: working, on: { ask: { to: asking } } }
        asking:   { label: Asking, waiting_for: [owner], on: { answer: { to: planning } } }
  instant:
    label: Instant
    attach_to: [ticket]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session:
      initial: planning
      states:
        planning: { label: Planning, status: working, on: { ask: { to: asking } } }
        asking:   { label: Asking, waiting_for: [owner], on: { answer: { to: planning } } }
  private:
    label: Private run
    attach_to: [ticket]
    read: { from: entity.owner }
    roles: *roles
    message_types: *types
    session: *session
  runs:
    label: Runs
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles: *roles
    message_types: *types
    session: *session
  pagerun:
    label: Page run
    attach_to: [page]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session: *session
  facerun:
    label: Face run
    attach_to: [page]
    anchor: face
    read: { from: attached }
    roles: *roles
    message_types: *types
    session: *session
  chat:
    attach_to: [ticket]
    read: { from: attached }
    roles: *roles
    message_types: *types
  idle:
    attach_to: [ticket]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session:
      initial: queued
      states:
        queued: { waiting_for: [owner], on: { claim: { to: done } } }
        done:   { final: true }
`

func newBusyHarness(t *testing.T) (*harness, *BusyReader) {
	t.Helper()
	h := newHarness(t, withYAML(busyRealtime))
	h.anchors.views = map[string]AnchorView{}
	b, err := NewBusyReader(h.svc)
	require.NoError(t, err)
	return h, b
}

func busyAt(typ, anchor string) RawAddress {
	ent := "ticket"
	if typ == "pagerun" || typ == "facerun" {
		ent = "page"
	}
	return RawAddress{EntityType: ent, Anchor: anchor, Type: typ}
}

// ticketRow is the busy anchor of the ticket row TKT-1.
func ticketRow() BusyAnchor {
	return BusyAnchor{Ref: entity.Ref{ID: "TKT-1"}, Type: "ticket"}
}

// pageRow is the busy anchor of a page row on face.
func pageRow(id string, face entity.Face) BusyAnchor {
	return BusyAnchor{Ref: entity.Ref{ID: id, Face: face}, Type: "page"}
}

func rowAddr(a BusyAnchor) entity.Address { return entity.AddressOf(a.Ref) }

func (h *harness) busy(t *testing.T, b *BusyReader, a *fakeAccess, typ string, rows ...BusyAnchor) (
	map[entity.Address][]BusyView, bool,
) {
	t.Helper()
	got, ok, err := b.Busy(as(a), typ, rows)
	require.NoError(t, err)
	return got, ok
}

// TestBusy_States pins AC 2 of TKT-21G6JS for the session states: only a
// channel whose current state is working is busy, a never-moved session
// with a working initial state reads as that state since the channel was
// created, and closed, archived and stranded channels are not busy.
func TestBusy_States(t *testing.T) {
	t.Parallel()
	alice := user("alice")
	cases := []struct {
		name  string
		typ   string
		setup func(t *testing.T, h *harness, addr RawAddress)
		want  string // the busy state, "" for not busy
	}{
		{name: "planning", typ: "run", want: "planning", setup: func(t *testing.T, h *harness, addr RawAddress) {
			t.Helper()
			h.postTo(t, alice, addr, "claim", props("body", "mine"))
		}},
		{name: "asking", typ: "run", setup: func(t *testing.T, h *harness, addr RawAddress) {
			t.Helper()
			h.postTo(t, alice, addr, "claim", props("body", "mine"))
			h.postTo(t, alice, addr, "ask", props("body", "why?"))
		}},
		{name: "closed in planning", typ: "run", setup: func(t *testing.T, h *harness, addr RawAddress) {
			t.Helper()
			h.postTo(t, alice, addr, "claim", props("body", "mine"))
			_, err := h.svc.Lifecycle(as(alice), addr, "close")
			require.NoError(t, err)
		}},
		{name: "archived in planning", typ: "run", setup: func(t *testing.T, h *harness, addr RawAddress) {
			t.Helper()
			h.postTo(t, alice, addr, "claim", props("body", "mine"))
			_, err := h.svc.Lifecycle(as(alice), addr, "close")
			require.NoError(t, err)
			_, err = h.svc.Lifecycle(as(alice), addr, "archive")
			require.NoError(t, err)
		}},
		{
			// The stored state planning is renamed to thinking. The query
			// lists only declared working states, so the store never
			// returns the stranded channel.
			name: "stored state no longer declared", typ: "run",
			setup: func(t *testing.T, h *harness, addr RawAddress) {
				t.Helper()
				h.postTo(t, alice, addr, "claim", props("body", "mine"))
				h.cfg.set(Snapshot{Config: testCompiled(t, replaceState(busyRealtime, "planning", "thinking"))})
			},
		},
		{name: "never moved, initial queued", typ: "run", setup: func(t *testing.T, h *harness, addr RawAddress) {
			t.Helper()
			h.postTo(t, alice, addr, "chat", props("body", "hi"))
		}},
		{
			name: "never moved, initial working", typ: "instant", want: "planning",
			setup: func(t *testing.T, h *harness, addr RawAddress) {
				t.Helper()
				h.postTo(t, alice, addr, "chat", props("body", "hi"))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, b := newBusyHarness(t)
			h.ticket("TKT-1", "alice")
			addr := busyAt(tc.typ, "TKT-1")
			created := h.clock.Now()
			tc.setup(t, h, addr)
			h.clock.Add(time.Hour)
			got, ok := h.busy(t, b, alice, tc.typ, ticketRow())
			require.True(t, ok)
			if tc.want == "" {
				require.Empty(t, got)
				return
			}
			vs := got[rowAddr(ticketRow())]
			require.Len(t, vs, 1)
			v := vs[0]
			require.Equal(t, tc.want, v.State)
			require.Equal(t, "Planning", v.Label)
			require.Equal(t, tc.typ, v.Type)
			require.Empty(t, v.Name)
			if tc.typ == "instant" {
				require.Equal(t, "Instant", v.TypeLabel)
				head, err := h.store.Head(t.Context(), channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "instant"})
				require.NoError(t, err)
				require.True(t, v.Since.Equal(head.CreatedAt), "since %v, created %v", v.Since, head.CreatedAt)
				require.True(t, v.Since.Equal(created))
			} else {
				require.Equal(t, "Agent run", v.TypeLabel)
			}
		})
	}
}

// replaceState renames the state from to to in every session of yaml.
func replaceState(yaml, from, to string) string { return strings.ReplaceAll(yaml, from, to) }

// TestBusy_ReadRole pins AC 2 for the read role: a channel whose read
// role does not name the caller is not busy for them, and an anchor the
// caller may not read is absent.
func TestBusy_ReadRole(t *testing.T) {
	t.Parallel()
	h, b := newBusyHarness(t)
	alice, bob := user("alice"), user("bob")
	h.ticket("TKT-1", "alice")
	h.postTo(t, alice, busyAt("private", "TKT-1"), "claim", props("body", "mine"))
	h.postTo(t, alice, busyAt("run", "TKT-1"), "claim", props("body", "mine"))

	got, ok := h.busy(t, b, alice, "private", ticketRow())
	require.True(t, ok)
	require.Len(t, got[rowAddr(ticketRow())], 1)

	got, ok = h.busy(t, b, bob, "private", ticketRow())
	require.True(t, ok)
	require.Empty(t, got)

	// run's read role is attached, so only the anchor gate can drop it.
	got, ok = h.busy(t, b, bob, "run", ticketRow())
	require.True(t, ok)
	require.Len(t, got[rowAddr(ticketRow())], 1)

	// The anchor turns unreadable: nobody sees the channel.
	h.anchors.mu.Lock()
	delete(h.anchors.views, "TKT-1")
	h.anchors.mu.Unlock()
	got, ok = h.busy(t, b, bob, "run", ticketRow())
	require.True(t, ok)
	require.Empty(t, got)
}

// TestBusy_Anchor pins AC 2 for the anchor: a face type answers the row's
// face only, and a family type answers every face row of the entity.
func TestBusy_Anchor(t *testing.T) {
	t.Parallel()
	alice := user("alice")
	draft, published := pageRow("PAGE-1", "draft"), pageRow("PAGE-1", "published")

	t.Run("face", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.page("PAGE-1", "alice", "alice")
		h.postTo(t, alice, busyAt("facerun", "PAGE-1@draft"), "claim", props("body", "mine"))
		got, ok := h.busy(t, b, alice, "facerun", draft, published)
		require.True(t, ok)
		require.Len(t, got[rowAddr(draft)], 1)
		require.Empty(t, got[rowAddr(published)])
	})

	t.Run("family", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.page("PAGE-1", "alice", "alice")
		h.postTo(t, alice, busyAt("pagerun", "PAGE-1"), "claim", props("body", "mine"))
		h.store.waitingLists.Store(0)
		got, ok := h.busy(t, b, alice, "pagerun", draft, published)
		require.True(t, ok)
		require.Len(t, got[rowAddr(draft)], 1)
		require.Len(t, got[rowAddr(published)], 1)
		require.Equal(t, int64(1), h.store.waitingLists.Load())
	})

	// A page with rows of several types is served: rows of a type the
	// channel type attaches to get their entries, and the others are left
	// out of the query and the answer.
	t.Run("mixed types", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.ticket("TKT-1", "alice")
		h.page("PAGE-1", "alice", "alice")
		h.postTo(t, alice, busyAt("run", "TKT-1"), "claim", props("body", "mine"))
		h.store.waitingLists.Store(0)
		got, ok := h.busy(t, b, alice, "run", ticketRow(), published)
		require.True(t, ok)
		require.Len(t, got[rowAddr(ticketRow())], 1)
		require.NotContains(t, got, rowAddr(published))
		require.Equal(t, int64(1), h.store.waitingLists.Load())
		require.Equal(t, []channels.Anchor{{ID: "TKT-1"}}, h.store.lastWaiting.Load().Anchors)
	})
}

// TestBusy_ManyChannels pins AC 2 for cardinality many: every busy
// channel of the row is listed, the one longest in its state first.
func TestBusy_ManyChannels(t *testing.T) {
	t.Parallel()
	h, b := newBusyHarness(t)
	alice := user("alice")
	h.ticket("TKT-1", "alice")
	var names []string
	for range 3 {
		addr := h.threadOf(t, alice, "runs", "TKT-1")
		names = append(names, addr.Name)
		h.postTo(t, alice, addr, "chat", props("body", "hi"))
	}
	// Two of the three are busy. The later name is claimed first, so the
	// order cannot come from the name.
	busy := slices.Clone(names[:2])
	slices.Sort(busy)
	slices.Reverse(busy)
	for _, name := range busy {
		h.postTo(t, alice, RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "runs", Name: name}, "claim",
			props("body", "mine"))
		h.clock.Add(time.Minute)
	}
	got, ok := h.busy(t, b, alice, "runs", ticketRow())
	require.True(t, ok)
	vs := got[rowAddr(ticketRow())]
	require.Len(t, vs, 2)
	order := []string{vs[0].Name, vs[1].Name}
	require.Equal(t, busy, order)
	require.True(t, vs[0].Since.Before(vs[1].Since))
}

// TestBusy_ChannelsOfAnotherCardinality pins the security review finding
// of TKT-21G6JS: named channels on a cardinality one type, as a type
// switched from many to one keeps, neither count toward the row ceiling
// nor show. A ceiling counted over them would fail the whole page before
// the read gate, for a caller who may not read them.
func TestBusy_ChannelsOfAnotherCardinality(t *testing.T) {
	t.Parallel()
	h, b := newBusyHarness(t)
	alice := user("alice")
	h.ticket("TKT-1", "alice")
	h.postTo(t, alice, busyAt("instant", "TKT-1"), "chat", props("body", "hi"))
	// Written straight to the store: the service refuses a named channel on
	// a one type. Never moved, they read as instant's working initial state.
	for range 2 {
		_, err := h.store.CreateMany(t.Context(), channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
			Type: "instant", By: channels.Author{User: "alice"}, At: h.clock.Now(),
		})
		require.NoError(t, err)
	}

	got, ok := h.busy(t, b, alice, "instant", ticketRow())
	require.True(t, ok)
	vs := got[rowAddr(ticketRow())]
	require.Len(t, vs, 1)
	require.Empty(t, vs[0].Name)
	q := h.store.lastWaiting.Load()
	require.Equal(t, channels.UnnamedOnly, q.Names)
	require.Equal(t, 1, q.MaxRows)
}

// threadOf creates a channel of the many type typ on id as p.
func (h *harness) threadOf(t *testing.T, p *fakeAccess, typ, id string) RawAddress {
	t.Helper()
	ch, err := h.svc.Create(as(p), "ticket", id, typ, CreateInput{})
	require.NoError(t, err)
	return RawAddress{EntityType: "ticket", Anchor: id, Type: typ, Name: ch.Name}
}

// TestBusy_NotServed pins AC 3: a read that cannot be served for a reason
// of configuration or identity answers ok=false and no error, with no
// store call; a store fault is an error.
func TestBusy_NotServed(t *testing.T) {
	t.Parallel()
	alice := user("alice")
	cases := []struct {
		name  string
		typ   string
		setup func(h *harness)
	}{
		{name: "realtime off", typ: "run", setup: func(h *harness) { h.cfg.set(Snapshot{}) }},
		{name: "degraded", typ: "run", setup: func(h *harness) {
			h.cfg.set(Snapshot{Config: h.cfg.Load().Config, Degraded: true})
		}},
		{name: "unknown type", typ: "nope"},
		{name: "no session", typ: "chat"},
		{name: "no working state", typ: "idle"},
		{name: "does not attach", typ: "pagerun"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, b := newBusyHarness(t)
			h.ticket("TKT-1", "alice")
			h.postTo(t, alice, busyAt("run", "TKT-1"), "claim", props("body", "mine"))
			if tc.setup != nil {
				tc.setup(h)
			}
			h.store.waitingLists.Store(0)
			got, ok, err := b.Busy(as(alice), tc.typ, []BusyAnchor{ticketRow()})
			require.NoError(t, err)
			require.False(t, ok)
			require.Empty(t, got)
			require.Zero(t, h.store.waitingLists.Load())
		})
	}

	t.Run("no principal", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.ticket("TKT-1", "alice")
		h.postTo(t, alice, busyAt("run", "TKT-1"), "claim", props("body", "mine"))
		h.store.waitingLists.Store(0)
		got, ok, err := b.Busy(failingAccess(ErrNoPrincipal), "run", []BusyAnchor{ticketRow()})
		require.NoError(t, err)
		require.False(t, ok)
		require.Empty(t, got)
		require.Zero(t, h.store.waitingLists.Load())
	})

	t.Run("no rows", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.store.waitingLists.Store(0)
		got, ok := h.busy(t, b, alice, "run")
		require.True(t, ok)
		require.Empty(t, got)
		require.Zero(t, h.store.waitingLists.Load())
	})

	t.Run("access fault", func(t *testing.T) {
		t.Parallel()
		_, b := newBusyHarness(t)
		_, ok, err := b.Busy(failingAccess(errors.New("disk")), "run", []BusyAnchor{ticketRow()})
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("store fault", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.ticket("TKT-1", "alice")
		h.store.waitingErr = channels.ErrTooManyRows
		_, ok, err := b.Busy(as(alice), "run", []BusyAnchor{ticketRow()})
		require.ErrorIs(t, err, channels.ErrTooManyRows)
		require.False(t, ok)
	})

	t.Run("anchor fault", func(t *testing.T) {
		t.Parallel()
		h, b := newBusyHarness(t)
		h.ticket("TKT-1", "alice")
		h.postTo(t, alice, busyAt("run", "TKT-1"), "claim", props("body", "mine"))
		h.anchors.fail = func(int64) error { return errors.New("disk") }
		_, ok, err := b.Busy(as(alice), "run", []BusyAnchor{ticketRow()})
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("nil service", func(t *testing.T) {
		t.Parallel()
		_, err := NewBusyReader(nil)
		require.Error(t, err)
	})
}
