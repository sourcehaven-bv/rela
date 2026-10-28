package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// summary serves /api/v1/_inbox/summary?query as alice and returns what
// it cost.
func (e *inboxEnv) summary(t *testing.T, method, query string) (rec *httptest.ResponseRecorder, cost channelCost) {
	t.Helper()
	e.counting.Reset()
	e.chans.reset()
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(method, inboxSummaryPath+query, http.NoBody)
	e.app.channels.serveInboxSummary(rec, req.WithContext(gateCtxFor(principalCtx("alice"), t, e.d)))
	return rec, channelCost{entity: e.counting.Calls(), channel: e.chans.snapshot()}
}

// TestInboxSummaryHandler_Shape pins AC 1: the summary carries the counts
// of the default inbox answer, and ignores its query string.
func TestInboxSummaryHandler_Shape(t *testing.T) {
	ids := []string{"TKT-100", "TKT-101", "TKT-102"}
	e := newInboxEnv(t, func(app *App) {
		for _, id := range ids {
			seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id, "owner": "alice"}})
			seedRelation(app, entity.NewRelation("alice", "officer-of", id))
		}
	})
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	for _, id := range ids {
		e.waitOn(t, id, at)
	}
	e.markAndChat(t, "TKT-100", at)
	e.markAndChat(t, "TKT-101", at)
	e.mentionAlice(t, "TKT-102", at)

	inbox, _ := e.get(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, inbox.Code, inbox.Body.String())
	full := decodeAs[v1.Inbox](t, inbox.Body.Bytes())

	rec, _ := e.summary(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.ElementsMatch(t, []string{"waiting", "waiting_truncated", "unread_channels", "mentions", "degraded"},
		jsonKeys(raw))
	got := decodeAs[v1.InboxSummary](t, rec.Body.Bytes())
	require.Equal(t, v1.InboxSummary{
		Waiting: *full.Count, WaitingTruncated: *full.Truncated, UnreadChannels: *full.UnreadCount,
		Mentions: *full.MentionCount, Degraded: full.Degraded,
	}, got)
	require.Equal(t, v1.InboxSummary{Waiting: 3, UnreadChannels: 2, Mentions: 1}, got)

	ignored, _ := e.summary(t, http.MethodGet, "?limit=abc")
	require.Equal(t, http.StatusOK, ignored.Code, ignored.Body.String())
	require.Equal(t, rec.Body.String(), ignored.Body.String())

	t.Run("routed", func(t *testing.T) {
		env := newChannelEnv(t)
		resp, body := env.do(t, "alice", http.MethodGet, inboxSummaryPath, "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		require.Equal(t, v1.InboxSummary{}, decodeAs[v1.InboxSummary](t, body))
	})
}

// TestInboxSummaryHandler_Errors pins AC 2: the summary refuses like the
// inbox.
//
//nolint:paralleltest // the subtests share helpers only
func TestInboxSummaryHandler_Errors(t *testing.T) {
	serve := func(h *channelsHandler, method string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.serveInboxSummary(rec, httptest.NewRequest(method, inboxSummaryPath, http.NoBody))
		return rec
	}
	requireProblem := func(t *testing.T, rec *httptest.ResponseRecorder, status int, typ string) v1.Error {
		t.Helper()
		require.Equal(t, status, rec.Code, rec.Body.String())
		require.Equal(t, typ, problemType(t, rec.Body.Bytes()))
		return decodeAs[v1.Error](t, rec.Body.Bytes())
	}

	t.Run("not wired", func(t *testing.T) {
		p := requireProblem(t, serve(nil, http.MethodGet), http.StatusNotFound, "not_found")
		require.Equal(t, realtimeOffTitle, p.Title)
		plain := newLiveEnv(t, nil)
		resp, body := plain.do(t, "alice", http.MethodGet, inboxSummaryPath, "", nil)
		require.Equal(t, http.StatusNotFound, resp.StatusCode, string(body))
	})
	t.Run("off", func(t *testing.T) {
		e := newInboxEnv(t, nil)
		e.ch.cfg.Store(nil)
		rec, _ := e.summary(t, http.MethodGet, "")
		p := requireProblem(t, rec, http.StatusNotFound, "not_found")
		require.Equal(t, realtimeOffTitle, p.Title)
		h := &channelsHandler{on: stubChannels{}, inbox: stubInbox{err: realtime.ErrRealtimeOff}}
		p = requireProblem(t, serve(h, http.MethodGet), http.StatusNotFound, "not_found")
		require.Equal(t, realtimeOffTitle, p.Title)
	})
	t.Run("method", func(t *testing.T) {
		e := newInboxEnv(t, nil)
		rec, _ := e.summary(t, http.MethodPost, "")
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		require.Equal(t, http.MethodGet, rec.Header().Get("Allow"))
	})
	t.Run("no principal", func(t *testing.T) {
		h := &channelsHandler{on: stubChannels{}, inbox: stubInbox{err: realtime.ErrNoPrincipal}}
		requireProblem(t, serve(h, http.MethodGet), http.StatusForbidden, realtime.TypeNoPrincipal)
		e := newInboxEnv(t, nil)
		rec := httptest.NewRecorder()
		e.app.channels.serveInboxSummary(rec, httptest.NewRequest(http.MethodGet, inboxSummaryPath, http.NoBody))
		requireProblem(t, rec, http.StatusForbidden, realtime.TypeNoPrincipal)
	})
	t.Run("row ceiling", func(t *testing.T) {
		capped, _ := seedInbox(t, 2)
		capped.chans.mu.Lock()
		capped.chans.maxRows = 2
		capped.chans.mu.Unlock()
		inbox, _ := capped.get(t, http.MethodGet, "")
		rec, _ := capped.summary(t, http.MethodGet, "")
		requireProblem(t, rec, http.StatusServiceUnavailable, realtime.TypeInboxTooLarge)
		require.Equal(t, inbox.Code, rec.Code)
	})
	t.Run("store fault", func(t *testing.T) {
		h := &channelsHandler{on: stubChannels{}, inbox: stubInbox{err: errors.New("disk on fire")}}
		rec := serve(h, http.MethodGet)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "disk on fire")
	})
}

// TestInboxSummary_HiddenRowsChangeNothing pins AC 3 through the deployed
// wiring: waiting sessions, unread channels and mentions on rows alice
// cannot read leave the summary byte for byte the same.
func TestInboxSummary_HiddenRowsChangeNothing(t *testing.T) {
	policy := inboxPolicy()
	editor := policy.Roles["editor"]
	editor.Visible = map[string][]acl.FieldGrant{"ticket": {
		{Field: "title"}, {Field: "watchers"}, {Field: "owner", When: "entity.title == 'shown'"},
	}}
	policy.Roles["editor"] = editor
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	build := func(hidden bool) *inboxEnv {
		e := newInboxEnvPolicy(t, func(app *App) {
			for i := range 4 {
				id := fmt.Sprintf("TKT-%d", 100+i)
				seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
					Properties: map[string]any{"title": "shown", "owner": "alice"}})
				seedRelation(app, entity.NewRelation("alice", "officer-of", id))
				if hidden {
					// The owner is hidden from alice: not a waiting holder.
					id = fmt.Sprintf("TKT-%d", 150+i)
					seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
						Properties: map[string]any{"title": "hidden", "owner": "alice"}})
					seedRelation(app, entity.NewRelation("alice", "officer-of", id))
					// No officer relation: the channel is not readable.
					seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%d", 500+i), Type: "ticket",
						Properties: map[string]any{"title": "shown", "owner": "alice"}})
					// Features are not readable at all.
					seedEntity(app, &entity.Entity{ID: fmt.Sprintf("FEAT-%d", 100+i), Type: "feature",
						Properties: map[string]any{"title": "f", "owner": "alice"}})
				}
			}
		}, policy)
		for i := range 4 {
			id := fmt.Sprintf("TKT-%d", 100+i)
			e.waitOn(t, id, at.Add(time.Duration(i)*time.Second))
			if i < 2 {
				e.markAndChat(t, id, at)
			}
			if i == 3 {
				e.mentionAlice(t, id, at)
			}
			if !hidden {
				continue
			}
			// TKT-15x reads, but its owner does not: only the waiting
			// session is hidden.
			e.waitOn(t, fmt.Sprintf("TKT-%d", 150+i), at.Add(time.Duration(i)*time.Second+time.Millisecond))
			for _, h := range []string{fmt.Sprintf("TKT-%d", 500+i), fmt.Sprintf("FEAT-%d", 100+i)} {
				e.waitOn(t, h, at.Add(time.Duration(i)*time.Second+time.Millisecond))
				e.markAndChat(t, h, at)
				e.mentionAlice(t, h, at)
			}
		}
		return e
	}
	plain, mixed := build(false), build(true)
	base, _ := plain.summary(t, http.MethodGet, "")
	withHidden, _ := mixed.summary(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, base.Code, base.Body.String())
	require.Equal(t, base.Body.String(), withHidden.Body.String())
	got := decodeAs[v1.InboxSummary](t, base.Body.Bytes())
	require.Equal(t, 4, got.Waiting)
	require.Equal(t, 1, got.Mentions)
}

// TestInboxSummaryBudget_10vs50 pins AC 4: the summary costs the same
// store calls at 10 rows of each kind as at 50.
func TestInboxSummaryBudget_10vs50(t *testing.T) {
	cost := func(n int) channelCost {
		e, want := seedInbox(t, n)
		at := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
		for i := range n {
			e.markAndChat(t, fmt.Sprintf("TKT-%d", 100+i), at)
			e.markAndChat(t, fmt.Sprintf("FEAT-%d", 100+i), at)
			e.mentionAlice(t, fmt.Sprintf("TKT-%d", 100+i), at)
			e.mentionAlice(t, fmt.Sprintf("TKT-%d", 500+i), at)
		}
		rec, c := e.summary(t, http.MethodGet, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodeAs[v1.InboxSummary](t, rec.Body.Bytes())
		require.Equal(t, v1.InboxSummary{Waiting: want, UnreadChannels: n, Mentions: n}, got)
		return c
	}
	small, large := cost(10), cost(50)
	require.True(t, small.equal(large), "10 rows: %s\n50 rows: %s", small, large)
	require.Equal(t, 1, small.channel["ListWaiting"], small.String())
	require.Equal(t, 1, small.channel["ListReads"], small.String())
	require.Equal(t, 1, small.channel["Reads"], small.String())
	require.Equal(t, 1, small.channel["ListAddressed"], small.String())
	require.LessOrEqual(t, small.entity["ResolveHeaders"], 1, small.String())
	for _, op := range []string{"Get", "Head", "List", "ListEntity"} {
		require.Zero(t, small.channel[op], "%s: %s", op, small.String())
	}
	require.Zero(t, small.entity["Get"], small.String())
}

// TestInbox_MentionMarkReadCreatesMarker pins the server side of AC 24:
// marking read at a mention's seq, on a channel where alice has no marker,
// clears the mention and creates the marker, so a later message lists the
// channel under unread channels.
func TestInbox_MentionMarkReadCreatesMarker(t *testing.T) {
	e := newInboxEnv(t, func(app *App) {
		seedEntity(app, &entity.Entity{ID: "TKT-100", Type: "ticket", Properties: map[string]any{"title": "t"}})
		seedRelation(app, entity.NewRelation("alice", "officer-of", "TKT-100"))
	})
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	e.waitOn(t, "TKT-100", at)
	e.mentionAlice(t, "TKT-100", at.Add(time.Second))
	rec, _ := e.get(t, http.MethodGet, "")
	got := decodeAs[v1.Inbox](t, rec.Body.Bytes())
	require.Equal(t, 1, *got.MentionCount)
	require.Equal(t, 0, *got.UnreadCount, "no marker yet")
	seq := got.Mentions[0].Seq

	_, err := e.ch.svc.MarkRead(gateCtxFor(principalCtx("alice"), t, e.d),
		realtime.RawAddress{EntityType: "ticket", Anchor: "TKT-100", Type: "case"}, &seq)
	require.NoError(t, err)
	reads, err := e.ch.store.Reads(context.Background(), channels.Reader{User: "alice"},
		[]channels.Key{{Anchor: channels.Anchor{ID: "TKT-100"}, Type: "case"}})
	require.NoError(t, err)
	require.Len(t, reads, 1, "the mark created a marker")

	rec, _ = e.get(t, http.MethodGet, "")
	got = decodeAs[v1.Inbox](t, rec.Body.Bytes())
	require.Equal(t, 0, *got.MentionCount)
	require.Equal(t, 0, *got.UnreadCount)

	e.waitOn(t, "TKT-100", at.Add(time.Minute))
	rec, _ = e.get(t, http.MethodGet, "")
	got = decodeAs[v1.Inbox](t, rec.Body.Bytes())
	require.Equal(t, 1, *got.UnreadCount, "the marker lists the channel under unread")
}
