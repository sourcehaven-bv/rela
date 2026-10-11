package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// The MCP comment operations (TKT-QIQO1W) run the HTTP routes' gates. These
// tests drive them through MCPComments with the ctx the remote endpoint's
// middleware stamps, and pin that each refusal carries the route's code.

func mcpCtx(t *testing.T, user string, d *acl.Declarative) context.Context {
	t.Helper()
	ctx := principal.With(t.Context(), principal.Principal{User: user, Tool: principal.ToolMCP})
	if d != nil {
		ctx = gateCtxFor(ctx, t, d)
	}
	return ctx
}

func mcpOps(t *testing.T, app *App) *MCPComments {
	t.Helper()
	ops := mcpHost(app).Comments
	require.NotNil(t, ops, "comments are enabled, so the host must carry the operations")
	return ops
}

// requireOpCode asserts err is an OpError with code.
func requireOpCode(t *testing.T, err error, code string) *comments.OpError {
	t.Helper()
	var opErr *comments.OpError
	require.ErrorAs(t, err, &opErr)
	require.Equal(t, code, opErr.Code, opErr.Error())
	return opErr
}

func TestMCPComments_DisabledIsNil(t *testing.T) {
	app := newTestAppV1(t)
	require.Nil(t, mcpHost(app).Comments)
}

func TestMCPComments_Roundtrip(t *testing.T) {
	app := commentsApp(t)
	ops := mcpOps(t, app)
	ctx := mcpCtx(t, "alice@example.com", nil)

	created, err := ops.AddComment(ctx, "TKT-001", comments.AddInput{
		Kind: comments.AnchorProperty, Ref: "status", Body: "looks wrong",
	})
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", created.Author)

	list, err := ops.ListComments(ctx, "TKT-001")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, created.ID, list[0].ID)
	require.True(t, list[0].Editable)
	require.False(t, list[0].Detached)

	require.NoError(t, ops.UpdateComment(ctx, "TKT-001", created.ID, nil, new(true)))
	list, err = ops.ListComments(ctx, "TKT-001")
	require.NoError(t, err)
	require.True(t, list[0].Resolved)
	require.Equal(t, "looks wrong", list[0].Body, "a nil body keeps the stored one")

	require.NoError(t, ops.DeleteComment(ctx, "TKT-001", created.ID))
	list, err = ops.ListComments(ctx, "TKT-001")
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestMCPComments_Refusals(t *testing.T) {
	app := commentsApp(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "A"}})
	ops := mcpOps(t, app)
	ctx := mcpCtx(t, "alice@example.com", nil)
	note := comments.AddInput{Kind: comments.AnchorProperty, Ref: "status", Body: "x"}

	_, err := ops.ListComments(ctx, "TKT-404")
	requireOpCode(t, err, "not_found")
	_, err = ops.ListComments(ctx, "../etc")
	requireOpCode(t, err, "not_found")
	_, err = ops.ListComments(ctx, "FEAT-001")
	requireOpCode(t, err, "comments_not_enabled")
	requireOpCode(t, ops.DeleteComment(ctx, "TKT-001", "nope"), "not_found")

	_, err = ops.AddComment(ctx, "TKT-001", comments.AddInput{Kind: comments.AnchorText, Quote: "absent text here", Body: "x"})
	requireOpCode(t, err, "invalid_comment")
	_, err = ops.AddComment(ctx, "TKT-001", comments.AddInput{Kind: comments.AnchorProperty, Ref: "status"})
	requireOpCode(t, err, "invalid_comment")

	_, err = ops.AddComment(mcpCtx(t, "unknown", nil), "TKT-001", note)
	requireOpCode(t, err, "forbidden")
}

func TestMCPComments_ReadOnly(t *testing.T) {
	app := commentsApp(t)
	ops := mcpOps(t, app)
	ctx := mcpCtx(t, "alice@example.com", nil)
	created, err := ops.AddComment(ctx, "TKT-001", comments.AddInput{Kind: comments.AnchorProperty, Ref: "status", Body: "x"})
	require.NoError(t, err)
	app.acl = acl.ReadOnlyACL{}

	_, err = ops.AddComment(ctx, "TKT-001", comments.AddInput{Kind: comments.AnchorProperty, Ref: "status", Body: "x"})
	require.Contains(t, requireOpCode(t, err, "forbidden").Title, "read-only")
	requireOpCode(t, ops.UpdateComment(ctx, "TKT-001", created.ID, new("y"), nil), "forbidden")
	requireOpCode(t, ops.DeleteComment(ctx, "TKT-001", created.ID), "forbidden")
	_, err = ops.AcceptSuggestion(ctx, "TKT-001", created.ID)
	requireOpCode(t, err, "forbidden")

	_, err = ops.AddComment(ctx, "TKT-404", comments.AddInput{Kind: comments.AnchorProperty, Ref: "status", Body: "x"})
	require.Contains(t, requireOpCode(t, err, "forbidden").Title, "read-only", "refused before the target resolves")

	list, err := ops.ListComments(ctx, "TKT-001")
	require.NoError(t, err, "reads still work")
	require.Len(t, list, 1)
}

func TestMCPComments_AcceptSuggestion(t *testing.T) {
	suggest := func(t *testing.T, ops *MCPComments, ctx context.Context, replacement *string) string {
		t.Helper()
		c, err := ops.AddComment(ctx, "TKT-001", comments.AddInput{
			Kind: comments.AnchorText, Quote: suggestQuote, Replacement: replacement, Body: "suggest",
		})
		require.NoError(t, err)
		return c.ID
	}

	t.Run("applies and resolves", func(t *testing.T) {
		app := commentsApp(t)
		ops := mcpOps(t, app)
		ctx := mcpCtx(t, suggestAuthor, nil)
		id := suggest(t, ops, ctx, new(suggestReplace))

		list, err := ops.ListComments(ctx, "TKT-001")
		require.NoError(t, err)
		require.True(t, list[0].Acceptable)

		got, err := ops.AcceptSuggestion(ctx, "TKT-001", id)
		require.NoError(t, err)
		require.Contains(t, got.Content, suggestReplace)
		require.Contains(t, storedBody(t, app), suggestReplace)

		_, err = ops.AcceptSuggestion(ctx, "TKT-001", id)
		requireOpCode(t, err, "comment_resolved")
	})

	t.Run("no suggestion", func(t *testing.T) {
		app := commentsApp(t)
		ops := mcpOps(t, app)
		ctx := mcpCtx(t, suggestAuthor, nil)
		id := suggest(t, ops, ctx, nil)
		_, err := ops.AcceptSuggestion(ctx, "TKT-001", id)
		requireOpCode(t, err, "no_suggestion")
	})

	t.Run("stale", func(t *testing.T) {
		app := commentsApp(t)
		ops := mcpOps(t, app)
		ctx := mcpCtx(t, suggestAuthor, nil)
		id := suggest(t, ops, ctx, new(suggestReplace))
		setBody(t, app, "This paragraph shares no wording whatsoever with the original text.\n")
		_, err := ops.AcceptSuggestion(ctx, "TKT-001", id)
		requireOpCode(t, err, "suggestion_stale")
	})
}

func TestMCPComments_Authorization(t *testing.T) {
	commenting := []string{"comment:read", "comment:add"}
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reviewer": {Read: []string{"ticket"}, Permissions: commenting},
			"editor":   {Read: []string{"ticket"}, Update: []string{"ticket"}, Permissions: commenting},
			"blind":    {Read: []string{"ticket"}, Update: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "reviewer", "bob": "editor", "carol": "blind"},
	}
	app, d, sink := suggestionACLApp(t, policy)
	ops := mcpOps(t, app)
	created, err := ops.AddComment(mcpCtx(t, "alice", d), "TKT-001", comments.AddInput{
		Kind: comments.AnchorText, Quote: suggestQuote, Replacement: new(suggestReplace), Body: "suggest",
	})
	require.NoError(t, err)

	t.Run("unreadable target is not found", func(t *testing.T) {
		_, err := ops.ListComments(mcpCtx(t, "mallory", d), "TKT-001")
		requireOpCode(t, err, "not_found")
	})
	t.Run("without comment:read", func(t *testing.T) {
		_, err := ops.ListComments(mcpCtx(t, "carol", d), "TKT-001")
		require.Contains(t, requireOpCode(t, err, "forbidden").Title, "comment:read")
		_, err = ops.AddComment(mcpCtx(t, "carol", d), "TKT-001",
			comments.AddInput{Kind: comments.AnchorProperty, Ref: "status", Body: "x"})
		require.Contains(t, requireOpCode(t, err, "forbidden").Title, "comment:add")
		_, err = ops.AcceptSuggestion(mcpCtx(t, "carol", d), "TKT-001", created.ID)
		require.Contains(t, requireOpCode(t, err, "forbidden").Title, "comment:read")
	})
	t.Run("someone else's comment", func(t *testing.T) {
		err := ops.DeleteComment(mcpCtx(t, "bob", d), "TKT-001", created.ID)
		require.Contains(t, requireOpCode(t, err, "forbidden").Title, "comment:delete-own")
	})
	t.Run("commenter without entity update", func(t *testing.T) {
		_, err := ops.AcceptSuggestion(mcpCtx(t, "alice", d), "TKT-001", created.ID)
		requireOpCode(t, err, "forbidden")
		require.Equal(t, fixtureBody, storedBody(t, app))
		var denied int
		for _, r := range sink.Records() {
			if r.Op == audit.OpDeniedWrite && r.Principal.User == "alice" {
				denied++
			}
		}
		require.Equal(t, 1, denied, "the refusal is audited, as on the HTTP route")
	})
	t.Run("provisioning runs before the target resolves", func(t *testing.T) {
		// mallory cannot read the ticket until provisioned; this seam grants
		// the reads a provisioned user entity would.
		suggestion, err := ops.AddComment(mcpCtx(t, "alice", d), "TKT-001", comments.AddInput{
			Kind: comments.AnchorText, Quote: suggestQuote, Replacement: new(suggestReplace), Body: "again",
		})
		require.NoError(t, err)
		prev := app.comments.provision
		t.Cleanup(func() { app.comments.provision = prev })
		app.comments.provision = func(context.Context) context.Context { return mcpCtx(t, "bob", d) }

		_, err = ops.AcceptSuggestion(mcpCtx(t, "mallory", d), "TKT-001", suggestion.ID)
		require.NoError(t, err)
		setBody(t, app, fixtureBody)
	})
	t.Run("editor accepts", func(t *testing.T) {
		_, err := ops.AcceptSuggestion(mcpCtx(t, "bob", d), "TKT-001", created.ID)
		require.NoError(t, err)
		require.Contains(t, storedBody(t, app), suggestReplace)
	})
}

func TestMCPComments_Faces(t *testing.T) {
	app, d := facedCommentsApp(t, nil)
	ops := mcpOps(t, app)
	ctx := withWorld(mcpCtx(t, "alice", d), worldHandle{name: "published", scope: rankedPolicyWorld()})
	note := comments.AddInput{Kind: comments.AnchorProperty, Ref: "title", Body: "check this"}

	_, err := ops.AddComment(ctx, "POL-1", note)
	opErr := requireOpCode(t, err, "face_required")
	require.Equal(t, []string{"POL-1@draft", "POL-1@published"}, opErr.Faces)

	created, err := ops.AddComment(ctx, "POL-1@draft", note)
	require.NoError(t, err)
	draft, err := ops.ListComments(ctx, "POL-1@draft")
	require.NoError(t, err)
	require.Len(t, draft, 1)
	require.Equal(t, created.ID, draft[0].ID)
	published, err := ops.ListComments(ctx, "POL-1@published")
	require.NoError(t, err)
	require.Empty(t, published, "threads are per face")

	requireOpCode(t, ops.DeleteComment(ctx, "POL-1", created.ID), "face_required")
	requireOpCode(t, ops.UpdateComment(ctx, "POL-1", created.ID, nil, new(true)), "face_required")
	_, err = ops.AcceptSuggestion(ctx, "POL-1", created.ID)
	requireOpCode(t, err, "face_required")
}

// A named face the caller may not read answers like a face that does not
// exist, on reads and writes alike, while the readable face works.
func TestMCPComments_UnreadableFaceIsNotFound(t *testing.T) {
	app, d := facedCommentsApp(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"viewer": {
			Read:        []string{"policy@published", "feature"},
			Permissions: []string{"comment:read", "comment:add"},
		}},
		Assignments: map[string]string{"alice": "viewer"},
	})
	ops := mcpOps(t, app)
	ctx := withWorld(mcpCtx(t, "alice", d), worldHandle{name: "published", scope: rankedPolicyWorld()})
	note := comments.AddInput{Kind: comments.AnchorProperty, Ref: "title", Body: "check this"}

	for _, address := range []string{"POL-1@draft", "POL-1@nope"} {
		_, err := ops.ListComments(ctx, address)
		hidden := requireOpCode(t, err, "not_found")
		_, err = ops.AddComment(ctx, address, note)
		require.Equal(t, hidden, requireOpCode(t, err, "not_found"), address)
	}
	_, err := ops.AddComment(ctx, "POL-1@published", note)
	require.NoError(t, err)
}

// commentsOverMCPMount serves, at MCPPath, a handler that lists TKT-001's
// comments through the host's comment operations with the request's ctx. It
// stands in for the MCP server, which is not importable here, so the test
// covers what the mount must deliver: the ctx the gates read.
func commentsOverMCPMount(t *testing.T, app *App, subject string) (code int, body string) {
	t.Helper()
	require.NoError(t, app.SetJWTGate(JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: subject},
		HeaderName: "X-Auth-Assertion",
	}))
	require.NoError(t, app.SetRemoteMCP(func(host MCPHost) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := host.Comments.AddComment(r.Context(), "TKT-001", comments.AddInput{
				Kind: comments.AnchorProperty, Ref: "status", Body: "via mcp",
			}); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			list, err := host.Comments.ListComments(r.Context(), "TKT-001")
			if err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(list[0].Author))
		}), nil
	}))
	req := httptest.NewRequest(http.MethodPost, MCPPath, strings.NewReader("{}"))
	req.Header.Set("X-Auth-Assertion", "good.jwt.token")
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestMCPComments_ThroughTheRemoteMount(t *testing.T) {
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reviewer": {Read: []string{"ticket"}, Permissions: []string{"comment:read", "comment:add"}},
			"blind":    {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "reviewer", "carol": "blind"},
	}

	t.Run("the verified user is the author", func(t *testing.T) {
		app, _, _ := suggestionACLApp(t, policy)
		code, body := commentsOverMCPMount(t, app, "alice")
		require.Equal(t, http.StatusOK, code, body)
		require.Equal(t, "alice", body)
	})
	t.Run("the request's grants apply", func(t *testing.T) {
		app, _, _ := suggestionACLApp(t, policy)
		code, body := commentsOverMCPMount(t, app, "carol")
		require.Equal(t, http.StatusForbidden, code)
		require.Contains(t, body, "comment:add")
	})
}

// An internal failure reaches an MCP caller as the route's code and constant
// detail, never the cause; a cancelled request returns the ctx error itself.
func TestCommentFailure_OpError(t *testing.T) {
	cause := errors.New("open /srv/project/.rela/comments/TKT-001.yaml: permission denied")
	for _, tc := range []struct {
		name string
		fail *commentFailure
		code string
	}{
		{"gate", failGate(cause), "acl_query_failed"},
		{"gate timeout", failGate(context.DeadlineExceeded), "acl_query_timeout"},
		{"internal", failInternal("Could not read comments", cause), "comments_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opErr := requireOpCode(t, tc.fail.opError(t.Context()), tc.code)
			require.Equal(t, internalErrorDetail, opErr.Detail)
			require.NotContains(t, opErr.Error(), "/srv/project")
		})
	}

	err := failGate(context.Canceled).opError(t.Context())
	require.ErrorIs(t, err, context.Canceled)
	var opErr *comments.OpError
	require.NotErrorAs(t, err, &opErr)
}
