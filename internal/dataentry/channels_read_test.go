package dataentry

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// Read marker route tests (TKT-2B2AIA). They run the real router over a
// file channel store, as the other channel route tests do.

// readerAlice is alice's reader in these tests: the raw login, no org.
var readerAlice = channels.Reader{User: "alice"}

// roomKey is the key of the room on TKT-1.
var roomKey = channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}

// listRead returns the read of the room in user's channel list of TKT-1.
func (e *channelEnv) listRead(t *testing.T, user string) *v1.ChannelRead {
	t.Helper()
	code, b := e.get(t, user, "/api/v1/tickets/TKT-1/_channels")
	require.Equal(t, http.StatusOK, code, "%s", b)
	for _, tl := range decodeAs[v1.ChannelList](t, b).Types {
		for _, c := range tl.Channels {
			if c.Type == "room" {
				return c.Read
			}
		}
	}
	t.Fatal("no room in the list")
	return nil
}

// headRead returns the read of the head at path as user.
func (e *channelEnv) headRead(t *testing.T, user, path string) *v1.ChannelRead {
	t.Helper()
	code, b := e.get(t, user, path)
	require.Equal(t, http.StatusOK, code, "%s", b)
	return decodeAs[v1.ChannelHead](t, b).Channel.Read
}

// mark PUTs body to path's _read as user and requires 200.
func (e *channelEnv) mark(t *testing.T, user, path, body string) v1.ChannelMarkRead {
	t.Helper()
	resp, b := e.do(t, user, http.MethodPut, path+"/_read", body, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
	return decodeAs[v1.ChannelMarkRead](t, b)
}

// TestChannelRead_ListAndHead pins TKT-2B2AIA AC 7 through the router: a
// post marks the poster's channel read, a reader without a marker gets no
// read, and another user's post raises a marked reader's unread.
func TestChannelRead_ListAndHead(t *testing.T) {
	env := newChannelEnv(t)
	env.postText(t, "alice", roomPath, "one")
	require.Equal(t, &v1.ChannelRead{Seq: 1}, env.listRead(t, "alice"))
	require.Nil(t, env.listRead(t, "bob"), "no marker, no read")
	require.Nil(t, env.headRead(t, "bob", roomPath))

	env.mark(t, "bob", roomPath, "")
	env.postText(t, "alice", roomPath, "two")
	require.Equal(t, &v1.ChannelRead{Seq: 1, Unread: 1}, env.listRead(t, "bob"))
	require.Equal(t, &v1.ChannelRead{Seq: 1, Unread: 1}, env.headRead(t, "bob", roomPath))
	require.Equal(t, &v1.ChannelRead{Seq: 2}, env.listRead(t, "alice"), "the poster's own post is never unread")

	code, b := env.get(t, "bob", roomPath+"/messages")
	require.Equal(t, http.StatusOK, code, "%s", b)
	require.NotContains(t, string(b), `"read"`, "a message page carries no read")
}

// TestChannelMarkRead_Handler pins TKT-2B2AIA AC 8: the answers of
// PUT …/_read.
func TestChannelMarkRead_Handler(t *testing.T) {
	env := newChannelEnv(t)
	env.postText(t, "bob", roomPath, "one")
	env.postText(t, "bob", roomPath, "two")
	other := "/api/v1/tickets/TKT-2/_channels/room"
	env.postText(t, "bob", other, "one")

	t.Run("ok", func(t *testing.T) {
		for _, tc := range []struct {
			name, user, path, body string
			want                   v1.ChannelMarkRead
		}{
			{"to the head", "alice", roomPath, "{}", v1.ChannelMarkRead{Seq: 2, LastSeq: 2}},
			{"a seq", "carol", roomPath, `{"seq":1}`, v1.ChannelMarkRead{Seq: 1, Unread: 1, LastSeq: 2}},
			{"clamped", "dave", roomPath, `{"seq":99}`, v1.ChannelMarkRead{Seq: 2, LastSeq: 2}},
			{"empty body", "alice", other, "", v1.ChannelMarkRead{Seq: 1, LastSeq: 1}},
			{"unstored", "alice", "/api/v1/tickets/TKT-2/_channels/chat", "", v1.ChannelMarkRead{}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.Equal(t, tc.want, env.mark(t, tc.user, tc.path, tc.body))
			})
		}
		st, err := env.ch.store.Reads(context.Background(), readerAlice, []channels.Key{roomKey})
		require.NoError(t, err)
		require.Equal(t, int64(2), st[roomKey].Seq, "alice's reader is the raw login")
		_, err = env.ch.store.Reads(context.Background(), readerAlice,
			[]channels.Key{{Anchor: channels.Anchor{ID: "TKT-2"}, Type: "chat"}})
		require.NoError(t, err)
	})

	t.Run("refused", func(t *testing.T) {
		for _, tc := range []struct {
			name, user, method, path, body string
			code                           int
			ptype                          string
		}{
			{"negative", "bob", http.MethodPut, roomPath, `{"seq":-1}`, http.StatusBadRequest, realtime.TypeInvalid},
			{"string", "bob", http.MethodPut, roomPath, `{"seq":"x"}`, http.StatusBadRequest, "invalid_request"},
			{"fraction", "bob", http.MethodPut, roomPath, `{"seq":1.5}`, http.StatusBadRequest, "invalid_request"},
			{"unknown field", "bob", http.MethodPut, roomPath, `{"foo":1}`, http.StatusBadRequest, "invalid_request"},
			{"trailing", "bob", http.MethodPut, roomPath, `{} {}`, http.StatusBadRequest, "invalid_request"},
			{"read denied", "alice", http.MethodPut, "/api/v1/tickets/TKT-1/_channels/secret", "",
				http.StatusForbidden, realtime.TypeReadDenied},
			{"GET", "bob", http.MethodGet, roomPath, "", http.StatusMethodNotAllowed, "method_not_allowed"},
			{"POST", "bob", http.MethodPost, roomPath, "", http.StatusMethodNotAllowed, "method_not_allowed"},
			{"PATCH", "bob", http.MethodPatch, roomPath, "", http.StatusMethodNotAllowed, "method_not_allowed"},
			{"DELETE", "bob", http.MethodDelete, roomPath, "", http.StatusMethodNotAllowed, "method_not_allowed"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp, b := env.do(t, tc.user, tc.method, tc.path+"/_read", tc.body, nil)
				require.Equal(t, tc.code, resp.StatusCode, "%s", b)
				require.Equal(t, tc.ptype, problemType(t, b))
				if tc.code == http.StatusMethodNotAllowed {
					require.Equal(t, http.MethodPut, resp.Header.Get("Allow"))
				}
			})
		}
		secret := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "secret"}
		st, err := env.ch.store.Reads(context.Background(), readerAlice, []channels.Key{secret})
		require.NoError(t, err)
		require.NotContains(t, st, secret, "a refused mark stores nothing")
	})

	t.Run("hidden anchor", func(t *testing.T) {
		env.postText(t, "dave", "/api/v1/features/FEAT-1/_channels/room", "hidden")
		code, want := env.get(t, "alice", "/api/v1/features/FEAT-1")
		resp, b := env.do(t, "alice", http.MethodPut, "/api/v1/features/FEAT-1/_channels/room/_read", "", nil)
		requireUniform404(t, resp.StatusCode, b, code, want)
		k := channels.Key{Anchor: channels.Anchor{ID: "FEAT-1"}, Type: "room"}
		st, err := env.ch.store.Reads(context.Background(), readerAlice, []channels.Key{k})
		require.NoError(t, err)
		require.NotContains(t, st, k, "a mark on a hidden anchor stores nothing")
	})

	t.Run("rate limited", func(t *testing.T) {
		for range 2 {
			env.mark(t, "bob", roomPath, "")
		}
		resp, b := env.do(t, "bob", http.MethodPut, roomPath+"/_read", "", nil)
		require.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "%s", b)
		require.NotEmpty(t, resp.Header.Get("Retry-After"))
		env.postText(t, "bob", roomPath, "marks do not use up the post budget")
	})
}

// TestLive_MarkReadFramesHead pins TKT-2B2AIA AC 14: a moved marker frames
// the reader's own live head and replays nobody else's; a mark that does
// not move frames nothing. Since TKT-9YRRM5 every stored mark wakes the
// reader's own subscriptions, so the unmoved mark replays alice's head,
// with the same hash.
func TestLive_MarkReadFramesHead(t *testing.T) {
	env := newChannelEnv(t)
	env.postText(t, "carol", roomPath, "one")
	a, aConn := env.open(t, "alice")
	b, bConn := env.open(t, "bob")
	env.subscribe(t, "alice", aConn, liveSubEntry{"head", roomPath})
	env.subscribe(t, "bob", bConn, liveSubEntry{"head", roomPath})
	liveQuiet(t, a)
	liveQuiet(t, b)
	replays := countReplaysByURL(t, env.app.live)

	env.mark(t, "alice", roomPath, "")
	requireChanged(t, a, "head")
	liveQuiet(t, b)
	if got, want := replays.take(), map[string]int{roomPath: 1}; !maps.Equal(got, want) {
		t.Fatalf("a mark replayed %v, want only alice's head %v", got, want)
	}

	env.mark(t, "alice", roomPath, `{"seq":1}`)
	liveQuiet(t, a)
	liveQuiet(t, b)
	if got, want := replays.take(), map[string]int{roomPath: 1}; !maps.Equal(got, want) {
		t.Fatalf("a mark that does not move replayed %v, want only alice's head %v", got, want)
	}
}

// TestRoundFilter_SelectsReader pins that a read change selects only the
// subscriptions of its reader on the channel's topics.
func TestRoundFilter_SelectsReader(t *testing.T) {
	f := newRoundFilter(nil, nil, false)
	f.addReads(map[channels.Reader][]channels.Key{readerAlice: {roomKey}})
	head := channelTopic("TKT-1", "room", "")
	require.True(t, f.selects(&liveSub{channel: true, topic: head, reader: readerAlice}))
	require.True(t, f.selects(&liveSub{channel: true, topic: anchorTopic("TKT-1"), reader: readerAlice}))
	require.False(t, f.selects(&liveSub{channel: true, topic: head, reader: channels.Reader{User: "bob"}}))
	require.False(t, f.selects(&liveSub{channel: true, topic: head}))
	require.False(t, f.selects(&liveSub{channel: true, topic: channelTopic("TKT-2", "room", ""), reader: readerAlice}))
}

// markAndChat gives alice a marker at seq 1 on the case channel of id and
// has bob post once more.
func (e *inboxEnv) markAndChat(t *testing.T, id string, at time.Time) {
	t.Helper()
	k := channels.Key{Anchor: channels.Anchor{ID: id}, Type: "case"}
	_, _, err := e.ch.store.SetRead(context.Background(), k, readerAlice, 1, at)
	require.NoError(t, err)
	e.waitOn(t, id, at.Add(time.Second))
}

// TestInboxUnread_ACL pins TKT-2B2AIA AC 9 through the deployed wiring:
// markers with unread messages on channels alice cannot read (a feature,
// a ticket without her officer relation) change nothing in the answer.
func TestInboxUnread_ACL(t *testing.T) {
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	build := func(hidden bool) *inboxEnv {
		e := newInboxEnv(t, func(app *App) {
			owned := func(id, typ string) *entity.Entity {
				return &entity.Entity{ID: id, Type: typ, Properties: map[string]any{"title": id, "owner": "alice"}}
			}
			seedEntity(app, owned("TKT-100", "ticket"))
			seedRelation(app, entity.NewRelation("alice", "officer-of", "TKT-100"))
			if hidden {
				seedEntity(app, owned("TKT-500", "ticket"))
				seedEntity(app, owned("FEAT-100", "feature"))
			}
		})
		e.waitOn(t, "TKT-100", at)
		e.markAndChat(t, "TKT-100", at)
		if hidden {
			for _, id := range []string{"TKT-500", "FEAT-100"} {
				e.waitOn(t, id, at)
				e.markAndChat(t, id, at.Add(time.Minute))
			}
		}
		return e
	}
	plain, mixed := build(false), build(true)
	for _, q := range []string{"", "?kinds=unread"} {
		base, _ := plain.get(t, http.MethodGet, q)
		withHidden, _ := mixed.get(t, http.MethodGet, q)
		require.Equal(t, http.StatusOK, base.Code, base.Body.String())
		require.JSONEq(t, base.Body.String(), withHidden.Body.String(), "query %q", q)
		got := decodeAs[v1.Inbox](t, base.Body.Bytes())
		require.NotNil(t, got.UnreadCount)
		require.Equal(t, 1, *got.UnreadCount)
		require.Equal(t, "TKT-100", got.UnreadChannels[0].Channel.Anchor)
		require.Equal(t, "/api/v1/tickets/TKT-100/_channels/case", got.UnreadChannels[0].Links.Channel)
	}
}

// TestInboxHandler_Kinds pins TKT-2B2AIA AC 11 on the route: the kinds
// query and its refusals.
func TestInboxHandler_Kinds(t *testing.T) {
	e, _ := seedInbox(t, 2)
	for _, tc := range []struct {
		name, query string
		code        int
		ptype       string
	}{
		{"unknown", "?kinds=bogus", http.StatusBadRequest, realtime.TypeInvalid},
		{"cursor with mention only", "?kinds=mention&cursor=abc", http.StatusBadRequest, realtime.TypeInvalidCursor},
		{"empty", "?kinds=", http.StatusBadRequest, realtime.TypeInvalid},
		{"cursor with unread only", "?kinds=unread&cursor=abc", http.StatusBadRequest, realtime.TypeInvalidCursor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := e.get(t, http.MethodGet, tc.query)
			require.Equal(t, tc.code, rec.Code, rec.Body.String())
			require.Equal(t, tc.ptype, problemType(t, rec.Body.Bytes()))
		})
	}

	rec, _ := e.get(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"unread_count":0`)
	require.Contains(t, rec.Body.String(), `"unread_channels":[]`)
	require.Contains(t, rec.Body.String(), `"mention_count":0`)
	require.Contains(t, rec.Body.String(), `"mentions":[]`)

	rec, _ = e.get(t, http.MethodGet, "?kinds=mention,reply")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"mention_count":0`)
	require.NotContains(t, rec.Body.String(), "unread")
	require.NotContains(t, rec.Body.String(), `"entries"`)

	rec, _ = e.get(t, http.MethodGet, "?kinds=waiting")
	require.NotContains(t, rec.Body.String(), "unread")
	require.NotContains(t, rec.Body.String(), "mention")

	rec, _ = e.get(t, http.MethodGet, "?kinds=unread")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, f := range []string{`"entries"`, `"count"`, `"truncated"`, `"next_cursor"`} {
		require.NotContains(t, rec.Body.String(), f)
	}
}

// TestChannelsBudget_HeadRead pins TKT-2B2AIA AC 10: a head with a marker
// makes one Reads beside its Head and Get, at 10 messages as at 50.
func TestChannelsBudget_HeadRead(t *testing.T) {
	cost := func(n int) channelCost {
		b := newChannelBudgetWith(t, 0, channelTestRealtime)
		for i := range n {
			appendText(t, b, "bob", i)
		}
		_, _, err := b.ch.store.SetRead(context.Background(), roomKey, readerAlice, 1, time.Now())
		require.NoError(t, err)
		got, body := b.get(t, roomPath)
		require.Equal(t, &v1.ChannelRead{Seq: 1, Unread: n - 1}, decodeAs[v1.ChannelHead](t, body).Channel.Read)
		return got
	}
	small, large := cost(10), cost(50)
	require.True(t, small.equal(large), "10: %s\n50: %s", small, large)
	require.Equal(t, 1, small.channel["Reads"], small.String())
	require.Equal(t, 1, small.channel["Head"], small.String())
	require.Equal(t, 1, small.channel["Get"], small.String())
}

// TestChannelsBudget_MarkRead pins TKT-2B2AIA AC 10: a mark makes one Head
// and one SetRead, at 10 messages as at 50.
func TestChannelsBudget_MarkRead(t *testing.T) {
	cost := func(n int) channelCost {
		b := newChannelBudgetWith(t, 0, channelTestRealtime)
		for i := range n {
			appendText(t, b, "bob", i)
		}
		b.counting.Reset()
		b.chans.reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, roomPath+"/_read", strings.NewReader("{}"))
		b.app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, b.d)))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, v1.ChannelMarkRead{Seq: int64(n), LastSeq: int64(n)},
			decodeAs[v1.ChannelMarkRead](t, rec.Body.Bytes()))
		return channelCost{entity: b.counting.Calls(), channel: b.chans.snapshot()}
	}
	small, large := cost(10), cost(50)
	require.True(t, small.equal(large), "10: %s\n50: %s", small, large)
	require.Equal(t, map[string]int{"Head": 1, "SetRead": 1}, small.channel, small.String())
}

// appendText appends a text message by user to the room, past the
// service.
func appendText(t *testing.T, b *channelBudget, user string, i int) {
	t.Helper()
	_, err := b.ch.store.Append(context.Background(), roomKey, channels.NewMessage{
		Type: "text", Author: channels.Author{User: user, Tool: principal.ToolDataEntry}, At: time.Now(),
		Props: map[string]any{"body": fmt.Sprint("m", i)},
	})
	require.NoError(t, err)
}
