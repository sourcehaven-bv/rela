package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// mentionRealtime is a config for the mention tests: a room with a
// markdown text, a plain string, refs that notify and one that does not,
// and a secret type whose read needs a permission.
const mentionRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:    { properties: { body: { type: markdown } }, post: [member] }
      plain:   { properties: { body: { type: string } }, post: [member] }
      handoff: { properties: { to: { type: ref, to: [person] } }, post: [member] }
      quiet:   { properties: { to: { type: ref, to: [person], notify: false } }, post: [member] }
      about:   { properties: { on: { type: ref, to: [ticket] } }, post: [member] }
      ping:    { persist: ephemeral, ttl: 5s, properties: { body: { type: markdown } }, post: [member] }
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member] }
  secret:
    attach_to: [ticket]
    cardinality: one
    read: { permission: chat.secret }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown } }, post: [member] }
`

var secretRoom = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "secret"}

// newMentionHarness is a harness over mentionRealtime. With mapping set,
// acl.yaml maps logins to person entities, and USR-1 to USR-12 are
// readable persons.
func newMentionHarness(t *testing.T, mapping bool) *harness {
	t.Helper()
	opts := []harnessOpt{withYAML(mentionRealtime)}
	if mapping {
		opts = append(opts, withPersonMapping())
	}
	h := newHarness(t, opts...)
	h.refs.mu.Lock()
	for i := 1; i <= 12; i++ {
		id := fmt.Sprintf("USR-%d", i)
		h.refs.views[id] = RefView{ID: id, Type: "person", Title: "Person " + id}
	}
	// The resolver matches ids case-insensitively, as the entity store does.
	h.refs.views["usr-7"] = RefView{ID: "USR-7", Type: "person", Title: "Person USR-7"}
	h.refs.mu.Unlock()
	return h
}

func md(body string) PostInput { return PostInput{Type: "text", Props: map[string]any{"body": body}} }

// storedMentions reads the stored targets of message seq in the room.
func storedMentions(t *testing.T, h *harness, seq int64) []channels.Mention {
	t.Helper()
	m, err := h.store.Get(context.Background(), roomKey(), seq)
	require.NoError(t, err)
	return m.Mentions
}

// spans is n code spans `USR-1` to `USR-n`.
func spans(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "`USR-%d` ", i)
	}
	return b.String()
}

// TestClassifySpans pins classification (TKT-6RXAC0 AC 2).
func TestClassifySpans(t *testing.T) {
	t.Parallel()
	job := func(s string) channels.Mention { return channels.Mention{Target: s, Form: channels.FormJob} }
	login := func(s string) channels.Mention {
		return channels.Mention{Target: s, Form: channels.FormLogin, Span: "@" + s}
	}
	for _, tc := range []struct {
		name     string
		spans    []string
		userType string
		users    []string
		others   []channels.Mention
	}{
		{"id with mapping on", []string{"USR-7"}, "person", []string{"USR-7"}, nil},
		{"id with mapping off", []string{"USR-7"}, "", nil, nil},
		{"login with mapping off", []string{"@jeroen"}, "", nil, []channels.Mention{login("jeroen")}},
		{"login with mapping on is plain", []string{"@jeroen"}, "person", nil, nil},
		{"job either way", []string{"integration:triage-bot", "system:ops"}, "person", nil,
			[]channels.Mention{job("integration:triage-bot"), job("system:ops")}},
		{"job with mapping off", []string{"integration:triage-bot"}, "", nil,
			[]channels.Mention{job("integration:triage-bot")}},
		{"rela identity is no job", []string{"system:realtime"}, "", nil, nil},
		{"bad job name", []string{"integration:Bad Name"}, "", nil, nil},
		{"@system: is no login", []string{"@system:x", "@integration:x"}, "", nil, nil},
		{"@unknown", []string{"@unknown"}, "", nil, nil},
		{"zero width space", []string{"@jeroen\u200b"}, "", nil, nil},
		{"bidi mark", []string{"@jer\u202eoen"}, "", nil, nil},
		{"not NFC", []string{"@joél"}, "", nil, nil},
		{"NFC accent", []string{"@joél"}, "", nil, []channels.Mention{login("joél")}},
		{"empty login", []string{"@"}, "", nil, nil},
		{"space in login", []string{"@a b"}, "", nil, nil},
		{"case is kept", []string{"@Jeroen"}, "", nil, []channels.Mention{login("Jeroen")}},
		{"plain code", []string{"go test", "x := 1"}, "person", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			users, others := classifySpans(tc.spans, tc.userType)
			require.Equal(t, tc.users, users)
			require.Equal(t, tc.others, others)
		})
	}

	t.Run("64 candidates", func(t *testing.T) {
		t.Parallel()
		var ss []string
		for i := 1; i <= 70; i++ {
			ss = append(ss, fmt.Sprintf("USR-%d", i))
		}
		users, _ := classifySpans(ss, "person")
		require.Len(t, users, maxCandidateSpans)
		require.Equal(t, "USR-64", users[63], "document order is kept")
	})
}

// TestPost_MentionTargets pins post resolution (AC 3): user candidates
// resolve in the one Resolve batch and are stored as the canonical id; a
// job target as written; the poster's own key is dropped; a string
// property is never parsed; a signal addresses no one.
func TestPost_MentionTargets(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	h.refs.calls.Store(0)
	v := h.post(t, user("USR-3"),
		md("`usr-7` look `integration:triage-bot` and `@bob` and `USR-3` and `TKT-2` and `USR-99`"))
	require.Equal(t, int64(1), h.refs.calls.Load(), "one Resolve for refs and mentions together")
	require.Equal(t, []channels.Mention{
		{Target: "USR-7", Form: channels.FormUser, Span: "usr-7"},
		{Target: "integration:triage-bot", Form: channels.FormJob},
	}, storedMentions(t, h, v.Seq))
	require.Equal(t, []MentionView{
		{Text: "usr-7", Kind: "user", Ref: &RefValue{ID: "USR-7", Type: "person", Title: "Person USR-7"}},
		{Text: "integration:triage-bot", Kind: "job"},
	}, v.Mentions)

	v = h.post(t, user("USR-3"), PostInput{Type: "plain", Props: map[string]any{"body": "`USR-7`"}})
	require.Nil(t, storedMentions(t, h, v.Seq), "a string property is never parsed")

	res, err := h.svc.Post(as(user("USR-3")), room, PostInput{Type: "ping", Props: map[string]any{
		"body": spans(11),
	}})
	require.NoError(t, err, "a signal addresses no one, so the cap does not apply")
	require.True(t, res.Ephemeral)
}

// TestPost_LoginMentions pins the login form where person mapping is off.
func TestPost_LoginMentions(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	v := h.post(t, user("carol"), md("`@bob` `USR-7` `@carol`"))
	require.Equal(t, []channels.Mention{{Target: "bob", Form: channels.FormLogin, Span: "@bob"}},
		storedMentions(t, h, v.Seq), "an id is plain code and the poster's own login is dropped")
	require.Equal(t, []MentionView{{Text: "@bob", Kind: "login"}}, v.Mentions)
}

// stripVolatile drops the fields of a message JSON that differ between two
// posts: seq, rev and time.
func stripVolatile(t *testing.T, v MessageView) string {
	t.Helper()
	v.Seq, v.Rev, v.At, v.EditUntil = 0, 0, time.Time{}, time.Time{}
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// TestPost_MentionAnswerIsUniform pins AC 3's two equalities: a hidden
// user, a missing entity and a non-user entity give equal answers (no
// chip); two readable users give equal answers (a chip), whatever their
// own access.
func TestPost_MentionAnswerIsUniform(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	poster := user("USR-3")
	h.refs.hide = map[string][]string{"USR-3": {"USR-9"}}
	var none []string
	for _, id := range []string{"USR-9", "USR-404", "TKT-2"} {
		v := h.post(t, poster, md("`"+id+"`"))
		require.Nil(t, v.Mentions, id)
		require.Nil(t, storedMentions(t, h, v.Seq), id)
		none = append(none, strings.Replace(stripVolatile(t, v), id, "ID", 1))
	}
	require.Equal(t, none[0], none[1])
	require.Equal(t, none[0], none[2])

	a := h.post(t, poster, md("`USR-7`"))
	b := h.post(t, poster, md("`USR-8`"))
	require.Len(t, a.Mentions, 1)
	require.Equal(t, strings.ReplaceAll(stripVolatile(t, a), "USR-7", "ID"),
		strings.ReplaceAll(stripVolatile(t, b), "USR-8", "ID"))
}

// TestPost_TooManyMentions pins the cap (AC 3): 11 readable users are a
// 422 and store nothing; 10 are stored; 11 spans of which one is hidden
// are stored, because the hidden one is no target.
func TestPost_TooManyMentions(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	poster := user("USR-12")
	_, err := h.svc.Post(as(poster), room, md(spans(11)))
	requireDenial(t, err, http.StatusUnprocessableEntity, TypeTooManyMentions)
	require.Zero(t, h.store.appends.Load(), "nothing is stored")

	v := h.post(t, poster, md(spans(10)))
	require.Len(t, storedMentions(t, h, v.Seq), 10)

	h.refs.hide = map[string][]string{"USR-12": {"USR-11"}}
	v = h.post(t, poster, md(spans(11)))
	require.Len(t, storedMentions(t, h, v.Seq), 10)
}

// TestPost_RefPropMention pins ref properties as mentions (AC 3): a ref
// to the user type is a target unless the property says notify: false; a
// ref to another type is not.
func TestPost_RefPropMention(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	poster := user("USR-3")
	v := h.post(t, poster, PostInput{Type: "handoff", Props: map[string]any{"to": "USR-7"}})
	require.Equal(t, []channels.Mention{{Target: "USR-7", Form: channels.FormUser}}, storedMentions(t, h, v.Seq))
	v = h.post(t, poster, PostInput{Type: "quiet", Props: map[string]any{"to": "USR-7"}})
	require.Nil(t, storedMentions(t, h, v.Seq))
	v = h.post(t, poster, PostInput{Type: "about", Props: map[string]any{"on": "TKT-2"}})
	require.Nil(t, storedMentions(t, h, v.Seq))
}

func (h *harness) edit(t *testing.T, a *fakeAccess, seq int64, body string) MessageView {
	t.Helper()
	v, err := h.svc.Edit(as(a), room, seq, map[string]any{"body": body})
	require.NoError(t, err)
	return v
}

func targetsOf(ms []channels.Mention) []string { return channels.MentionTargets(ms) }

// TestEdit_KeepsUnreadableMention pins AC 6: an author who lost read
// access to USR-7 edits another word, and the target stays.
func TestEdit_KeepsUnreadableMention(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	author := user("USR-3")
	v := h.post(t, author, md("`USR-7` please look"))
	h.refs.hide = map[string][]string{"USR-3": {"USR-7"}}
	got := h.edit(t, author, v.Seq, "`USR-7` please look now")
	require.Equal(t, []string{"USR-7"}, targetsOf(storedMentions(t, h, v.Seq)))
	require.Nil(t, got.Mentions, "the editor still sees no chip for a user they cannot read")
}

// TestEdit_DropsRemovedMention pins that an edit without the span drops
// the target.
func TestEdit_DropsRemovedMention(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	author := user("USR-3")
	v := h.post(t, author, md("`USR-7` and `USR-8`"))
	h.edit(t, author, v.Seq, "`USR-8` only")
	require.Equal(t, []string{"USR-8"}, targetsOf(storedMentions(t, h, v.Seq)))
	h.edit(t, author, v.Seq, "nobody")
	require.Nil(t, storedMentions(t, h, v.Seq))
}

// TestEdit_AddedMentionWakesTarget pins that an edit wakes only the
// targets it added, on an addressed subscription.
func TestEdit_AddedMentionWakesTarget(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	author := user("USR-3")
	v := h.post(t, author, md("`USR-7`"))
	sub := h.svc.SubscribeAddressed()
	defer sub.Close()
	plain := h.svc.Subscribe()
	defer plain.Close()
	h.edit(t, author, v.Seq, "`USR-7` and `USR-8`")
	require.Equal(t, []string{"USR-8"}, sub.TakeAddressed())
	require.Nil(t, plain.TakeAddressed(), "a plain subscription takes no addressed keys")
	require.Equal(t, []channels.Key{roomKey()}, takeKeys(t, plain))
}

// TestEdit_TooManyWithKeptHidden pins that the cap counts kept hidden
// targets (AC 6): 9 visible plus 2 kept hidden is a 422; 8 plus 2 is
// stored.
func TestEdit_TooManyWithKeptHidden(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	author := user("USR-12")
	v := h.post(t, author, md(spans(10)))
	h.refs.hide = map[string][]string{"USR-12": {"USR-1", "USR-2"}}
	_, err := h.svc.Edit(as(author), room, v.Seq, map[string]any{"body": spans(11)})
	requireDenial(t, err, http.StatusUnprocessableEntity, TypeTooManyMentions)
	require.Len(t, storedMentions(t, h, v.Seq), 10, "the refused edit changed nothing")
	h.edit(t, author, v.Seq, spans(10))
	require.Len(t, storedMentions(t, h, v.Seq), 10)
}

// TestPost_ReplyWakesParentAuthor pins the reply wake: a reply wakes the
// parent author's key on an addressed subscription.
func TestPost_ReplyWakesParentAuthor(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	parent := h.post(t, user("alice"), md("question"))
	sub := h.svc.SubscribeAddressed()
	defer sub.Close()
	in := md("answer `@carol`")
	in.ReplyTo = parent.Seq
	h.post(t, user("bob"), in)
	require.Equal(t, []string{"alice", "carol"}, sub.TakeAddressed())
}

// TestRender_Mentions pins rendering (AC 14): a reader who may read USR-7
// sees its chip with the title; one who may not sees no entry; the stored
// form wins over the current configuration.
func TestRender_Mentions(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	h.post(t, user("carol"), md("`@jeroen` `integration:triage-bot`"))
	h.cfg.set(Snapshot{Config: testCompiledWith(t, mentionRealtime, personMappingPolicy())})
	h.post(t, user("carol"), md("`USR-7` hello"))

	all := h.readAll(t, user("bob"), room)
	require.Equal(t, []MentionView{{Text: "integration:triage-bot", Kind: "job"}, {Text: "@jeroen", Kind: "login"}},
		all.Messages[0].Mentions, "an old login target stays a login after mapping is switched on")
	require.Equal(t, []MentionView{
		{Text: "USR-7", Kind: "user", Ref: &RefValue{ID: "USR-7", Type: "person", Title: "Person USR-7"}},
	}, all.Messages[1].Mentions)

	h.refs.hide = map[string][]string{"dave": {"USR-7"}}
	hidden := h.readAll(t, user("dave"), room)
	require.Nil(t, hidden.Messages[1].Mentions, "a user the reader cannot read is absent")
	require.NotContains(t, mustJSON(t, hidden.Messages[1]), "USR-7\",\"kind")

	_, err := h.svc.Delete(as(user("carol")), room, 2, "")
	require.NoError(t, err)
	require.Nil(t, h.readAll(t, user("bob"), room).Messages[1].Mentions, "a removed message has none")
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// TestRender_MessagesOneResolve pins that a page resolves its refs and
// mentions in one batch (AC 13).
func TestRender_MessagesOneResolve(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, true)
	for i := range 5 {
		h.post(t, user("USR-12"), md(fmt.Sprintf("`USR-%d` `USR-%d`", i+1, i+2)))
	}
	h.refs.calls.Store(0)
	v := h.readAll(t, user("bob"), room)
	require.Len(t, v.Messages, 5)
	require.Equal(t, int64(1), h.refs.calls.Load())
}

// TestLocal_AddressedOptIn pins that only an addressed subscription takes
// addressed keys (AC 16): a plain subscription is unaffected by 3000
// hints, and a resync clears the addressed set.
func TestLocal_AddressedOptIn(t *testing.T) {
	t.Parallel()
	clock := &testClock{now: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}
	l, err := newLocal(clock.Now, defaultEphemeralLimits(), defaultLimitCaps())
	require.NoError(t, err)
	t.Cleanup(l.Stop)
	plain, addr := l.Subscribe(), l.SubscribeAddressed()
	k := roomKey()
	for i := range 3000 {
		l.markAddressed([]string{fmt.Sprint("user-", i)}, k)
	}
	keys, resync := plain.Take()
	require.False(t, resync, "a plain subscription never resyncs from addressed keys")
	require.Equal(t, []channels.Key{k}, keys)
	require.Nil(t, plain.TakeAddressed())

	require.Nil(t, addr.TakeAddressed(), "the overflow dropped the keys")
	_, resync = addr.Take()
	require.True(t, resync)

	l.markAddressed([]string{"bob", "alice"}, k)
	require.Equal(t, []string{"alice", "bob"}, addr.TakeAddressed())
	require.Nil(t, addr.TakeAddressed(), "TakeAddressed empties the set")
	l.markAddressed([]string{"bob"}, k)
	addr.mu.Lock()
	addr.resync = true
	addr.mu.Unlock()
	_, resync = addr.Take()
	require.True(t, resync)
	require.Nil(t, addr.TakeAddressed(), "a resync clears the addressed set")
}

// TestLocal_SubscribeWhileMarking pins that subscribing with options while
// other goroutines mark addressed keys and read markers is free of data
// races. Run it with -race.
func TestLocal_SubscribeWhileMarking(t *testing.T) {
	t.Parallel()
	clock := &testClock{now: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}
	l, err := newLocal(clock.Now, defaultEphemeralLimits(), defaultLimitCaps())
	require.NoError(t, err)
	t.Cleanup(l.Stop)
	k := roomKey()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				l.markAddressed([]string{"alice"}, k)
				l.markFor(channels.Reader{User: "alice"}, k)
			}
		}
	})
	for range 200 {
		l.SubscribeAddressed().Close()
		l.SubscribeReads().Close()
	}
	close(stop)
	wg.Wait()
	addr, reads := l.SubscribeAddressed(), l.SubscribeReads()
	l.markAddressed([]string{"alice"}, k)
	l.markFor(channels.Reader{User: "alice"}, k)
	require.Equal(t, []string{"alice"}, addr.TakeAddressed())
	require.Equal(t, map[channels.Reader][]channels.Key{{User: "alice"}: {k}}, reads.TakeReads())
}
