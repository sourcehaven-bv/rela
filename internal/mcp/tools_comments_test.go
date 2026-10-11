package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// fakeComments records the calls the comment tools make and answers with a
// fixed thread, or with err when set.
type fakeComments struct {
	err error

	address   string
	commentID string
	add       comments.AddInput
	body      *string
	resolved  *bool
	calls     []string
	// user is the principal the last add ran as.
	user string
}

var fakeThread = []comments.View{{
	ID: "c1", Author: "alice", CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	Anchor: comments.AnchorView{Kind: comments.AnchorProperty, Ref: "title"}, Body: "Shorter?",
	Editable: true, Deletable: true,
}}

func (f *fakeComments) ListComments(_ context.Context, address string) ([]comments.View, error) {
	f.calls, f.address = append(f.calls, "list"), address
	if f.err != nil {
		return nil, f.err
	}
	return fakeThread, nil
}

func (f *fakeComments) AddComment(ctx context.Context, address string, in comments.AddInput) (comments.Comment, error) {
	f.calls, f.address, f.add = append(f.calls, "add"), address, in
	f.user = principal.From(ctx).User
	if f.err != nil {
		return comments.Comment{}, f.err
	}
	return comments.Comment{ID: "c2", Author: "tester", Body: in.Body}, nil
}

func (f *fakeComments) UpdateComment(_ context.Context, address, id string, body *string, resolved *bool) error {
	f.calls, f.address, f.commentID, f.body, f.resolved = append(f.calls, "update"), address, id, body, resolved
	return f.err
}

func (f *fakeComments) DeleteComment(_ context.Context, address, id string) error {
	f.calls, f.address, f.commentID = append(f.calls, "delete"), address, id
	return f.err
}

func (f *fakeComments) AcceptSuggestion(_ context.Context, address, id string) (comments.Accepted, error) {
	f.calls, f.address, f.commentID = append(f.calls, "accept"), address, id
	if f.err != nil {
		return comments.Accepted{}, f.err
	}
	return comments.Accepted{Content: "new body"}, nil
}

// newCommentServer builds a server with fake as its comment backing; writes
// selects the remote shape (all five tools) over the stdio one.
func newCommentServer(t *testing.T, fake *fakeComments, writes bool) *Server {
	t.Helper()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	deps.Comments.Reader = fake
	if writes {
		deps.Comments.Writer = fake
	}
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func listedTools(t *testing.T, s *Server) []string {
	t.Helper()
	result, rpcErr := dispatch(t, s, "tools/list", `{}`)
	if rpcErr != nil {
		t.Fatalf("tools/list: %s", rpcErr.Message)
	}
	var decoded struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(decoded.Tools))
	for _, tool := range decoded.Tools {
		names = append(names, tool.Name)
	}
	return names
}

var commentWriteTools = []string{"add_comment", "update_comment", "delete_comment", "accept_comment_suggestion"}

func TestCommentTools_Registration(t *testing.T) {
	t.Parallel()

	t.Run("comments off: none", func(t *testing.T) {
		t.Parallel()
		meta, st := makeTestFixture(t)
		deps := newTestDeps(t, meta, st)
		deps.Comments.Reader = nil
		srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range listedTools(t, srv) {
			if strings.Contains(name, "comment") {
				t.Errorf("%s registered without a comments backing", name)
			}
		}
	})
	t.Run("stdio: list only", func(t *testing.T) {
		t.Parallel()
		names := listedTools(t, newCommentServer(t, &fakeComments{}, false))
		if !slices.Contains(names, "list_comments") {
			t.Error("list_comments not registered")
		}
		for _, name := range commentWriteTools {
			if slices.Contains(names, name) {
				t.Errorf("%s registered without a comment writer", name)
			}
		}
	})
	t.Run("remote: all five", func(t *testing.T) {
		t.Parallel()
		names := listedTools(t, newCommentServer(t, &fakeComments{}, true))
		for _, name := range append([]string{"list_comments"}, commentWriteTools...) {
			if !slices.Contains(names, name) {
				t.Errorf("%s not registered", name)
			}
		}
	})
}

func TestCommentTools_PassArguments(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		text, isErr := callTool(t, newCommentServer(t, fake, true), "list_comments", `{"id":" REQ-001@draft "}`)
		if isErr {
			t.Fatal(text)
		}
		if fake.address != "REQ-001@draft" {
			t.Errorf("address = %q", fake.address)
		}
		var got struct {
			Comments []comments.View `json:"comments"`
		}
		if err := json.Unmarshal([]byte(text), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Comments) != 1 || got.Comments[0].ID != fakeThread[0].ID || !got.Comments[0].Editable {
			t.Errorf("comments = %+v", got.Comments)
		}
	})
	t.Run("add text with deletion suggestion", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		text, isErr := callTool(t, newCommentServer(t, fake, true), "add_comment",
			`{"id":"REQ-001","body":"drop it","anchor":"text","quote":"some words","quote_prefix":"a ","replacement":""}`)
		if isErr {
			t.Fatal(text)
		}
		in := fake.add
		if in.Kind != comments.AnchorText || in.Quote != "some words" || in.QuotePrefix != "a " || in.Body != "drop it" {
			t.Errorf("input = %+v", in)
		}
		if in.Replacement == nil || *in.Replacement != "" {
			t.Errorf("an empty replacement must reach the backing as a deletion, got %v", in.Replacement)
		}
	})
	t.Run("add without replacement", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		callTool(t, newCommentServer(t, fake, true), "add_comment",
			`{"id":"REQ-001","body":"b","anchor":"property","ref":"title"}`)
		if fake.add.Replacement != nil || fake.add.Ref != "title" {
			t.Errorf("input = %+v", fake.add)
		}
	})
	t.Run("update resolves only", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		_, isErr := callTool(t, newCommentServer(t, fake, true), "update_comment",
			`{"id":"REQ-001","comment_id":"c1","resolved":true}`)
		if isErr || fake.body != nil || fake.resolved == nil || !*fake.resolved || fake.commentID != "c1" {
			t.Errorf("body=%v resolved=%v id=%q", fake.body, fake.resolved, fake.commentID)
		}
	})
	t.Run("update with nothing", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		_, isErr := callTool(t, newCommentServer(t, fake, true), "update_comment", `{"id":"REQ-001","comment_id":"c1"}`)
		if !isErr || len(fake.calls) != 0 {
			t.Errorf("isErr=%v calls=%v", isErr, fake.calls)
		}
	})
	t.Run("update with mistyped resolved", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		_, isErr := callTool(t, newCommentServer(t, fake, true), "update_comment",
			`{"id":"REQ-001","comment_id":"c1","resolved":"yes"}`)
		if !isErr || len(fake.calls) != 0 {
			t.Errorf("isErr=%v calls=%v", isErr, fake.calls)
		}
	})
	t.Run("delete and accept", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		s := newCommentServer(t, fake, true)
		if text, isErr := callTool(t, s, "delete_comment", `{"id":"REQ-001","comment_id":"c1"}`); isErr {
			t.Fatal(text)
		}
		text, isErr := callTool(t, s, "accept_comment_suggestion", `{"id":"REQ-001","comment_id":"c1"}`)
		if isErr || !strings.Contains(text, `"content": "new body"`) {
			t.Errorf("accept: %s", text)
		}
		if !slices.Equal(fake.calls, []string{"delete", "accept"}) {
			t.Errorf("calls = %v", fake.calls)
		}
	})
	t.Run("empty comment_id", func(t *testing.T) {
		t.Parallel()
		fake := &fakeComments{}
		_, isErr := callTool(t, newCommentServer(t, fake, true), "delete_comment", `{"id":"REQ-001","comment_id":" "}`)
		if !isErr || len(fake.calls) != 0 {
			t.Errorf("isErr=%v calls=%v", isErr, fake.calls)
		}
	})
}

func TestCommentTools_RenderOpError(t *testing.T) {
	t.Parallel()
	fake := &fakeComments{err: &comments.OpError{
		Code: "face_required", Title: "Address one face", Detail: "REQ-001 has 2 faces",
		Faces: []string{"REQ-001@draft", "REQ-001@published"},
	}}
	text, isErr := callTool(t, newCommentServer(t, fake, true), "add_comment",
		`{"id":"REQ-001","body":"b","anchor":"property","ref":"title"}`)
	if !isErr {
		t.Fatal("want an error result")
	}
	for _, want := range []string{"Address one face", "REQ-001@draft", "REQ-001@published", "[face_required]"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q missing from %q", want, text)
		}
	}
}

// A reload can drop the comment deps while the tools stay registered (the
// tool set is fixed at construction). The tools must then refuse instead of
// calling a nil interface.
func TestCommentTools_AfterAReloadDropsComments(t *testing.T) {
	t.Parallel()
	fake := &fakeComments{}
	srv := newCommentServer(t, fake, true)
	reloaded := srv.deps()
	reloaded.Comments = CommentDeps{}
	if err := srv.ReloadDeps(reloaded); err != nil {
		t.Fatalf("ReloadDeps: %v", err)
	}

	for name, args := range map[string]string{
		"list_comments": `{"id":"REQ-001"}`,
		"add_comment":   `{"id":"REQ-001","body":"b","anchor":"property","ref":"title"}`,
	} {
		text, isErr := callTool(t, srv, name, args)
		if !isErr || !strings.Contains(text, "not enabled") {
			t.Errorf("%s after reload = %q (error %v), want a not-enabled error", name, text, isErr)
		}
	}
	if len(fake.calls) != 0 {
		t.Errorf("the dropped backing was called: %v", fake.calls)
	}
}

// The remote mount hands the SDK the request ctx, stamped with the verified
// caller. This sends a real tools/call through HTTPHandler and checks that the
// comment write runs as that caller, not as the server's fallback principal.
func TestCommentTools_RunAsTheHTTPCaller(t *testing.T) {
	t.Parallel()
	fake := &fakeComments{}
	srv := newCommentServer(t, fake, true)
	stamp := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller := principal.Principal{User: "alice", Tool: principal.ToolMCP}
			next.ServeHTTP(w, r.WithContext(principal.With(r.Context(), caller)))
		})
	}
	ts := httptest.NewServer(stamp(srv.HTTPHandler()))
	t.Cleanup(ts.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"add_comment",` +
		`"arguments":{"id":"REQ-001","body":"b","anchor":"property","ref":"title"}}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(fake.calls, []string{"add"}) {
		t.Fatalf("calls = %v, want one add", fake.calls)
	}
	if fake.user != "alice" {
		t.Errorf("add ran as %q, want the HTTP caller alice", fake.user)
	}
}
