package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// inboxRealtime declares the session types the inbox tests seed:
//
//   - case: one per ticket, waiting for the owner (lead) or a watcher;
//   - legacy: one per ticket; a chat moves it to a stored state waiting
//     for the owner, which the stranded row renames;
//   - secret: like case, but read needs case.read, which nobody holds;
//   - threads: many per ticket, waiting for the starter (the creator) or
//     the owner. reply needs the owner, wrap the starter, so the
//     transitions of two threads on one anchor differ by creator;
//   - pagechat: one per page family, waiting for the owner;
//   - facecase: one per page face, waiting for the owner.
const inboxRealtime = `
version: 1
channel_types:
  case:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:    { from: entity.owner, label: the lead }
      watcher: { from: entity.watchers, label: a watcher }
      member:  { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      text:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead, watcher] }
    lifecycle:
      close: { roles: [member] }
    session:
      initial: waiting
      states:
        waiting:
          label: Waiting
          waiting_for: [lead, watcher]
          on:
            answer: { to: done }
            text:   { to: working }
        working:
          on:
            answer: { to: waiting }
        done: { final: true }
  legacy:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: start
      states:
        start: { on: { chat: { to: ` + legacyState + ` } } }
        ` + legacyState + `: { waiting_for: [lead], on: { answer: { to: done } } }
        done: { final: true }
  secret:
    attach_to: [ticket]
    read: { permission: case.read }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: asked
      states:
        asked: { waiting_for: [lead], on: { answer: { to: done } } }
        done: { final: true }
  threads:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      starter: { creator: true, label: the starter }
      owner:   { from: entity.owner }
      member:  { from: attached }
    message_types:
      reply: { properties: { body: { type: string } }, post: [owner] }
      wrap:  { properties: { body: { type: string } }, post: [starter] }
    session:
      initial: asked
      states:
        asked:
          waiting_for: [starter, owner]
          on:
            reply: { to: answered, roles: [owner] }
            wrap:  { to: answered, roles: [starter] }
        answered: { final: true }
  pagechat:
    attach_to: [page]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: asked
      states:
        asked: { waiting_for: [lead], on: { answer: { to: done } } }
        done: { final: true }
  facecase:
    attach_to: [page]
    anchor: face
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: asked
      states:
        asked: { waiting_for: [lead], on: { answer: { to: done } } }
        done: { final: true }
`

// legacyState is the placeholder for the legacy type's waiting state.
const legacyState = "LEGACYSTATE"

// inboxYAML is inboxRealtime with the legacy waiting state named state.
func inboxYAML(state string) string {
	return strings.ReplaceAll(inboxRealtime, legacyState, state)
}

func newInboxHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, withYAML(inboxYAML("asked")))
	h.anchors.views = map[string]AnchorView{}
	return h
}

// ticket sets the view of a ticket whose owner and watchers are given.
func (h *harness) ticket(id string, owner any, watchers ...any) {
	h.anchors.mu.Lock()
	defer h.anchors.mu.Unlock()
	p := map[string]any{"title": "Title of " + id}
	if owner != nil {
		p["owner"] = owner
	}
	if len(watchers) > 0 {
		p["watchers"] = watchers
	}
	h.anchors.views[id] = AnchorView{ID: id, Type: "ticket", Title: "Title of " + id, Props: p}
}

// page sets the views of a page: the bare id serves the published face.
func (h *harness) page(id, publishedOwner, draftOwner string) {
	h.anchors.mu.Lock()
	defer h.anchors.mu.Unlock()
	pub := AnchorView{ID: id, Face: "published", Type: "page", Props: map[string]any{"owner": publishedOwner}}
	h.anchors.views[id] = pub
	h.anchors.views[id+"@published"] = pub
	h.anchors.views[id+"@draft"] = AnchorView{
		ID: id, Face: "draft", Type: "page", Props: map[string]any{"owner": draftOwner},
	}
}

// store stores the channel at addr by a chat post of a member, which moves
// no session.
func (h *harness) stored(t *testing.T, addr RawAddress) {
	t.Helper()
	h.postTo(t, user("seed-"+addr.Anchor+"-"+addr.Type, "case.read"), addr, "chat", props("body", "hello"))
}

func caseAt(id string) RawAddress { return RawAddress{EntityType: "ticket", Anchor: id, Type: "case"} }

// thread creates a threads channel on id as p.
func (h *harness) thread(t *testing.T, p *fakeAccess, id string) RawAddress {
	t.Helper()
	ch, err := h.svc.Create(as(p), "ticket", id, "threads", CreateInput{})
	require.NoError(t, err)
	return RawAddress{EntityType: "ticket", Anchor: id, Type: "threads", Name: ch.Name}
}

func (h *harness) inbox(t *testing.T, a *fakeAccess, req InboxRequest) InboxView {
	t.Helper()
	v, err := h.svc.Inbox(as(a), req)
	require.NoError(t, err)
	return v
}

func entryAddr(e InboxEntry) RawAddress {
	return RawAddress{EntityType: e.Channel.EntityType, Anchor: e.Channel.Anchor, Type: e.Channel.Type, Name: e.Channel.Name}
}

// TestInbox_MatchesWaitingOnMe pins AC 6 and AC 7: the inbox lists exactly
// the stored, open channels whose session answers waiting_on_me, and each
// entry's transitions are the session read's.
func TestInbox_MatchesWaitingOnMe(t *testing.T) {
	t.Parallel()
	h := newInboxHarness(t)
	alice := user("alice")
	acmeAlice := &fakeAccess{
		p:       principal.VerifiedFrom("alice", principal.ToolDataEntry, principal.Claims{OrgID: "acme"}),
		q:       alice.q,
		perms:   map[string]bool{},
		ceiling: true,
	}
	type row struct {
		name string
		addr RawAddress
		want bool
	}
	var rows []row
	add := func(name string, addr RawAddress, want bool) { rows = append(rows, row{name, addr, want}) }

	h.ticket("TKT-1", "alice")
	h.stored(t, caseAt("TKT-1"))
	add("holder", caseAt("TKT-1"), true)

	h.ticket("TKT-2", "bob")
	h.stored(t, caseAt("TKT-2"))
	add("not a holder", caseAt("TKT-2"), false)

	h.ticket("TKT-3", "alice")
	h.stored(t, caseAt("TKT-3"))
	_, err := h.svc.Lifecycle(as(user("bob")), caseAt("TKT-3"), "close")
	require.NoError(t, err)
	add("closed", caseAt("TKT-3"), false)

	h.ticket("TKT-4", "alice")
	h.postTo(t, alice, caseAt("TKT-4"), "answer", props("note", "done"))
	add("final", caseAt("TKT-4"), false)

	h.ticket("TKT-5", "alice")
	legacy := RawAddress{EntityType: "ticket", Anchor: "TKT-5", Type: "legacy"}
	h.stored(t, legacy)
	add("stranded", legacy, false)

	h.ticket("TKT-6", "alice")
	h.postTo(t, user("bob"), caseAt("TKT-6"), "text", props("body", "on it"))
	add("working", caseAt("TKT-6"), false)

	h.ticket("TKT-7", "alice")
	add("unstored cardinality one", caseAt("TKT-7"), false)

	h.ticket("TKT-8", "bob", "carol", "alice")
	h.stored(t, caseAt("TKT-8"))
	add("list property holds the caller", caseAt("TKT-8"), true)

	h.ticket("TKT-9", []any{"alice"})
	h.stored(t, caseAt("TKT-9"))
	add("list value on a scalar property", caseAt("TKT-9"), true)

	h.ticket("TKT-10", "bob")
	add("creator is the caller", h.thread(t, alice, "TKT-10"), true)
	add("creator is another user", h.thread(t, user("bob"), "TKT-10"), false)
	add("creator in another org", h.thread(t, acmeAlice, "TKT-10"), false)

	h.ticket("TKT-11", "alice")
	add("own thread on an owned anchor", h.thread(t, alice, "TKT-11"), true)
	add("other thread on an owned anchor", h.thread(t, user("bob"), "TKT-11"), true)

	h.page("PAGE-2", "bob", "alice")
	pagechat := RawAddress{EntityType: "page", Anchor: "PAGE-2", Type: "pagechat"}
	h.stored(t, pagechat)
	add("family anchor whose lower face names the caller", pagechat, false)

	h.page("PAGE-3", "alice", "alice")
	facecase := RawAddress{EntityType: "page", Anchor: "PAGE-3@draft", Type: "facecase"}
	h.stored(t, facecase)
	readable := RawAddress{EntityType: "page", Anchor: "PAGE-3@published", Type: "facecase"}
	h.stored(t, readable)
	add("face channel on a readable face", readable, true)
	h.anchors.mu.Lock()
	delete(h.anchors.views, "PAGE-3@draft")
	h.anchors.mu.Unlock()
	add("face channel on an unreadable face", facecase, false)

	// The legacy state is renamed after the seed: its channel is stranded.
	h.cfg.set(Snapshot{Config: testCompiled(t, inboxYAML("pending"))})

	view := h.inbox(t, alice, InboxRequest{Limit: MaxInboxLimit})
	require.Equal(t, UntrustedNotice, view.Notice)
	require.False(t, view.Truncated)
	got := map[RawAddress]InboxEntry{}
	for _, e := range view.Entries {
		got[entryAddr(e)] = e
	}
	require.Len(t, got, len(view.Entries), "an entry is listed once")
	require.Equal(t, len(view.Entries), view.Count)

	for _, r := range rows {
		e, listed := got[r.addr]
		require.Equalf(t, r.want, listed, "%s: listed", r.name)
		read, err := h.svc.Session(as(alice), r.addr)
		onMe := err == nil && read.Session != nil && read.Session.WaitingOnMe
		if r.name != "unstored cardinality one" {
			require.Equalf(t, listed, onMe, "%s: the inbox and read_session disagree", r.name)
		}
		if !listed {
			continue
		}
		require.Equal(t, InboxKindWaiting, e.Kind)
		require.Equalf(t, read.Transitions, e.Transitions, "%s: transitions", r.name)
		require.Equal(t, read.Session.State, e.Session.State)
		require.Equal(t, read.Session.EnteredAt, e.Since)
		require.NotEmpty(t, e.Roles, r.name)
	}
	require.Len(t, got, 7)
	require.Equal(t, []string{"lead"}, got[caseAt("TKT-1")].Roles)
	require.Equal(t, []string{"watcher"}, got[caseAt("TKT-8")].Roles)
	require.Equal(t, "Title of TKT-1", got[caseAt("TKT-1")].Channel.AnchorTitle)
	own := got[rows[slices.IndexFunc(rows, func(r row) bool { return r.name == "own thread on an owned anchor" })].addr]
	other := got[rows[slices.IndexFunc(rows, func(r row) bool { return r.name == "other thread on an owned anchor" })].addr]
	require.Equal(t, []string{"starter", "owner"}, own.Roles)
	require.Equal(t, []string{"owner"}, other.Roles)
	require.ElementsMatch(t, []string{"reply", "wrap"}, own.Transitions)
	require.Equal(t, []string{"reply"}, other.Transitions)
}

// seedVisible seeds n case channels waiting on alice, on TKT-100 onward,
// one microsecond apart.
func (h *harness) seedVisible(t *testing.T, n int) {
	t.Helper()
	for i := range n {
		id := fmt.Sprintf("TKT-%d", 100+i*2)
		h.ticket(id, "alice")
		h.stored(t, caseAt(id))
		h.clock.Add(time.Microsecond)
	}
}

// seedHidden seeds rows the caller must not see, between the visible rows
// seedVisible seeded: an unreadable entity, a role property hidden on that
// entity, and, on a visible anchor, a channel whose read role the caller
// fails. The last keeps the holder anchors as they were: its anchor is a
// readable holder either way.
func (h *harness) seedHidden(t *testing.T, n, visible int) {
	t.Helper()
	at := h.clock.Now()
	for i := range n {
		id := fmt.Sprintf("TKT-%d", 101+i*2)
		switch i % 3 {
		case 0:
			h.ticket(id, "alice")
			h.stored(t, caseAt(id))
			h.anchors.mu.Lock()
			delete(h.anchors.views, id)
			h.anchors.mu.Unlock()
		case 1:
			on := fmt.Sprintf("TKT-%d", 100+(i%visible)*2)
			h.stored(t, RawAddress{EntityType: "ticket", Anchor: on, Type: "secret"})
		default:
			h.ticket(id, "alice")
			h.stored(t, caseAt(id))
			h.anchors.mu.Lock()
			h.anchors.hideAt = map[string][]string{id: {"owner"}}
			h.anchors.mu.Unlock()
		}
	}
	h.clock.mu.Lock()
	h.clock.now = at
	h.clock.mu.Unlock()
}

// hideOwners hides the owner of every id in ids.
func (h *harness) hideOwners(ids []string) {
	h.anchors.mu.Lock()
	defer h.anchors.mu.Unlock()
	h.anchors.hideAt = map[string][]string{}
	for _, id := range ids {
		h.anchors.hideAt[id] = []string{"owner"}
	}
}

func inboxJSON(t *testing.T, v InboxView) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// lowerAnchorBound lowers the anchor ceiling of h's service only.
func lowerAnchorBound(h *harness, n int) { h.svc.bounds.anchors = n }

// TestInbox_HiddenRowsChangeNothing pins AC 8: rows the caller may not see
// change no byte of the answer, at a limit that pages and at the anchor
// bound.
func TestInbox_HiddenRowsChangeNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		limit int
		bound int
	}{
		{name: "paged", limit: 4},
		{name: "at the anchor bound", limit: 50, bound: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plain := newInboxHarness(t)
			plain.seedVisible(t, 9)
			mixed := newInboxHarness(t)
			if tc.bound > 0 {
				lowerAnchorBound(plain, tc.bound)
				lowerAnchorBound(mixed, tc.bound)
			}
			mixed.seedVisible(t, 9)
			mixed.seedHidden(t, 9, 9)
			var hidden []string
			for i := 2; i < 9; i += 3 {
				hidden = append(hidden, fmt.Sprintf("TKT-%d", 101+i*2))
			}
			mixed.hideOwners(hidden)

			alice := user("alice")
			cursor := ""
			for page := 0; ; page++ {
				base := plain.inbox(t, alice, InboxRequest{Limit: tc.limit, Cursor: cursor})
				withHidden := mixed.inbox(t, alice, InboxRequest{Limit: tc.limit, Cursor: cursor})
				require.Equal(t, inboxJSON(t, base), inboxJSON(t, withHidden), "page %d", page)
				if tc.bound > 0 {
					require.Equal(t, tc.bound, base.Count)
					require.False(t, base.Truncated, "the bound holds exactly the readable holders")
				}
				if base.NextCursor == "" {
					break
				}
				cursor = base.NextCursor
			}
		})
	}
}

// TestInbox_TruncatedIsPostGate pins AC 9: truncated counts gated,
// redacted holder anchors only, and keeps the first in (id, face) order.
func TestInbox_TruncatedIsPostGate(t *testing.T) {
	t.Parallel()
	alice := user("alice")

	t.Run("bound plus one readable holders", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		lowerAnchorBound(h, 3)
		h.seedVisible(t, 4)
		first := h.inbox(t, alice, InboxRequest{})
		require.True(t, first.Truncated)
		require.Equal(t, 3, first.Count)
		for i, e := range first.Entries {
			require.Equal(t, fmt.Sprintf("TKT-%d", 100+i*2), e.Channel.Anchor)
		}
		require.Equal(t, inboxJSON(t, first), inboxJSON(t, h.inbox(t, alice, InboxRequest{})))
	})

	t.Run("bound readable plus hidden holders", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		lowerAnchorBound(h, 3)
		h.seedVisible(t, 3)
		h.seedHidden(t, 12, 3)
		var hidden []string
		for i := 2; i < 12; i += 3 {
			hidden = append(hidden, fmt.Sprintf("TKT-%d", 101+i*2))
		}
		h.hideOwners(hidden)
		got := h.inbox(t, alice, InboxRequest{})
		require.False(t, got.Truncated)
		require.Equal(t, 3, got.Count)
	})
}

// TestInbox_Paging pins AC 10: oldest wait first, pages without gaps or
// repeats across equal and one-microsecond-apart since values.
func TestInbox_Paging(t *testing.T) {
	t.Parallel()
	h := newInboxHarness(t)
	for i := range 75 {
		id := fmt.Sprintf("TKT-%03d", 500-i)
		h.ticket(id, "alice")
		h.stored(t, caseAt(id))
		if i%3 == 2 {
			h.clock.Add(time.Microsecond)
		}
	}
	alice := user("alice")
	all := h.inbox(t, alice, InboxRequest{Limit: MaxInboxLimit})
	require.Len(t, all.Entries, 75)
	require.Empty(t, all.NextCursor)
	for i := 1; i < len(all.Entries); i++ {
		a, b := all.Entries[i-1], all.Entries[i]
		require.False(t, b.Since.Before(a.Since), "since ascending")
		if a.Since.Equal(b.Since) {
			require.Less(t, a.Channel.Anchor, b.Channel.Anchor, "id breaks a tie")
		}
	}
	var paged []InboxEntry
	cursor, pages := "", 0
	for {
		v := h.inbox(t, alice, InboxRequest{Limit: 20, Cursor: cursor})
		require.Equal(t, 75, v.Count)
		paged = append(paged, v.Entries...)
		pages++
		if v.NextCursor == "" {
			break
		}
		cursor = v.NextCursor
	}
	require.Equal(t, 4, pages)
	require.Equal(t, inboxJSON(t, InboxView{Entries: all.Entries}), inboxJSON(t, InboxView{Entries: paged}))
	// Page boundaries 20 and 40 split equal since values, 60 values one
	// microsecond apart.
	require.Equal(t, all.Entries[19].Since, all.Entries[20].Since)
	require.Equal(t, all.Entries[39].Since, all.Entries[40].Since)
	require.Equal(t, time.Microsecond, all.Entries[60].Since.Sub(all.Entries[59].Since))

	t.Run("default and bounds", func(t *testing.T) {
		t.Parallel()
		require.Len(t, h.inbox(t, alice, InboxRequest{}).Entries, DefaultInboxLimit)
		for _, limit := range []int{-1, MaxInboxLimit + 1} {
			_, err := h.svc.Inbox(as(alice), InboxRequest{Limit: limit})
			requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
		}
	})
}

func cursorOf(t *testing.T, c inboxCursor) string {
	t.Helper()
	b, err := json.Marshal(c)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

// TestInbox_CursorForged pins AC 10's cursor rules: a malformed cursor is
// invalid_cursor before any read; a cursor naming a hidden key, or made by
// another principal, only positions.
func TestInbox_CursorForged(t *testing.T) {
	t.Parallel()
	h := newInboxHarness(t)
	h.seedVisible(t, 6)
	alice := user("alice")
	all := h.inbox(t, alice, InboxRequest{}).Entries

	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		fresh := newInboxHarness(t)
		for _, c := range []string{
			"!!", base64.RawURLEncoding.EncodeToString([]byte("{")),
			cursorOf(t, inboxCursor{V: 2, ID: "TKT-1", Type: "case"}),
			cursorOf(t, inboxCursor{V: 1, Type: "case"}),
			cursorOf(t, inboxCursor{V: 1, ID: "TKT-1"}),
			cursorOf(t, inboxCursor{V: 1, ID: "not an id", Type: "case"}),
			cursorOf(t, inboxCursor{V: 1, ID: "TKT-1", Face: "Bad Face", Type: "case"}),
			cursorOf(t, inboxCursor{V: 1, ID: "TKT-1", Type: "Bad Type"}),
			cursorOf(t, inboxCursor{V: 1, ID: "TKT-1", Type: "case", Name: "Bad Name"}),
		} {
			before := fresh.store.waitingLists.Load()
			_, err := fresh.svc.Inbox(as(alice), InboxRequest{Cursor: c})
			requireDenial(t, err, http.StatusBadRequest, TypeInvalidCursor)
			require.Equal(t, before, fresh.store.waitingLists.Load(), "no read before the cursor is checked")
		}
	})

	t.Run("a hidden key positions only", func(t *testing.T) {
		t.Parallel()
		// TKT-103 is no anchor: it sorts between TKT-102 and TKT-104.
		forged := cursorOf(t, inboxCursor{V: 1, Since: all[1].Since, ID: "TKT-103", Type: "case"})
		nearest := cursorOf(t, inboxCursor{V: 1, Since: all[1].Since, ID: all[1].Channel.Anchor, Type: "case"})
		got := h.inbox(t, alice, InboxRequest{Limit: 2, Cursor: forged})
		want := h.inbox(t, alice, InboxRequest{Limit: 2, Cursor: nearest})
		require.Equal(t, inboxJSON(t, want), inboxJSON(t, got))
		require.Equal(t, all[2].Channel.Anchor, got.Entries[0].Channel.Anchor)
	})

	t.Run("another principal's cursor", func(t *testing.T) {
		t.Parallel()
		bobs := h.inbox(t, user("bob"), InboxRequest{Limit: 1})
		require.Empty(t, bobs.Entries)
		cur := cursorOf(t, inboxCursor{V: 1, Since: all[3].Since.Add(time.Nanosecond), ID: "ZZZ", Type: "case"})
		got := h.inbox(t, alice, InboxRequest{Cursor: cur})
		require.Len(t, got.Entries, 2, "a cursor time is normalized to microseconds")
	})
}

// TestInbox_DegradedAndOff pins AC 12 at the service: a degraded
// configuration answers empty with no store call; no identity is
// no_principal; too many candidate channels is inbox_too_large; a store
// fault is not a denial.
func TestInbox_DegradedAndOff(t *testing.T) {
	t.Parallel()
	alice := user("alice")

	t.Run("degraded", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		h.seedVisible(t, 2)
		h.cfg.set(Snapshot{Config: h.cfg.Load().Config, Degraded: true})
		h.store.waitingLists.Store(0)
		h.holders.calls.Store(0)
		got := h.inbox(t, alice, InboxRequest{})
		require.True(t, got.Degraded)
		require.NotNil(t, got.Entries)
		require.Empty(t, got.Entries)
		require.Zero(t, h.store.waitingLists.Load())
		require.Zero(t, h.holders.calls.Load())
		require.JSONEq(t, `{"notice":"`+UntrustedNotice+`","entries":[],"count":0,"truncated":false,"degraded":true}`,
			inboxJSON(t, got))
	})

	t.Run("off", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		h.cfg.set(Snapshot{})
		_, err := h.svc.Inbox(as(alice), InboxRequest{})
		require.ErrorIs(t, err, ErrRealtimeOff)
	})

	t.Run("no principal", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		nobody := &fakeAccess{p: principal.Principal{User: "x", Tool: principal.ToolDataEntry}, ceiling: true}
		_, err := h.svc.Inbox(as(nobody), InboxRequest{})
		requireDenial(t, err, http.StatusForbidden, TypeNoPrincipal)
	})

	t.Run("too many channels", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		h.seedVisible(t, 3)
		h.svc.bounds.rows = 2
		_, err := h.svc.Inbox(as(alice), InboxRequest{})
		requireDenial(t, err, http.StatusServiceUnavailable, TypeInboxTooLarge)
	})

	t.Run("holder fault", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		h.seedVisible(t, 1)
		h.holders.err = errors.New("disk")
		_, err := h.svc.Inbox(as(alice), InboxRequest{})
		require.Error(t, err)
		_, denial := errors.AsType[*DenialError](err)
		require.False(t, denial)
	})

	t.Run("no waiting roles", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.store.waitingLists.Store(0)
		got := h.inbox(t, alice, InboxRequest{})
		require.Empty(t, got.Entries)
		require.Zero(t, h.store.waitingLists.Load())
	})
}

// TestInbox_Budget pins the service half of AC 11: one holder query per
// waiting pair, one channel listing, one anchor batch and one prime,
// whatever the number of entries.
func TestInbox_Budget(t *testing.T) {
	t.Parallel()
	for _, n := range []int{3, 12} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			h := newInboxHarness(t)
			h.seedVisible(t, n)
			alice := user("alice")
			h.holders.calls.Store(0)
			h.anchors.calls.Store(0)
			h.anchors.anchorsCalls.Store(0)
			h.store.heads.Store(0)
			h.store.entityLists.Store(0)
			h.store.waitingLists.Store(0)
			got := h.inbox(t, alice, InboxRequest{})
			require.Len(t, got.Entries, n)
			pairs := len(h.cfg.Load().Config.Waiting().Pairs)
			require.Equal(t, int64(pairs), h.holders.calls.Load())
			require.Equal(t, int64(1), h.store.waitingLists.Load())
			require.Equal(t, int64(1), h.anchors.anchorsCalls.Load())
			require.Zero(t, h.anchors.calls.Load())
			require.Zero(t, h.store.heads.Load())
			require.Zero(t, h.store.entityLists.Load())
			require.Equal(t, int64(1), alice.primeCalls.Load())
		})
	}
}

// TestInboxView_MarshalJSON pins that an answer without the waiting kind
// drops exactly the waiting fields: every other JSON field of InboxView,
// set to a non-zero value, is in it, so a field added later cannot vanish.
func TestInboxView_MarshalJSON(t *testing.T) {
	t.Parallel()
	n := 1
	v := InboxView{
		Notice: "n", Entries: []InboxEntry{{Kind: "k"}}, Count: 1, Truncated: true, Degraded: true,
		NextCursor: "c", UnreadChannels: []InboxUnread{{Seq: 1}}, UnreadCount: &n,
	}
	var all []string
	rt := reflect.TypeFor[InboxView]()
	for i := range rt.NumField() {
		f := rt.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || name == "-" {
			continue
		}
		require.False(t, reflect.ValueOf(v).Field(i).IsZero(), "set %s in this test", f.Name)
		all = append(all, name)
	}
	for _, k := range waitingJSON {
		require.Contains(t, all, k, "waitingJSON names a field")
	}
	keys := func(v InboxView) []string {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		var m map[string]any
		require.NoError(t, json.Unmarshal(b, &m))
		return slices.Sorted(maps.Keys(m))
	}
	slices.Sort(all)
	require.Equal(t, all, keys(v))
	v.noWaiting = true
	want := slices.DeleteFunc(slices.Clone(all), func(k string) bool { return slices.Contains(waitingJSON, k) })
	require.Equal(t, want, keys(v))
}
