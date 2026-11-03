package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// Inbox waiting-message tests (TKT-V05706X): the inbox page carries the
// message that moved each waiting session and the caller's decision
// inputs, equal to what the channel head and message list answer.

// askRun appends an ask by bob, a watcher, to id's run channel, moving
// its session to asking. It writes through the store,
// as the service would, so seeding many runs meets no rate limit.
func (e *inboxEnv) askRun(t *testing.T, id, question string) {
	t.Helper()
	_, err := e.ch.store.Append(context.Background(), channels.Key{Anchor: channels.Anchor{ID: id}, Type: "run"},
		channels.NewMessage{
			Type: "ask", Author: channels.Author{User: "bob", Tool: principal.ToolDataEntry}, At: time.Now(),
			Props: map[string]any{"question": question, "about": id},
			Session: &channels.SessionStep{
				Decide: func(cur channels.Channel) (channels.SessionMove, bool) {
					from := cur.Session.State
					if from == "" {
						from = "working"
					}
					return channels.SessionMove{From: from, To: "asking"}, true
				},
				NoticeType:  realtimecfg.SessionNoticeType,
				NoticeBy:    channels.Author{User: principal.UserRealtime},
				TriggeredBy: channels.SessionMarker + "run",
				Due:         func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{} },
			},
		})
	require.NoError(t, err)
}

func runAddr(id string) realtime.RawAddress {
	return realtime.RawAddress{EntityType: "ticket", Anchor: id, Type: "run"}
}

// seedRuns seeds n tickets owned by alice with bob watching, alice an
// officer of each, and n tickets she may not read through the run type,
// each with an asked run.
func seedRuns(t *testing.T, n int) *inboxEnv {
	t.Helper()
	e := seedRunTickets(t, n)
	for i := range n {
		e.askRun(t, fmt.Sprintf("TKT-%d", 100+i), "Which store?")
		e.askRun(t, fmt.Sprintf("TKT-%d", 500+i), "secret")
	}
	return e
}

// seedRunTickets seeds the tickets of [seedRuns] without asking.
func seedRunTickets(t *testing.T, n int) *inboxEnv {
	t.Helper()
	return newInboxEnv(t, func(app *App) {
		for i := range n {
			for _, id := range []string{fmt.Sprintf("TKT-%d", 100+i), fmt.Sprintf("TKT-%d", 500+i)} {
				seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{
					"title": "run " + id, "owner": "alice", "watchers": []any{"bob"},
				}})
				seedRelation(app, entity.NewRelation("bob", "officer-of", id))
			}
			seedRelation(app, entity.NewRelation("alice", "officer-of", fmt.Sprintf("TKT-%d", 100+i)))
		}
	})
}

// TestInbox_DecisionInputsMatchHead pins AC 4 through the real ACL: each
// waiting entry's actions, session transitions and cause equal the
// channel head's for alice, and its message equals the message list's
// without the per-message actions and deadlines.
func TestInbox_DecisionInputsMatchHead(t *testing.T) {
	e := seedRuns(t, 2)
	rec, _ := e.get(t, http.MethodGet, "?kinds=waiting")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeAs[v1.Inbox](t, rec.Body.Bytes())
	require.Len(t, got.Entries, 2)
	ctx := gateCtxFor(principalCtx("alice"), t, e.d)
	for _, en := range got.Entries {
		addr := runAddr(en.Channel.Anchor)
		h, err := e.ch.svc.Head(ctx, addr)
		require.NoError(t, err)
		head := wireHead(h)
		require.Equal(t, head.Actions.Post, en.Actions.Post)
		require.Equal(t, head.Actions.Transitions, en.Actions.Transitions)
		require.Equal(t, head.Actions.Entries, en.Actions.Entries)
		require.Equal(t, head.Channel.Session.Transitions, en.Session.Transitions)
		require.Equal(t, head.Channel.Session.Cause, en.Session.Cause)
		require.Equal(t, head.Channel.LastSeq, en.LastSeq)
		require.Equal(t, []string{"answer"}, en.Actions.Transitions, "nudge is a watcher's")

		page, err := e.ch.svc.Read(ctx, addr, realtime.ReadRequest{})
		require.NoError(t, err)
		var want *v1.Message
		for _, m := range page.Messages {
			if m.Seq == en.Session.Cause {
				w := wireMessage(m)
				w.Actions, w.EditUntil, w.ReleaseAfter = nil, "", ""
				want = &w
			}
		}
		require.NotNil(t, want)
		wantJSON, err := json.Marshal(want)
		require.NoError(t, err)
		gotJSON, err := json.Marshal(en.Message)
		require.NoError(t, err)
		require.JSONEq(t, string(wantJSON), string(gotJSON))
		require.Equal(t, "Which store?", en.Message.Props["question"])
	}
}

// TestInbox_WaitingMessagesHiddenRowsRealACL pins AC 5 through the real
// ACL: runs on tickets alice may not read through the run type change no
// byte of the page, and their messages never show.
func TestInbox_WaitingMessagesHiddenRowsRealACL(t *testing.T) {
	small := seedRunTickets(t, 3)
	for i := range 3 {
		small.askRun(t, fmt.Sprintf("TKT-%d", 100+i), "Which store?")
	}
	base, _ := small.get(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, base.Code, base.Body.String())
	for i := range 3 {
		small.askRun(t, fmt.Sprintf("TKT-%d", 500+i), "secret")
	}
	rec, _ := small.get(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, base.Body.String(), rec.Body.String(), "hidden rows change no byte of the page")
	require.NotContains(t, rec.Body.String(), "secret")
	got := decodeAs[v1.Inbox](t, rec.Body.Bytes())
	require.Len(t, got.Entries, 3)
	for _, en := range got.Entries {
		require.NotNil(t, en.Message)
	}
}

// TestInboxBudget_WaitingMessages pins AC 6 on the route: the message step
// adds one GetMessages and no Get, Head or Messages, and one entity read
// for the refs of all entries, at 10 entries as at 50.
func TestInboxBudget_WaitingMessages(t *testing.T) {
	cost := func(n int) channelCost {
		e := seedRuns(t, n)
		rec, c := e.get(t, http.MethodGet, "?limit=200")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodeAs[v1.Inbox](t, rec.Body.Bytes())
		require.Len(t, got.Entries, n)
		for _, en := range got.Entries {
			require.NotNil(t, en.Message)
		}
		return c
	}
	small, large := cost(10), cost(50)
	require.True(t, small.equal(large), "10 entries: %s\n50 entries: %s", small, large)
	require.Equal(t, 1, small.channel["GetMessages"], small.String())
	require.Zero(t, small.channel["Get"], small.String())
	require.Zero(t, small.channel["Head"], small.String())
	require.Zero(t, small.channel["Messages"], small.String())
	e := seedRuns(t, 10)
	_, sum := e.summary(t, http.MethodGet, "")
	want := maps.Clone(sum.entity)
	want["ListEntityHeaders"]++
	require.Equal(t, want, small.entity, "the message step resolves the refs of all entries in one read")
}

// TestInboxSummary_NoWaitingMessages pins that the summary route never
// runs the message step.
func TestInboxSummary_NoWaitingMessages(t *testing.T) {
	e := seedRuns(t, 2)
	rec, c := e.summary(t, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Zero(t, c.channel["GetMessages"], c.String())
}
