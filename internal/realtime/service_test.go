package realtime

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// TestNew_RejectsNil pins that New refuses every missing collaborator.
func TestNew_RejectsNil(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	full := Deps{Store: h.store, Config: h.cfg, Access: fakeAccessSource{}, Anchors: h.anchors, Refs: h.refs, Local: h.local}
	tests := []struct {
		name string
		mut  func(*Deps)
	}{
		{"Store", func(d *Deps) { d.Store = nil }},
		{"Config", func(d *Deps) { d.Config = nil }},
		{"Access", func(d *Deps) { d.Access = nil }},
		{"Anchors", func(d *Deps) { d.Anchors = nil }},
		{"Refs", func(d *Deps) { d.Refs = nil }},
		{"Local", func(d *Deps) { d.Local = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := full
			tc.mut(&d)
			_, err := New(d)
			require.ErrorContains(t, err, tc.name)
		})
	}
	_, err := NewLocal(nil)
	require.Error(t, err)
}

// TestPost_PersistsAndRecords pins a persistent post: it appends, records
// the channel type shapes once, marks the channel changed and renders the
// author and props.
func TestPost_PersistsAndRecords(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	sub := h.svc.Subscribe()
	defer sub.Close()
	m := h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{
		"level": 2, "on": "2026-10-12", "at": "2026-10-12T09:00:00Z", "ok": true, "pick": "high", "also": "TKT-2",
	}})
	require.EqualValues(t, 1, m.Seq)
	require.Equal(t, "bob", m.Author.User)
	require.Equal(t, "note", m.Type)
	require.EqualValues(t, 2, m.Props["level"])
	require.Equal(t, "high", m.Props["pick"])
	require.Equal(t, []channels.Key{roomKey()}, takeKeys(t, sub))
	h.post(t, user("bob"), text("again"))
	require.EqualValues(t, 1, h.store.records.Load(), "type records are written once per process")
}

// TestPost_Validation pins the validation answers of a post; none of them
// reaches the store.
func TestPost_Validation(t *testing.T) {
	t.Parallel()
	many := map[string]any{"level": 1}
	for i := range channels.MaxProps + 1 {
		many[fmt.Sprintf("p%d", i)] = "v"
	}
	bad := func(k string, v any) map[string]any { return map[string]any{"level": 1, k: v} }
	tests := []struct {
		name   string
		in     PostInput
		status int
		typ    string
	}{
		{"unknown type", PostInput{Type: "nope"}, http.StatusBadRequest, TypeInvalid},
		{"missing required", PostInput{Type: "text"}, http.StatusBadRequest, TypeInvalid},
		{"undeclared prop", PostInput{Type: "text", Props: map[string]any{"body": "x", "y": 1}},
			http.StatusBadRequest, TypeInvalid},
		{"too many props", PostInput{Type: "note", Props: many}, http.StatusBadRequest, TypeTooManyProps},
		{"over max", PostInput{Type: "text", Props: map[string]any{"body": strings.Repeat("x", 101)}},
			http.StatusBadRequest, TypeInvalid},
		{"bad integer", PostInput{Type: "note", Props: map[string]any{"level": 1.5}}, http.StatusBadRequest, TypeInvalid},
		{"bad date", PostInput{Type: "note", Props: bad("on", "tomorrow")}, http.StatusBadRequest, TypeInvalid},
		{"bad datetime", PostInput{Type: "note", Props: bad("at", "noon")}, http.StatusBadRequest, TypeInvalid},
		{"bad boolean", PostInput{Type: "note", Props: bad("ok", "yes")}, http.StatusBadRequest, TypeInvalid},
		{"bad enum", PostInput{Type: "note", Props: bad("pick", "mid")}, http.StatusBadRequest, TypeInvalid},
		{"when fails", PostInput{Type: "note", Props: map[string]any{"level": 0}}, http.StatusForbidden, TypePostDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			_, err := h.svc.Post(as(user("bob")), room, tc.in)
			requireDenial(t, err, tc.status, tc.typ)
			require.Zero(t, h.store.appends.Load())
		})
	}
}

// TestPost_ReplyToMissing pins that a reply to a message that does not
// exist is a 400.
func TestPost_ReplyToMissing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), text("x"))
	_, err := h.svc.Post(as(user("bob")), room, PostInput{Type: "text", Props: map[string]any{"body": "x"}, ReplyTo: 9})
	requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
}

// TestPost_NullDropped pins that a null property value is dropped, not
// stored.
func TestPost_NullDropped(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": 1, "about": nil}})
	_, ok := m.Props["about"]
	require.False(t, ok)
}

// TestMessages_PageClamp pins that a page never returns more than the
// configured page limit.
func TestMessages_PageClamp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for i := range 55 {
		h.local.limiter = newLimiter(defaultLimitCaps()) // keep the rate limit out of this test
		h.post(t, user("bob"), text(fmt.Sprintf("m%d", i)))
	}
	page, err := h.svc.Messages(as(user("bob")), room, channels.Page{Limit: 500})
	require.NoError(t, err)
	require.Len(t, page.Messages, 50)
	page, err = h.svc.Messages(as(user("bob")), room, channels.Page{AfterSeq: 50})
	require.NoError(t, err)
	require.Len(t, page.Messages, 5)
}

// TestCreate_ChannelsPerEntity pins the per-anchor channel cap.
func TestCreate_ChannelsPerEntity(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice := user("alice", "chat.read")
	for i := range 3 {
		_, err := h.svc.Create(as(alice), "ticket", "TKT-1", "topics", CreateInput{Title: "t"})
		require.NoError(t, err, "create %d", i)
	}
	_, err := h.svc.Create(as(alice), "ticket", "TKT-1", "topics", CreateInput{Title: "t"})
	requireDenial(t, err, http.StatusConflict, TypeTooManyChannels)
}

// TestHead_Affordances pins the actions a head offers per caller.
func TestHead_Affordances(t *testing.T) {
	// Sequential: the subtests share one harness and the code around them.
	h := newHarness(t)
	tests := []struct {
		name   string
		access *fakeAccess
		want   Affordances
	}{
		{"member", user("bob"), Affordances{
			Post:    []string{"cursor", "note", "text", "typing"},
			EditOwn: true, DeleteOwn: true,
		}},
		{"owner and moderator", user("alice", "chat.moderate", "chat.post"), Affordances{
			Post:    []string{"cursor", "note", "owner-text", "perm-text", "text", "typing"},
			EditOwn: true, DeleteOwn: true, Remove: true,
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			head, err := h.svc.Head(as(tc.access), room)
			require.NoError(t, err)
			require.Equal(t, tc.want, head.Actions)
		})
	}
	head, err := h.svc.Head(as(readOnlyClient(user("alice", "chat.moderate"))), room)
	require.NoError(t, err)
	require.Empty(t, head.Actions.Post)
	require.False(t, head.Actions.Remove)
	require.False(t, head.Actions.EditOwn)
}

// TestListAnchor pins that listing an anchor's channels makes one anchor
// read, lists every readable type attached to the anchor's type, and
// leaves out a type the caller cannot read.
func TestListAnchor(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), text("x"))
	_, err := h.svc.Create(as(user("bob", "chat.read")), "ticket", "TKT-1", "topics", CreateInput{Title: "a"})
	require.NoError(t, err)
	h.anchors.calls.Store(0)
	ac, err := h.svc.ListAnchor(as(user("bob", "chat.read")), "ticket", "TKT-1", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, h.anchors.calls.Load())
	require.Len(t, channelsOf(t, ac, "room"), 1)
	require.Len(t, channelsOf(t, ac, "topics"), 1)
	require.Empty(t, channelsOf(t, ac, "threads"))

	ac, err = h.svc.ListAnchor(as(user("bob")), "ticket", "TKT-1", "")
	require.NoError(t, err)
	for _, tc := range ac.Types {
		require.NotEqual(t, "topics", tc.Type, "a type the caller cannot read is not listed")
	}

	_, err = h.svc.ListAnchor(as(user("bob")), "doc", "TKT-1", "")
	require.ErrorIs(t, err, ErrAnchorNotFound)
}
