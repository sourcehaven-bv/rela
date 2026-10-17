package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

const roomPath = "/api/v1/tickets/TKT-1/_channels/room"

// decodeAs decodes b into a T and fails the test on an error.
func decodeAs[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return v
}

// problemType returns the last segment of a problem body's type.
func problemType(t *testing.T, b []byte) string {
	t.Helper()
	p := decodeAs[v1.Error](t, b)
	return p.Type[strings.LastIndex(p.Type, "/")+1:]
}

// postText posts a text message as user and requires 201.
func (e *channelEnv) postText(t *testing.T, user, path, body string) v1.Message {
	t.Helper()
	resp, b := e.do(t, user, http.MethodPost, path+"/messages",
		fmt.Sprintf(`{"type":"text","props":{"body":%q}}`, body), nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post to %s as %s = %d %s, want 201", path, user, resp.StatusCode, b)
	}
	return decodeAs[v1.Message](t, b)
}

// TestChannels_Routes walks every route once as a member.
func TestChannels_Routes(t *testing.T) {
	env := newChannelEnv(t)

	first := env.postText(t, "alice", roomPath, "hello")
	if first.Seq != 1 || first.Author == nil || first.Author.User != "alice" || first.Props["body"] != "hello" {
		t.Fatalf("posted = %+v", first)
	}
	env.postText(t, "bob", roomPath, "hi")

	code, b := env.get(t, "alice", roomPath)
	if code != http.StatusOK {
		t.Fatalf("head = %d %s", code, b)
	}
	head := decodeAs[v1.ChannelHead](t, b)
	if head.Channel.LastSeq != 2 || head.Last == nil || head.Last.Seq != 2 ||
		!head.Actions.EditOwn || len(head.Actions.Post) != 2 {

		t.Fatalf("head = %+v", head)
	}

	code, b = env.get(t, "alice", roomPath+"/messages?limit=1")
	page := decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(page.Messages) != 1 || !page.HasMore || page.NextAfter != page.Messages[0].Seq {
		t.Fatalf("page = %d %+v", code, page)
	}

	resp, b := env.do(t, "alice", http.MethodPatch, roomPath+"/messages/1", `{"props":{"body":"edited"}}`, nil)
	if resp.StatusCode != http.StatusOK || decodeAs[v1.Message](t, b).EditedAt == "" {
		t.Fatalf("edit = %d %s", resp.StatusCode, b)
	}
	code, b = env.get(t, "alice", roomPath+"/messages?after_rev="+strconv.FormatInt(head.Channel.Rev, 10))
	page = decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(page.Messages) != 1 || page.Messages[0].Seq != 1 || page.NextAfterRev == 0 {
		t.Fatalf("after_rev page = %d %+v", code, page)
	}

	resp, b = env.do(t, "alice", http.MethodDelete, roomPath+"/messages/1", "", nil)
	if resp.StatusCode != http.StatusOK || decodeAs[v1.Message](t, b).Removed == nil {
		t.Fatalf("delete = %d %s", resp.StatusCode, b)
	}

	resp, b = env.do(t, "alice", http.MethodPost, roomPath+"/messages", `{"type":"typing"}`, nil)
	acc := decodeAs[v1.EphemeralAccepted](t, b)
	if resp.StatusCode != http.StatusAccepted || acc.ID == "" || acc.ExpiresAt == "" {
		t.Fatalf("ephemeral = %d %s", resp.StatusCode, b)
	}
	_, b = env.get(t, "bob", roomPath)
	if eph := decodeAs[v1.ChannelHead](t, b).Ephemeral; len(eph) != 1 || eph[0].ID != acc.ID {
		t.Fatalf("head ephemeral = %+v", eph)
	}

	resp, b = env.do(t, "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_channels/threads", `{"title":"Design"}`, nil)
	created := decodeAs[v1.Channel](t, b)
	if resp.StatusCode != http.StatusCreated || len(created.Name) != 13 || created.Title != "Design" {
		t.Fatalf("create = %d %s", resp.StatusCode, b)
	}
	named := "/api/v1/tickets/TKT-1/_channels/threads/" + created.Name
	env.postText(t, "bob", named, "in the thread")
	if code, b = env.get(t, "alice", named); code != http.StatusOK || decodeAs[v1.ChannelHead](t, b).Channel.LastSeq != 1 {
		t.Fatalf("named head = %d %s", code, b)
	}

	code, b = env.get(t, "alice", "/api/v1/tickets/TKT-1/_channels")
	list := decodeAs[v1.ChannelList](t, b)
	types := map[string]v1.ChannelTypeList{}
	for _, tl := range list.Types {
		types[tl.Type] = tl
	}
	if code != http.StatusOK || list.Anchor != "TKT-1" || len(types) != 3 || types["topics"].Create ||
		len(types["threads"].Channels) != 1 || !types["threads"].Create {

		t.Fatalf("list = %d %s", code, b)
	}
	if _, ok := types["secret"]; ok {
		t.Fatal("the list shows a channel type alice may not read")
	}
	if code, b = env.get(t, "alice", "/api/v1/tickets/TKT-1/_channels?state=closed"); code != http.StatusOK ||
		len(decodeAs[v1.ChannelList](t, b).Types[0].Channels) != 0 {

		t.Fatalf("closed list = %d %s", code, b)
	}
}

// AC2: a hidden or nonexistent anchor, and realtime off, answer the
// entity GET's 404 on every channel route, byte for byte apart from
// instance.
func TestChannels_HiddenAnchorUniform404(t *testing.T) {
	env := newChannelEnv(t)
	routes := []struct{ method, suffix, body string }{
		{http.MethodGet, "/_channels", ""},
		{http.MethodGet, "/_channels/room", ""},
		{http.MethodGet, "/_channels/room/messages", ""},
		{http.MethodPost, "/_channels/room/messages", `{"type":"text","props":{"body":"x"}}`},
		{http.MethodPatch, "/_channels/room/messages/1", `{"props":{"body":"x"}}`},
		{http.MethodDelete, "/_channels/room/messages/1", ""},
	}
	for _, tc := range []struct {
		name, user, entity string
	}{
		{"hidden", "alice", "/api/v1/features/FEAT-1"},
		{"nonexistent", "alice", "/api/v1/tickets/TKT-404"},
		{"wrong plural", "dave", "/api/v1/features/TKT-1"},
	} {
		code, want := env.get(t, tc.user, tc.entity)
		for _, rt := range routes {
			t.Run(tc.name+" "+rt.method+" "+rt.suffix, func(t *testing.T) {
				resp, b := env.do(t, tc.user, rt.method, tc.entity+rt.suffix, rt.body, nil)
				requireUniform404(t, resp.StatusCode, b, code, want)
			})
		}
	}

	// An unknown channel type, or one not attached to the URL's entity
	// type, answers before the anchor read, so a hidden anchor and a
	// visible one get the same answer. realtime.yaml is not a secret.
	for _, path := range []string{
		"/api/v1/tickets/TKT-1/_channels/nope",
		"/api/v1/features/FEAT-1/_channels/nope",
		"/api/v1/features/FEAT-1/_channels/secret",
		"/api/v1/features/FEAT-404/_channels/threads",
	} {
		resp, b := env.do(t, "alice", http.MethodGet, path, "", nil)
		if resp.StatusCode != http.StatusNotFound || problemType(t, b) != realtime.TypeUnknownChannelType {
			t.Errorf("GET %s = %d %s, want 404 unknown_channel_type", path, resp.StatusCode, b)
		}
	}
}

// AC14: with realtime off, or channels not wired, every channel route is
// the entity GET's 404.
func TestChannels_OffIsUniform404(t *testing.T) {
	off := newChannelEnv(t)
	off.ch.cfg.Store(nil)
	plain := newLiveEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	for _, tc := range []struct {
		name string
		env  *liveEnv
	}{{"realtime off", off.liveEnv}, {"not wired", plain}} {
		code, want := tc.env.get(t, "alice", "/api/v1/tickets/TKT-404")
		for _, rq := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/v1/tickets/TKT-1/_channels", ""},
			{http.MethodGet, roomPath, ""},
			{http.MethodGet, roomPath + "/messages", ""},
			{http.MethodPost, roomPath + "/messages", `{"type":"text","props":{"body":"x"}}`},
			{http.MethodPost, "/api/v1/tickets/TKT-1/_channels/threads", ""},
		} {
			t.Run(tc.name+" "+rq.method+" "+rq.path, func(t *testing.T) {
				resp, b := tc.env.do(t, "alice", rq.method, rq.path, rq.body, nil)
				requireUniform404(t, resp.StatusCode, b, code, want)
			})
		}
	}
}

// AC3: a refused role or gate is a 403 that names the rule.
func TestChannels_DenialsNameTheRule(t *testing.T) {
	env := newChannelEnv(t)
	env.postText(t, "alice", roomPath, "mine")
	for _, tc := range []struct {
		name, user, method, path, body, ptype string
	}{
		{"read", "alice", http.MethodGet, "/api/v1/tickets/TKT-1/_channels/secret", "", realtime.TypeReadDenied},
		{"post", "alice", http.MethodPost, roomPath + "/messages", `{"type":"announce","props":{"body":"x"}}`, realtime.TypePostDenied},
		{"create", "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_channels/topics", "", realtime.TypeCreateDenied},
		{"edit another's", "bob", http.MethodPatch, roomPath + "/messages/1", `{"props":{"body":"x"}}`, realtime.TypeEditDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, b := env.do(t, tc.user, tc.method, tc.path, tc.body, nil)
			p := decodeAs[v1.Error](t, b)
			if resp.StatusCode != http.StatusForbidden || problemType(t, b) != tc.ptype || p.Detail == "" {
				t.Fatalf("%s = %d %s, want 403 %s with the rule in detail", tc.name, resp.StatusCode, b, tc.ptype)
			}
		})
	}
}

// AC4: a ref to an entity the poster cannot read and a ref to one that does
// not exist get the same 422.
func TestChannels_RefsUniform422(t *testing.T) {
	env := newChannelEnv(t)
	post := func(ref string) (int, string) {
		resp, b := env.do(t, "alice", http.MethodPost, roomPath+"/messages",
			fmt.Sprintf(`{"type":"text","props":{"body":"x","about":%q}}`, ref), nil)
		return resp.StatusCode, string(b)
	}
	hiddenCode, hidden := post("FEAT-1")
	missingCode, missing := post("FEAT-404")
	if hiddenCode != http.StatusUnprocessableEntity || missingCode != hiddenCode {
		t.Fatalf("codes = %d %s and %d %s, want 422 for both", hiddenCode, hidden, missingCode, missing)
	}
	if hidden != missing {
		t.Fatalf("bodies differ:\n hidden=  %s\n missing= %s", hidden, missing)
	}
	if code, b := post("TKT-2"); code != http.StatusCreated {
		t.Fatalf("readable ref = %d %s", code, b)
	}
	_, b := env.get(t, "dave", roomPath)
	if ref, _ := decodeAs[v1.ChannelHead](t, b).Last.Props["about"].(map[string]any); ref["id"] != "TKT-2" ||
		ref["title"] != "two" {

		t.Fatalf("rendered ref = %v", ref)
	}
}

// The request checks the handler makes before the service: method, route
// shape, body and query.
func TestChannels_RequestChecks(t *testing.T) {
	env := newChannelEnv(t)
	huge := `{"type":"text","props":{"body":"` + strings.Repeat("x", channelBodyLimit) + `"}}`
	for _, tc := range []struct {
		name, method, path, body string
		code                     int
		ptype                    string
	}{
		{"list PUT", http.MethodPut, "/api/v1/tickets/TKT-1/_channels", "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"head DELETE", http.MethodDelete, roomPath, "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"named head POST", http.MethodPost, roomPath + "/0000000000001", "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"messages PUT", http.MethodPut, roomPath + "/messages", "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"message GET", http.MethodGet, roomPath + "/messages/1", "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"seq not a number", http.MethodPatch, roomPath + "/messages/one", "", http.StatusNotFound, "not_found"},
		{"seq zero", http.MethodPatch, roomPath + "/messages/0", "", http.StatusNotFound, "not_found"},
		{"too deep", http.MethodGet, roomPath + "/messages/1/more", "", http.StatusNotFound, "not_found"},
		{"body too large", http.MethodPost, roomPath + "/messages", huge, http.StatusRequestEntityTooLarge, realtime.TypeTooLarge},
		{"unknown field", http.MethodPost, roomPath + "/messages", `{"type":"text","oops":1}`, http.StatusBadRequest, "invalid_request"},
		{"trailing data", http.MethodPost, roomPath + "/messages", `{"type":"text"} {}`, http.StatusBadRequest, "invalid_request"},
		{"empty post body", http.MethodPost, roomPath + "/messages", "", http.StatusBadRequest, "invalid_request"},
		{"invalid message", http.MethodPost, roomPath + "/messages", `{"type":"text","props":{}}`, http.StatusBadRequest, realtime.TypeInvalid},
		{"bad after", http.MethodGet, roomPath + "/messages?after=x", "", http.StatusBadRequest, "invalid_request"},
		{"bad limit", http.MethodGet, roomPath + "/messages?limit=0", "", http.StatusBadRequest, "invalid_request"},
		{"bad state", http.MethodGet, "/api/v1/tickets/TKT-1/_channels?state=gone", "", http.StatusBadRequest, "invalid_request"},
		{"two cursors", http.MethodGet, roomPath + "/messages?after=1&before=3", "", http.StatusBadRequest, realtime.TypeInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, b := env.do(t, "alice", tc.method, tc.path, tc.body, nil)
			if resp.StatusCode != tc.code || problemType(t, b) != tc.ptype {
				t.Fatalf("%s %s = %d %s, want %d %s", tc.method, tc.path, resp.StatusCode, b, tc.code, tc.ptype)
			}
		})
	}
}

// AC14: a bare id of a type whose channels hang off a face is a 422
// face_required, before the anchor read.
func TestChannels_BareIDOnFaceTypeIsFaceRequired(t *testing.T) {
	app, d, _ := facedAppWithPolicy(t, acl.RoleDef{Read: []string{"*"}, Update: []string{"*"}})
	ch := newChannelService(t, app.State().Meta, app.store, d, `
version: 1
channel_types:
  review:
    attach_to: [ticket]
    anchor: face
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`)
	if err := SetChannels(app, ch.svc); err != nil {
		t.Fatal(err)
	}
	get := func(addr string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/"+addr+"/_channels/review", http.NoBody)
		app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, d)))
		return rec
	}
	if rec := get("TKT-1"); rec.Code != http.StatusUnprocessableEntity ||
		problemType(t, rec.Body.Bytes()) != realtime.TypeFaceRequired {

		t.Fatalf("bare id = %d %s, want 422 face_required", rec.Code, rec.Body)
	}
	if rec := get("TKT-1@published"); rec.Code != http.StatusOK {
		t.Fatalf("faced address = %d %s, want 200", rec.Code, rec.Body)
	}
}

// TestChannelErrorStatus_CoversEveryType fails when the service declares a
// refusal type the status table lacks: errors.go is the list.
func TestChannelErrorStatus_CoversEveryType(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "../realtime/errors.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range vs.Names {
			if !strings.HasPrefix(name.Name, "Type") || i >= len(vs.Values) {
				continue
			}
			lit, ok := vs.Values[i].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			seen++
			value, _ := strconv.Unquote(lit.Value)
			if _, ok := channelErrorStatus[value]; !ok {
				t.Errorf("realtime.%s (%q) has no status in channelErrorStatus", name.Name, value)
			}
		}
		return true
	})
	if seen != len(channelErrorStatus) {
		t.Errorf("errors.go declares %d refusal types, the table maps %d", seen, len(channelErrorStatus))
	}
}

// stubChannels answers every operation with err.
type stubChannels struct{ err error }

func (s stubChannels) ListAnchor(context.Context, string, string, channels.State) (realtime.AnchorChannels, error) {
	return realtime.AnchorChannels{}, s.err
}
func (s stubChannels) Head(context.Context, realtime.RawAddress) (realtime.HeadView, error) {
	return realtime.HeadView{}, s.err
}
func (s stubChannels) Messages(context.Context, realtime.RawAddress, channels.Page) (realtime.MessagePage, error) {
	return realtime.MessagePage{}, s.err
}
func (s stubChannels) Post(context.Context, realtime.RawAddress, realtime.PostInput) (realtime.PostResult, error) {
	return realtime.PostResult{}, s.err
}
func (s stubChannels) Create(context.Context, string, string, string, realtime.CreateInput) (realtime.ChannelView, error) {
	return realtime.ChannelView{}, s.err
}
func (s stubChannels) Edit(context.Context, realtime.RawAddress, int64, map[string]any) (realtime.MessageView, error) {
	return realtime.MessageView{}, s.err
}
func (s stubChannels) Delete(context.Context, realtime.RawAddress, int64, string) (realtime.MessageView, error) {
	return realtime.MessageView{}, s.err
}

// The error mapping every channel route shares, for every entry of the
// table plus the sentinels and an internal failure.
func TestChannels_ErrorMapping(t *testing.T) {
	type mapping struct {
		name       string
		err        error
		code       int
		ptype      string
		retryAfter string
	}
	cases := []mapping{
		{"realtime off", realtime.ErrRealtimeOff, http.StatusNotFound, "not_found", ""},
		{"anchor", realtime.ErrAnchorNotFound, http.StatusNotFound, "not_found", ""},
		{"no principal", realtime.ErrNoPrincipal, http.StatusForbidden, realtime.TypeNoPrincipal, ""},
		{"internal", errors.New("disk on fire"), http.StatusInternalServerError, "channel_failed", ""},
		{"rate limited", &realtime.DenialError{Status: http.StatusTooManyRequests, Type: realtime.TypeRateLimited,
			Title: "slow down", RetryAfter: 1200 * time.Millisecond}, http.StatusTooManyRequests, realtime.TypeRateLimited, "2"},
	}
	for typ, code := range channelErrorStatus {
		if code != http.StatusTooManyRequests {
			cases = append(cases, mapping{typ, &realtime.DenialError{Status: code, Type: typ, Title: typ}, code, typ, ""})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			app.channels = &channelsHandler{svc: stubChannels{err: tc.err}}
			router := app.NewRouter()
			for _, rq := range []struct{ method, path, body string }{
				{http.MethodGet, "/api/v1/tickets/TKT-1/_channels", ""},
				{http.MethodGet, roomPath, ""},
				{http.MethodPost, "/api/v1/tickets/TKT-1/_channels/threads", ""},
				{http.MethodGet, roomPath + "/messages", ""},
				{http.MethodPost, roomPath + "/messages", `{"type":"text"}`},
				{http.MethodPatch, roomPath + "/messages/1", `{"props":{}}`},
				{http.MethodDelete, roomPath + "/messages/1", ""},
			} {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(rq.method, rq.path, strings.NewReader(rq.body)).
					WithContext(aliceCtx()))
				if rec.Code != tc.code || problemType(t, rec.Body.Bytes()) != tc.ptype {
					t.Errorf("%s %s = %d %s, want %d %s", rq.method, rq.path, rec.Code, rec.Body, tc.code, tc.ptype)
				}
				if got := rec.Header().Get("Retry-After"); got != tc.retryAfter {
					t.Errorf("%s %s: Retry-After = %q, want %q", rq.method, rq.path, got, tc.retryAfter)
				}
			}
		})
	}
}

// The body limit leaves room for the largest message the config may allow.
func TestChannels_BodyLimitFitsTheLargestMessage(t *testing.T) {
	if channelBodyLimit <= realtimelimits.MaxMessageBytes {
		t.Fatalf("channelBodyLimit %d does not exceed MaxMessageBytes %d", channelBodyLimit, realtimelimits.MaxMessageBytes)
	}
}

// parseChannelRoute is structural: `messages` is the subresource wherever
// it stands, and a shape it does not know is refused. An empty segment
// never reaches it over HTTP, because the mux cleans the path, but the
// parser refuses one anyway.
func TestParseChannelRoute(t *testing.T) {
	room := realtime.RawAddress{Type: "room"}
	thread := realtime.RawAddress{Type: "threads", Name: "0000000000001"}
	for _, tc := range []struct {
		rest []string
		want channelRoute
		ok   bool
	}{
		{[]string{"room"}, channelRoute{addr: room}, true},
		{[]string{"room", "messages"}, channelRoute{addr: room, msgs: true}, true},
		{[]string{"room", "messages", "7"}, channelRoute{addr: room, msgs: true, seq: 7}, true},
		{[]string{"threads", "0000000000001"}, channelRoute{addr: thread}, true},
		{[]string{"threads", "0000000000001", "messages", "2"}, channelRoute{addr: thread, msgs: true, seq: 2}, true},
		{[]string{"room", ""}, channelRoute{}, false},
		{[]string{"", "messages"}, channelRoute{}, false},
		{[]string{"room", "messages", "-1"}, channelRoute{}, false},
		{[]string{"room", "messages", "1", "x"}, channelRoute{}, false},
		{[]string{"threads", "a", "b"}, channelRoute{}, false},
	} {
		t.Run(strings.Join(tc.rest, "/"), func(t *testing.T) {
			got, ok := parseChannelRoute("", "", tc.rest)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parse %v = %+v, %v; want %+v, %v", tc.rest, got, ok, tc.want, tc.ok)
			}
		})
	}
}
