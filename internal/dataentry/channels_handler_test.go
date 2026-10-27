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
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

const roomPath = "/api/v1/tickets/TKT-1/_channels/room"

// chatPath is the channel of the typing type chat on TKT-1.
const chatPath = "/api/v1/tickets/TKT-1/_channels/chat"

// incidentPath is the channel of the session type incident on TKT-1.
const incidentPath = "/api/v1/tickets/TKT-1/_channels/incident"

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

	// Each page names the cursor that continues it in its own direction.
	code, b = env.get(t, "alice", roomPath+"/messages?limit=1")
	page := decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(page.Messages) != 1 || page.Messages[0].Seq != 2 || !page.HasMore ||
		page.NextBefore != 2 || page.NextAfter != 0 || page.NextAfterRev == nil || *page.NextAfterRev != head.Channel.Rev {

		t.Fatalf("newest page = %d %+v, want seq 2 continuing at next_before 2 and next_after_rev %d",
			code, page, head.Channel.Rev)
	}
	code, b = env.get(t, "alice", roomPath+"/messages?before=2&limit=1")
	page = decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(page.Messages) != 1 || page.Messages[0].Seq != 1 ||
		page.NextBefore != 1 || page.NextAfter != 0 {

		t.Fatalf("before page = %d %+v, want seq 1 continuing at next_before 1", code, page)
	}
	code, b = env.get(t, "alice", roomPath+"/messages?after=1&limit=1")
	page = decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(page.Messages) != 1 || page.Messages[0].Seq != 2 ||
		page.NextAfter != 2 || page.NextBefore != 0 {

		t.Fatalf("after page = %d %+v, want seq 2 continuing at next_after 2", code, page)
	}

	resp, b := env.do(t, "alice", http.MethodPatch, roomPath+"/messages/1", `{"props":{"body":"edited"}}`, nil)
	if resp.StatusCode != http.StatusOK || decodeAs[v1.Message](t, b).EditedAt == "" {
		t.Fatalf("edit = %d %s", resp.StatusCode, b)
	}
	code, b = env.get(t, "alice", roomPath+"/messages?after_rev="+strconv.FormatInt(head.Channel.Rev, 10))
	page = decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(page.Messages) != 1 || page.Messages[0].Seq != 1 || page.NextAfterRev == nil ||
		*page.NextAfterRev != page.Messages[0].Rev || page.NextBefore != 0 {

		t.Fatalf("after_rev page = %d %+v", code, page)
	}
	// An empty after_rev page still names where to continue, and rev 0
	// is a cursor: it follows the channel from its start.
	code, b = env.get(t, "alice", roomPath+"/messages?after_rev="+strconv.FormatInt(*page.NextAfterRev, 10))
	empty := decodeAs[v1.MessagePage](t, b)
	if code != http.StatusOK || len(empty.Messages) != 0 || empty.NextAfterRev == nil ||
		*empty.NextAfterRev != *page.NextAfterRev {

		t.Fatalf("empty after_rev page = %d %s", code, b)
	}
	code, b = env.get(t, "alice", roomPath+"/messages?after_rev=0")
	if all := decodeAs[v1.MessagePage](t, b); code != http.StatusOK || len(all.Messages) != 2 {
		t.Fatalf("after_rev=0 page = %d %s, want both messages", code, b)
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
	if code != http.StatusOK || list.Anchor != "TKT-1" || len(types) != 6 || types["topics"].Create ||
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
		{http.MethodPost, "/_channels/chat/_typing", ""},
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
// the entity GET's 404, before the route, query or body is read: a request
// that would be a 400, 405 or 413 with realtime on gets the same 404.
func TestChannels_OffIsUniform404(t *testing.T) {
	off := newChannelEnv(t)
	off.ch.cfg.Store(nil)
	plain := newLiveEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	huge := `{"type":"text","props":{"body":"` + strings.Repeat("x", channelBodyLimit) + `"}}`
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
			{http.MethodPost, roomPath + "/messages", `{"type":`},
			{http.MethodPost, roomPath + "/messages", huge},
			{http.MethodGet, roomPath + "/messages?limit=0", ""},
			{http.MethodGet, "/api/v1/tickets/TKT-1/_channels?state=x", ""},
			{http.MethodPut, roomPath, ""},
			{http.MethodGet, roomPath + "/messages/1/more", ""},
			{http.MethodPost, chatPath + "/_typing", ""},
			{http.MethodGet, chatPath + "/_typing", ""},
			{http.MethodPut, roomPath + "/_read", `{"seq":-1}`},
			{http.MethodGet, roomPath + "/_read", ""},
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

// TestChannels_Lifecycle walks the lifecycle routes: the affordance, a
// close, the refusals and a delete.
func TestChannels_Lifecycle(t *testing.T) {
	env := newChannelEnv(t)
	env.postText(t, "alice", roomPath, "hello")

	_, b := env.get(t, "alice", roomPath)
	if got := decodeAs[v1.ChannelHead](t, b).Actions.Lifecycle; !slices.Equal(got, []string{"close"}) {
		t.Fatalf("open room lifecycle actions = %v, want [close]", got)
	}

	resp, b := env.do(t, "alice", http.MethodPost, roomPath+"/_lifecycle/close", `{}`, nil)
	if resp.StatusCode != http.StatusOK || decodeAs[v1.Channel](t, b).State != "closed" {
		t.Fatalf("close = %d %s", resp.StatusCode, b)
	}
	_, b = env.get(t, "alice", roomPath)
	if got := decodeAs[v1.ChannelHead](t, b).Actions.Lifecycle; got == nil || len(got) != 0 {
		t.Fatalf("closed room lifecycle actions = %#v, want []", got)
	}

	for _, tc := range []struct {
		op    string
		code  int
		ptype string
	}{
		{"close", http.StatusConflict, realtime.TypeLifecycleWrongState},
		{"reopen", http.StatusForbidden, realtime.TypeLifecycleDenied},
		{"archive", http.StatusForbidden, realtime.TypeLifecycleNotOffered},
		{"delete", http.StatusConflict, realtime.TypeRetentionActive},
	} {
		res, body := env.do(t, "alice", http.MethodPost, roomPath+"/_lifecycle/"+tc.op, "", nil)
		if res.StatusCode != tc.code || problemType(t, body) != tc.ptype {
			t.Errorf("%s = %d %s, want %d %s", tc.op, res.StatusCode, body, tc.code, tc.ptype)
		}
	}

	resp, b = env.do(t, "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_channels/threads", `{"title":"Design"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d %s", resp.StatusCode, b)
	}
	named := "/api/v1/tickets/TKT-1/_channels/threads/" + decodeAs[v1.Channel](t, b).Name
	resp, b = env.do(t, "alice", http.MethodPost, named+"/_lifecycle/delete", "", nil)
	if resp.StatusCode != http.StatusNoContent || len(b) != 0 {
		t.Fatalf("delete = %d %s, want 204 and no body", resp.StatusCode, b)
	}
	if code, b := env.get(t, "alice", named); code != http.StatusNotFound || problemType(t, b) != realtime.TypeChannelNotFound {
		t.Fatalf("deleted head = %d %s", code, b)
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
		{"lifecycle GET", http.MethodGet, roomPath + "/_lifecycle/close", "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"unknown lifecycle op", http.MethodPost, roomPath + "/_lifecycle/explode", "", http.StatusNotFound, "not_found"},
		{"lifecycle body", http.MethodPost, roomPath + "/_lifecycle/close", `{"why":"done"}`, http.StatusBadRequest, "invalid_request"},
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
// refusal type the status table lacks. Every Type* string constant in a
// non-test file of internal/realtime is a refusal type, wherever it is
// declared.
func TestChannelErrorStatus_CoversEveryType(t *testing.T) {
	files, err := filepath.Glob("../realtime/*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if !strings.HasPrefix(name.Name, "Type") || i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, _ := strconv.Unquote(lit.Value)
					seen[value] = filepath.Base(path)
					if _, ok := channelErrorStatus[value]; !ok {
						t.Errorf("realtime.%s (%q, %s) has no status in channelErrorStatus",
							name.Name, value, filepath.Base(path))
					}
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("found no refusal types; the scan is not reading internal/realtime")
	}
	if len(seen) != len(channelErrorStatus) {
		t.Errorf("internal/realtime declares %d refusal types, the table maps %d", len(seen), len(channelErrorStatus))
	}
}

// stubChannels answers every operation with err, with realtime on.
type stubChannels struct{ err error }

func (stubChannels) Enabled() bool { return true }

func (s stubChannels) ListAnchor(context.Context, string, string, channels.State) (realtime.AnchorChannels, error) {
	return realtime.AnchorChannels{}, s.err
}
func (s stubChannels) Head(context.Context, realtime.RawAddress) (realtime.HeadView, error) {
	return realtime.HeadView{}, s.err
}
func (s stubChannels) Read(context.Context, realtime.RawAddress, realtime.ReadRequest) (realtime.ReadView, error) {
	return realtime.ReadView{}, s.err
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
func (s stubChannels) Lifecycle(context.Context, realtime.RawAddress, string) (realtime.ChannelView, error) {
	return realtime.ChannelView{}, s.err
}
func (s stubChannels) RunAction(context.Context, realtime.RawAddress, int64, string) (realtime.ActionResult, error) {
	return realtime.ActionResult{}, s.err
}
func (s stubChannels) ReleaseAction(context.Context, realtime.RawAddress, int64) (realtime.MessageView, error) {
	return realtime.MessageView{}, s.err
}
func (s stubChannels) MarkRead(context.Context, realtime.RawAddress, *int64) (realtime.MarkReadResult, error) {
	return realtime.MarkReadResult{}, s.err
}

func (s stubChannels) Typing(context.Context, realtime.RawAddress) (realtime.TypingResult, error) {
	return realtime.TypingResult{}, s.err
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
			stub := stubChannels{err: tc.err}
			app.channels = &channelsHandler{
				read: stub, messageOps: stub, actions: stub, channels: stub, typist: stub, marker: stub, on: stub,
			}
			router := app.NewRouter()
			for _, rq := range []struct{ method, path, body string }{
				{http.MethodGet, "/api/v1/tickets/TKT-1/_channels", ""},
				{http.MethodGet, roomPath, ""},
				{http.MethodPost, "/api/v1/tickets/TKT-1/_channels/threads", ""},
				{http.MethodGet, roomPath + "/messages", ""},
				{http.MethodPost, roomPath + "/messages", `{"type":"text"}`},
				{http.MethodPatch, roomPath + "/messages/1", `{"props":{}}`},
				{http.MethodDelete, roomPath + "/messages/1", ""},
				{http.MethodPost, roomPath + "/_lifecycle/close", ""},
				{http.MethodPost, roomPath + "/messages/1/_action/ack", ""},
				{http.MethodDelete, roomPath + "/messages/1/_action", ""},
				{http.MethodPost, chatPath + "/_typing", ""},
				{http.MethodPut, roomPath + "/_read", ""},
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
		{[]string{"room", "_lifecycle", "close"}, channelRoute{addr: room, lifecycle: "close"}, true},
		{[]string{"threads", "0000000000001", "_lifecycle", "delete"}, channelRoute{addr: thread, lifecycle: "delete"}, true},
		{[]string{"room", "_lifecycle", "explode"}, channelRoute{}, false},
		{[]string{"room", "_lifecycle"}, channelRoute{}, false},
		{[]string{"room", "_lifecycle", "close", "x"}, channelRoute{}, false},
		{[]string{"room", "messages", "1", "_action", "ack"}, channelRoute{addr: room, msgs: true, seq: 1, action: "ack"}, true},
		{[]string{"room", "messages", "1", "_action"}, channelRoute{addr: room, msgs: true, seq: 1, release: true}, true},
		{[]string{"room", "messages", "1", "_actions", "ack"}, channelRoute{}, false},
		{[]string{"room", "messages", "1", "_action", "ack", "x"}, channelRoute{}, false},
		{[]string{"room", "messages", "_action", "ack"}, channelRoute{}, false},
		{[]string{"room", "_typing"}, channelRoute{addr: room, typing: true}, true},
		{[]string{"threads", "0000000000001", "_typing"}, channelRoute{addr: thread, typing: true}, true},
		{[]string{"room", "_typing", "x"}, channelRoute{}, false},
		{[]string{"room", "messages", "_typing"}, channelRoute{}, false},
		{[]string{"room", "_read"}, channelRoute{addr: room, read: true}, true},
		{[]string{"threads", "0000000000001", "_read"}, channelRoute{addr: thread, read: true}, true},
		{[]string{"room", "_read", "x"}, channelRoute{}, false},
		{[]string{"room", "messages", "_read"}, channelRoute{}, false},
	} {
		t.Run(strings.Join(tc.rest, "/"), func(t *testing.T) {
			got, ok := parseChannelRoute("", "", tc.rest)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parse %v = %+v, %v; want %+v, %v", tc.rest, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestChannelsHandler_SessionInReads pins that the head, the message page
// and the list carry the session of a session type, and leave it out for a
// type without one.
func TestChannelsHandler_SessionInReads(t *testing.T) {
	env := newChannelEnv(t)
	head := func(path string) v1.ChannelHead {
		t.Helper()
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s", b)
		var h v1.ChannelHead
		require.NoError(t, json.Unmarshal(b, &h))
		return h
	}
	before := head(incidentPath).Channel.Session
	require.NotNil(t, before)
	require.Equal(t, "detected", before.State)
	require.Zero(t, before.Generation)
	require.Empty(t, before.EnteredAt, "a channel that is not stored has no entered_at")
	require.Equal(t, "working", before.Status)
	require.Equal(t, []v1.ChannelSessionTransition{{MessageType: "contained", To: "contained"}}, before.Transitions)

	resp, b := env.do(t, "alice", http.MethodPost, incidentPath+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	after := head(incidentPath).Channel.Session
	require.Equal(t, "contained", after.State)
	require.Equal(t, int64(1), after.Generation)
	require.True(t, after.Final)
	require.Equal(t, "completed", after.Status)
	require.NotEmpty(t, after.EnteredAt)

	code, b := env.get(t, "alice", incidentPath+"/messages")
	require.Equal(t, http.StatusOK, code, "%s", b)
	var page v1.MessagePage
	require.NoError(t, json.Unmarshal(b, &page))
	require.Equal(t, "contained", page.Channel.Session.State)
	require.Len(t, page.Messages, 2)
	require.Equal(t, "session", page.Messages[1].Type)

	code, b = env.get(t, "alice", "/api/v1/tickets/TKT-1/_channels")
	require.Equal(t, http.StatusOK, code, "%s", b)
	var list v1.ChannelList
	require.NoError(t, json.Unmarshal(b, &list))
	seen := map[string]bool{}
	for _, tl := range list.Types {
		for _, c := range tl.Channels {
			seen[c.Type] = c.Session != nil
		}
	}
	require.True(t, seen["incident"])

	env.postText(t, "alice", roomPath, "hello")
	require.Nil(t, head(roomPath).Channel.Session)
	require.NotContains(t, string(b), `"session":null`)
}

// TestChannelsHandler_SessionTimersInReads pins TKT-INM3AZ AC 15: an open
// channel's head, page and list carry due_at and the timers with their due
// times. A channel not stored yet, and a final state, carry neither. Under
// a degraded configuration, and on a closed channel, the timers are listed
// without due times.
func TestChannelsHandler_SessionTimersInReads(t *testing.T) {
	env := newChannelEnv(t)
	session := func(path string) *v1.ChannelSession {
		t.Helper()
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s", b)
		return decodeAs[v1.ChannelHead](t, b).Channel.Session
	}
	before := session(incidentPath)
	require.Empty(t, before.DueAt, "not stored yet")
	require.Empty(t, before.Timers)

	resp, b := env.do(t, "alice", http.MethodPost, incidentPath+"/messages", `{"type":"note","props":{"body":"x"}}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	open := session(incidentPath)
	require.NotEmpty(t, open.DueAt)
	entered, err := time.Parse(time.RFC3339Nano, open.EnteredAt)
	require.NoError(t, err)
	due, err := time.Parse(time.RFC3339Nano, open.DueAt)
	require.NoError(t, err)
	require.Equal(t, entered.Add(24*time.Hour), due)
	require.Equal(t, []v1.ChannelSessionTimer{{
		Name: "no-containment", Label: "Not contained within a day", To: "contained", Since: "state",
		DueAt: open.DueAt,
	}}, open.Timers)

	code, b := env.get(t, "alice", incidentPath+"/messages")
	require.Equal(t, http.StatusOK, code, "%s", b)
	require.Equal(t, open, decodeAs[v1.MessagePage](t, b).Channel.Session)
	code, b = env.get(t, "alice", "/api/v1/tickets/TKT-1/_channels")
	require.Equal(t, http.StatusOK, code, "%s", b)
	var listed *v1.ChannelSession
	for _, tl := range decodeAs[v1.ChannelList](t, b).Types {
		for _, c := range tl.Channels {
			if c.Type == "incident" {
				listed = c.Session
			}
		}
	}
	require.Equal(t, open, listed)

	frozen := []v1.ChannelSessionTimer{{
		Name: "no-containment", Label: "Not contained within a day", To: "contained", Since: "state",
	}}
	env.ch.degraded.Store(true)
	degraded := session(incidentPath)
	require.Empty(t, degraded.DueAt)
	require.Equal(t, frozen, degraded.Timers)
	env.ch.degraded.Store(false)

	resp, b = env.do(t, "alice", http.MethodPost, incidentPath+"/_lifecycle/close", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
	closed := session(incidentPath)
	require.Empty(t, closed.DueAt)
	require.Equal(t, frozen, closed.Timers)

	other := "/api/v1/tickets/TKT-2/_channels/incident"
	resp, b = env.do(t, "alice", http.MethodPost, other+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	final := session(other)
	require.True(t, final.Final)
	require.Empty(t, final.DueAt)
	require.Empty(t, final.Timers)
}

// runTimerRound runs one round of the leader's timer loop over env's
// channel store at now.
func runTimerRound(t *testing.T, env *channelEnv, now time.Time) realtime.TimerReport {
	t.Helper()
	tm, err := realtime.NewTimers(realtime.TimerDeps{
		Store: env.ch.store, Config: testRealtimeConfig{p: env.ch.cfg, degraded: &env.ch.degraded},
		Marker: noMarks{}, Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	rep, err := tm.RunRound(context.Background())
	require.NoError(t, err)
	return rep
}

// noMarks is a realtime.ChangeMarker that drops every mark.
type noMarks struct{}

func (noMarks) Mark(...channels.Key) {}

// TestChannelsHandler_SessionRemindersInReads pins TKT-36Z6X3 AC 17: the
// head, the page and the list carry the reminders of the current state,
// with count and with due_at equal to the next slot (not the stored hint);
// a closed channel and a degraded configuration list them without due_at,
// and a state outside the reminder's in: lists none.
func TestChannelsHandler_SessionRemindersInReads(t *testing.T) {
	env := newChannelEnv(t)
	env.ch.cfg.Store(env.ch.parse(t, strings.Replace(channelTestRealtime,
		"        contained: { label: Contained, final: true }",
		`        contained: { label: Contained, final: true }
      reminders:
        nudge:
          label: Nudge
          in: [detected]
          every: 30m
          first: 10m
          max: 5
          post: { type: note, props: { body: "Any news?" } }`, 1)))
	session := func(path string) *v1.ChannelSession {
		t.Helper()
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s", b)
		return decodeAs[v1.ChannelHead](t, b).Channel.Session
	}
	require.Empty(t, session(incidentPath).Reminders, "not stored yet")

	resp, b := env.do(t, "alice", http.MethodPost, incidentPath+"/messages", `{"type":"note","props":{"body":"x"}}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	entered, err := time.Parse(time.RFC3339Nano, session(incidentPath).EnteredAt)
	require.NoError(t, err)
	rep := runTimerRound(t, env, entered.Add(15*time.Minute))
	require.Equal(t, 1, rep.Reminded)

	open := session(incidentPath)
	require.Equal(t, []v1.ChannelSessionReminder{{
		Name: "nudge", Label: "Nudge", Every: "30m", Count: 1, Max: 5,
		DueAt: entered.Add(40 * time.Minute).UTC().Format(time.RFC3339Nano),
	}}, open.Reminders, "due_at is the slot, not the slot plus slack")
	code, b := env.get(t, "alice", incidentPath+"/messages")
	require.Equal(t, http.StatusOK, code, "%s", b)
	page := decodeAs[v1.MessagePage](t, b)
	require.Equal(t, open, page.Channel.Session)
	last := page.Messages[len(page.Messages)-1]
	require.Equal(t, "reminder:incident/nudge", last.TriggeredBy)
	code, b = env.get(t, "alice", "/api/v1/tickets/TKT-1/_channels")
	require.Equal(t, http.StatusOK, code, "%s", b)
	var listed *v1.ChannelSession
	for _, tl := range decodeAs[v1.ChannelList](t, b).Types {
		for _, c := range tl.Channels {
			if c.Type == "incident" {
				listed = c.Session
			}
		}
	}
	require.Equal(t, open, listed)

	frozen := []v1.ChannelSessionReminder{{Name: "nudge", Label: "Nudge", Every: "30m", Count: 1, Max: 5}}
	env.ch.degraded.Store(true)
	require.Equal(t, frozen, session(incidentPath).Reminders)
	env.ch.degraded.Store(false)
	resp, b = env.do(t, "alice", http.MethodPost, incidentPath+"/_lifecycle/close", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
	require.Equal(t, frozen, session(incidentPath).Reminders)

	other := "/api/v1/tickets/TKT-2/_channels/incident"
	resp, b = env.do(t, "alice", http.MethodPost, other+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	require.Empty(t, session(other).Reminders, "contained is outside in:")
}

// TestChannelTyping_Accepted pins AC 17: POST …/_typing answers 202 with
// the TTL, takes an empty body or {}, and another reader's head lists the
// typist.
func TestChannelTyping_Accepted(t *testing.T) {
	env := newChannelEnv(t)
	for _, body := range []string{"", "{}"} {
		resp, b := env.do(t, "bob", http.MethodPost, chatPath+"/_typing", body, nil)
		require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
		require.Equal(t, v1.TypingAccepted{TTLSeconds: int(realtimelimits.TypingTTL / time.Second)},
			decodeAs[v1.TypingAccepted](t, b))
		time.Sleep(realtimelimits.TypingRate.Per)
	}
	code, b := env.get(t, "alice", chatPath)
	require.Equal(t, http.StatusOK, code, "%s", b)
	head := decodeAs[v1.ChannelHead](t, b)
	require.Len(t, head.TypingNow, 1)
	require.Equal(t, "bob", head.TypingNow[0].Author.User)
	code, b = env.get(t, "bob", chatPath)
	require.Equal(t, http.StatusOK, code, "%s", b)
	require.Empty(t, decodeAs[v1.ChannelHead](t, b).TypingNow, "the typist does not see themselves")
}

// TestChannelTyping_RequestChecks pins AC 17: the method, the body and the
// refusals of the typing route.
func TestChannelTyping_RequestChecks(t *testing.T) {
	env := newChannelEnv(t)
	huge := `{"x":"` + strings.Repeat("x", channelBodyLimit) + `"}`
	for _, tc := range []struct {
		name, user, method, path, body string
		code                           int
		ptype                          string
	}{
		{"GET", "alice", http.MethodGet, chatPath + "/_typing", "", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"body", "alice", http.MethodPost, chatPath + "/_typing", `{"x":1}`, http.StatusBadRequest, "invalid_request"},
		{"too large", "alice", http.MethodPost, chatPath + "/_typing", huge,
			http.StatusRequestEntityTooLarge, realtime.TypeTooLarge},
		{"typing off", "alice", http.MethodPost, roomPath + "/_typing", "", http.StatusForbidden,
			realtime.TypeTypingNotOffered},
		{"no read", "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_channels/secret/_typing", "",
			http.StatusForbidden, realtime.TypeReadDenied},
		{"no log post role", "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_channels/briefing/_typing", "",
			http.StatusForbidden, realtime.TypePostDenied},
		{"unknown name", "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_channels/threads/0000000000001/_typing", "",
			http.StatusNotFound, realtime.TypeChannelNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, b := env.do(t, tc.user, tc.method, tc.path, tc.body, nil)
			if resp.StatusCode != tc.code || problemType(t, b) != tc.ptype {
				t.Fatalf("%s %s = %d %s, want %d %s", tc.method, tc.path, resp.StatusCode, b, tc.code, tc.ptype)
			}
			if tc.code == http.StatusMethodNotAllowed {
				require.Equal(t, http.MethodPost, resp.Header.Get("Allow"))
			}
		})
	}
}

// TestChannelTyping_HiddenAnchorKeepsNothing pins AC 17: typing on an
// anchor the caller cannot read is the uniform 404 and keeps no entry.
func TestChannelTyping_HiddenAnchorKeepsNothing(t *testing.T) {
	env := newChannelEnv(t)
	featureChat := "/api/v1/features/FEAT-1/_channels/chat"
	code, want := env.get(t, "alice", "/api/v1/features/FEAT-1")
	resp, b := env.do(t, "alice", http.MethodPost, featureChat+"/_typing", "", nil)
	requireUniform404(t, resp.StatusCode, b, code, want)
	code, b = env.get(t, "dave", featureChat)
	require.Equal(t, http.StatusOK, code, "%s", b)
	require.Empty(t, decodeAs[v1.ChannelHead](t, b).TypingNow)
}

// TestChannelsHandler_NoticeOutputs pins a session notice's outputs on
// the page: alice, who cannot read the created feature, sees it
// restricted, dave sees it, and the notice offers no actions.
func TestChannelsHandler_NoticeOutputs(t *testing.T) {
	env := newChannelEnv(t)
	env.ch.cfg.Store(env.ch.parse(t, outputsRealtime))
	const room = "/api/v1/tickets/TKT-2/_channels/incident"
	resp, b := env.do(t, "alice", http.MethodPost, room+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)

	notice := func(user string) v1.Message {
		t.Helper()
		code, b := env.get(t, user, room+"/messages")
		require.Equal(t, http.StatusOK, code, "%s", b)
		var page v1.MessagePage
		require.NoError(t, json.Unmarshal(b, &page))
		for _, m := range page.Messages {
			if m.Outputs != nil {
				return m
			}
		}
		t.Fatalf("no notice with outputs in %s", b)
		return v1.Message{}
	}
	got := notice("alice")
	require.Empty(t, got.Actions)
	require.Nil(t, got.Action)
	require.Equal(t, "done", got.Outputs.Status)
	require.Equal(t, "alice", got.Outputs.Writer.User)
	require.Len(t, got.Outputs.Results, 2)
	require.True(t, got.Outputs.Results[1].Entity.Restricted)
	require.Empty(t, got.Outputs.Results[1].Entity.ID)

	seen := notice("dave").Outputs.Results[1].Entity
	require.False(t, seen.Restricted)
	require.Equal(t, "FEAT-1", seen.ID)
}
