package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// messageAt reads message seq of the room as user.
func (e *channelEnv) messageAt(t *testing.T, user string, seq int64) v1.Message {
	t.Helper()
	code, b := e.get(t, user, roomPath+"/messages")
	if code != http.StatusOK {
		t.Fatalf("messages = %d %s", code, b)
	}
	for _, m := range decodeAs[v1.MessagePage](t, b).Messages {
		if m.Seq == seq {
			return m
		}
	}
	t.Fatalf("message %d not on the page", seq)
	return v1.Message{}
}

// pend sets a pending ack on message seq, pending since at, straight in
// the store: a promote that never finished.
func (e *channelEnv) pend(t *testing.T, seq int64, at time.Time) {
	t.Helper()
	key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	if _, err := e.ch.store.SetAction(context.Background(), key, seq, "ack", channels.ActionNone, channels.Action{
		Status: channels.ActionPending, By: channels.Author{User: "alice"}, At: at, PendingAt: at,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestChannels_Actions walks the action routes: the affordance, a run, the
// refusals, and the release of a pending action.
func TestChannels_Actions(t *testing.T) {
	env := newChannelEnv(t)
	first := env.postText(t, "alice", roomPath, "ship it")

	// Without a promoter, accept is not offered and the POST agrees.
	if got := env.messageAt(t, "bob", first.Seq).Actions; !slices.Equal(got, []string{"ack"}) {
		t.Fatalf("_actions = %v, want [ack]", got)
	}
	ackPath := roomPath + "/messages/1/_action/ack"
	for _, tc := range []struct{ path, ptype string }{
		{roomPath + "/messages/1/_action/accept", realtime.TypeActionNotOffered},
		{roomPath + "/messages/1/_action/nope", realtime.TypeActionNotOffered},
	} {
		res, b := env.do(t, "bob", http.MethodPost, tc.path, "", nil)
		if res.StatusCode != http.StatusForbidden || problemType(t, b) != tc.ptype {
			t.Errorf("POST %s = %d %s, want 403 %s", tc.path, res.StatusCode, b, tc.ptype)
		}
	}

	res, b := env.do(t, "bob", http.MethodPost, ackPath, `{}`, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("ack = %d %s", res.StatusCode, b)
	}
	got := decodeAs[v1.MessageActionResponse](t, b)
	if got.Message.Action == nil || got.Message.Action.Name != "ack" || got.Message.Action.Status != "done" ||
		len(got.Message.Actions) != 0 || len(got.Warnings) != 0 {

		t.Fatalf("ack answer = %+v", got)
	}
	res, b = env.do(t, "alice", http.MethodPost, ackPath, "", nil)
	if res.StatusCode != http.StatusConflict || problemType(t, b) != realtime.TypeActionTaken {
		t.Fatalf("second ack = %d %s, want 409 action_taken", res.StatusCode, b)
	}
	res, b = env.do(t, "alice", http.MethodDelete, roomPath+"/messages/1/_action", "", nil)
	if res.StatusCode != http.StatusConflict || problemType(t, b) != realtime.TypeActionNotPending {
		t.Fatalf("release of a done action = %d %s, want 409 action_not_pending", res.StatusCode, b)
	}

	// A pending action freezes the message until it may be released.
	young := env.postText(t, "alice", roomPath, "young")
	env.pend(t, young.Seq, time.Now())
	youngPath := roomPath + "/messages/2"
	for _, rq := range []struct{ method, path, body string }{
		{http.MethodPatch, youngPath, `{"props":{"body":"edited"}}`},
		{http.MethodDelete, youngPath, ""},
		{http.MethodDelete, youngPath + "/_action", ""},
	} {
		res, b := env.do(t, "alice", rq.method, rq.path, rq.body, nil)
		if res.StatusCode != http.StatusConflict || problemType(t, b) != realtime.TypeActionPending {
			t.Errorf("%s %s = %d %s, want 409 action_pending", rq.method, rq.path, res.StatusCode, b)
		}
	}

	old := env.postText(t, "alice", roomPath, "old")
	env.pend(t, old.Seq, time.Now().Add(-realtimelimits.PendingRelease-time.Minute))
	res, b = env.do(t, "bob", http.MethodDelete, roomPath+"/messages/3/_action", `{}`, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("release = %d %s", res.StatusCode, b)
	}
	if m := decodeAs[v1.Message](t, b); m.Action != nil || !slices.Equal(m.Actions, []string{"ack"}) {
		t.Fatalf("released message = %+v, want no action and ack offered again", m)
	}
}

// TestChannels_ActionRouteChecks pins the method and body checks of the
// action routes.
func TestChannels_ActionRouteChecks(t *testing.T) {
	env := newChannelEnv(t)
	env.postText(t, "alice", roomPath, "hello")
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{http.MethodGet, roomPath + "/messages/1/_action/ack", "", http.StatusMethodNotAllowed},
		{http.MethodPost, roomPath + "/messages/1/_action", "", http.StatusMethodNotAllowed},
		{http.MethodPost, roomPath + "/messages/1/_action/ack", `{"x":1}`, http.StatusBadRequest},
		{http.MethodPost, roomPath + "/messages/1/_action/ack/x", "", http.StatusNotFound},
	} {
		res, b := env.do(t, "alice", tc.method, tc.path, tc.body, nil)
		if res.StatusCode != tc.code {
			t.Errorf("%s %s = %d %s, want %d", tc.method, tc.path, res.StatusCode, b, tc.code)
		}
	}
}

// okActions answers every action with a fixed result.
type okActions struct{ res realtime.ActionResult }

func (a okActions) RunAction(context.Context, realtime.RawAddress, int64, string) (realtime.ActionResult, error) {
	return a.res, nil
}

func (a okActions) ReleaseAction(context.Context, realtime.RawAddress, int64) (realtime.MessageView, error) {
	return a.res.Message, nil
}

// TestChannels_ActionAnswerCarriesWarnings pins the wire shape of a
// promote that succeeded with warnings.
func TestChannels_ActionAnswerCarriesWarnings(t *testing.T) {
	app := newTestAppV1(t)
	stub := stubChannels{}
	app.channels = &channelsHandler{read: stub, messageOps: stub, channels: stub, on: stub, actions: okActions{
		res: realtime.ActionResult{
			Message: realtime.MessageView{Seq: 1, Rev: 2, Action: &realtime.ActionView{
				Name: "accept", Status: channels.ActionDone,
				Promoted: &realtime.RefValue{ID: "TKT-9", Type: "ticket", Title: "x"},
			}},
			Warnings: []entity.Warning{{Code: realtime.WarnPromoteUnconfirmed, Detail: "d"}},
		},
	}}
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, roomPath+"/messages/1/_action/accept",
		http.NoBody).WithContext(aliceCtx()))
	if rec.Code != http.StatusOK {
		t.Fatalf("action = %d %s", rec.Code, rec.Body)
	}
	got := decodeAs[v1.MessageActionResponse](t, rec.Body.Bytes())
	if got.Message.Action == nil || got.Message.Action.Promoted == nil || got.Message.Action.Promoted.ID != "TKT-9" ||
		len(got.Warnings) != 1 || got.Warnings[0].Code != realtime.WarnPromoteUnconfirmed {

		t.Fatalf("answer = %s", rec.Body)
	}
}
