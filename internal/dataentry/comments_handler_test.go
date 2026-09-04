package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// commentsApp returns a test App with commenting enabled for `ticket` and one
// seeded ticket to comment on.
func commentsApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	app.State().Meta.Comments = &metamodel.CommentsConfig{Enabled: true, On: []string{"ticket"}}

	svc, err := comments.NewService(memcomments.New(), nil)
	require.NoError(t, err)
	app.SetComments(svc)

	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "Test Ticket", "status": "open"},
	})
	return app
}

// asUser stamps a principal so the service can attribute an author.
func asUser(r *http.Request, user string) *http.Request {
	return r.WithContext(principal.With(r.Context(),
		principal.Principal{User: user, Tool: "data-entry"}))
}

func doComments(t *testing.T, app *App, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, http.NoBody)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if user != "" {
		req = asUser(req, user)
	}
	rec := httptest.NewRecorder()
	app.handleV1Comments(rec, req)
	return rec
}

func listComments(t *testing.T, app *App) commentListResponse {
	t.Helper()
	rec := doComments(t, app, http.MethodGet, "/api/v1/_comments/ticket/TKT-001", "", "alice@example.com")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got commentListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

const addBody = `{"anchor":{"kind":"property","ref":"status"},"body":"looks wrong"}`

// TestComments_DisabledRoutes404 pins AC1: with no `comments:` block the routes
// do not exist, so a project without the block is indistinguishable from one
// built before the feature.
func TestComments_DisabledRoutes404(t *testing.T) {
	app := newTestAppV1(t) // no comments config, no service

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/_comments/ticket/TKT-001", ""},
		{http.MethodPost, "/api/v1/_comments/ticket/TKT-001", addBody},
		{http.MethodDelete, "/api/v1/_comments/ticket/TKT-001/abc", ""},
	} {
		t.Run(tc.method, func(t *testing.T) {
			rec := doComments(t, app, tc.method, tc.path, tc.body, "alice@example.com")
			require.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

// TestComments_AddAndList pins AC2.
func TestComments_AddAndList(t *testing.T) {
	app := commentsApp(t)

	rec := doComments(t, app, http.MethodPost, "/api/v1/_comments/ticket/TKT-001", addBody, "alice@example.com")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	got := listComments(t, app)
	require.Len(t, got.Comments, 1)
	require.Equal(t, "looks wrong", got.Comments[0].Body)
	require.Equal(t, "status", got.Comments[0].Anchor.Ref)
	require.Equal(t, "property", got.Comments[0].Anchor.Kind)
}

// TestComments_AuthorIsServerStamped is AC3 at the HTTP boundary: a client that
// puts an author (or an id) in the body must not have it honored.
func TestComments_AuthorIsServerStamped(t *testing.T) {
	app := commentsApp(t)

	forged := `{"anchor":{"kind":"property","ref":"status"},"body":"x",` +
		`"author":"bob@example.com","id":"forged-id"}`
	rec := doComments(t, app, http.MethodPost, "/api/v1/_comments/ticket/TKT-001", forged, "alice@example.com")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	got := listComments(t, app)
	require.Len(t, got.Comments, 1)
	require.Equal(t, "alice@example.com", got.Comments[0].Author,
		"the request principal wins over a body-supplied author")
	require.NotEqual(t, "forged-id", got.Comments[0].ID,
		"the server mints the id")
}

// TestComments_UnidentifiedAuthorRefused pins that a caller with no resolvable
// identity cannot leave a comment nobody can later own.
func TestComments_UnidentifiedAuthorRefused(t *testing.T) {
	app := commentsApp(t)

	rec := doComments(t, app, http.MethodPost, "/api/v1/_comments/ticket/TKT-001", addBody, "")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestComments_TypeNotEnabled pins AC4. The refusal names the reason: which
// types accept comments is metamodel config, not a secret.
func TestComments_TypeNotEnabled(t *testing.T) {
	app := commentsApp(t)
	seedEntity(app, &entity.Entity{
		ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "A feature"},
	})

	rec := doComments(t, app, http.MethodGet, "/api/v1/_comments/feature/FEAT-001", "", "alice@example.com")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "comments_not_enabled")
}

// TestComments_InvalidPaths pins the path guards, including the traversal shape
// that must never reach a file backend.
func TestComments_InvalidPaths(t *testing.T) {
	app := commentsApp(t)

	for _, path := range []string{
		"/api/v1/_comments/ticket",
		"/api/v1/_comments/",
		"/api/v1/_comments/ticket/../../etc",
		"/api/v1/_comments/ticket/.hidden",
	} {
		t.Run(path, func(t *testing.T) {
			rec := doComments(t, app, http.MethodGet, path, "", "alice@example.com")
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

// TestComments_BodyValidation pins AC12 at the boundary.
func TestComments_BodyValidation(t *testing.T) {
	app := commentsApp(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty body", `{"anchor":{"kind":"property","ref":"status"},"body":""}`},
		{"whitespace body", `{"anchor":{"kind":"property","ref":"status"},"body":"   "}`},
		{"control char in body", "{\"anchor\":{\"kind\":\"property\",\"ref\":\"status\"},\"body\":\"a\\u0000b\"}"},
		{"oversized body", `{"anchor":{"kind":"property","ref":"status"},"body":"` +
			strings.Repeat("x", comments.MaxBodyBytes+1) + `"}`},
		{"unknown anchor kind", `{"anchor":{"kind":"wat","ref":"status"},"body":"x"}`},
		{"empty anchor ref", `{"anchor":{"kind":"property","ref":""},"body":"x"}`},
		{"malformed json", `{not json`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doComments(t, app, http.MethodPost,
				"/api/v1/_comments/ticket/TKT-001", tc.body, "alice@example.com")
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

// TestComments_DetachedAnchor pins AC9: an anchor naming a property that no
// longer exists still returns, flagged — a warning, never a rejection.
func TestComments_DetachedAnchor(t *testing.T) {
	app := commentsApp(t)

	live := `{"anchor":{"kind":"property","ref":"status"},"body":"on a real property"}`
	gone := `{"anchor":{"kind":"property","ref":"no_such_property"},"body":"on a ghost"}`
	for _, body := range []string{live, gone} {
		rec := doComments(t, app, http.MethodPost, "/api/v1/_comments/ticket/TKT-001", body, "alice@example.com")
		require.Equal(t, http.StatusCreated, rec.Code, "a detached anchor must not be refused")
	}

	got := listComments(t, app)
	require.Len(t, got.Comments, 2)

	byRef := map[string]commentWire{}
	for _, c := range got.Comments {
		byRef[c.Anchor.Ref] = c
	}
	require.False(t, byRef["status"].Detached)
	require.True(t, byRef["no_such_property"].Detached, "a vanished property reads as detached")
}

// TestComments_SectionAnchorNeverDetached pins that a section ref — which
// resolves against the view config, not the entity — is not mislabelled by the
// per-entity property check.
func TestComments_SectionAnchorNeverDetached(t *testing.T) {
	app := commentsApp(t)

	body := `{"anchor":{"kind":"section","ref":"acceptance-criteria"},"body":"about this section"}`
	rec := doComments(t, app, http.MethodPost, "/api/v1/_comments/ticket/TKT-001", body, "alice@example.com")
	require.Equal(t, http.StatusCreated, rec.Code)

	got := listComments(t, app)
	require.Len(t, got.Comments, 1)
	require.False(t, got.Comments[0].Detached,
		"a section anchor is not resolvable per-entity and must not be flagged")
}

// TestComments_UpdateAndDelete covers the mutating routes end to end.
func TestComments_UpdateAndDelete(t *testing.T) {
	app := commentsApp(t)

	rec := doComments(t, app, http.MethodPost, "/api/v1/_comments/ticket/TKT-001", addBody, "alice@example.com")
	require.Equal(t, http.StatusCreated, rec.Code)
	var created commentWire
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	path := "/api/v1/_comments/ticket/TKT-001/" + created.ID

	t.Run("resolve without echoing the body", func(t *testing.T) {
		rec := doComments(t, app, http.MethodPatch, path, `{"resolved":true}`, "alice@example.com")
		require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

		got := listComments(t, app)
		require.True(t, got.Comments[0].Resolved)
		require.Equal(t, "looks wrong", got.Comments[0].Body,
			"an unsupplied body is preserved, not blanked")
	})

	t.Run("edit the body", func(t *testing.T) {
		rec := doComments(t, app, http.MethodPatch, path, `{"body":"actually fine"}`, "alice@example.com")
		require.Equal(t, http.StatusNoContent, rec.Code)

		got := listComments(t, app)
		require.Equal(t, "actually fine", got.Comments[0].Body)
		require.True(t, got.Comments[0].Resolved, "an unsupplied flag is preserved too")
	})

	t.Run("delete", func(t *testing.T) {
		rec := doComments(t, app, http.MethodDelete, path, "", "alice@example.com")
		require.Equal(t, http.StatusNoContent, rec.Code)
		require.Empty(t, listComments(t, app).Comments)
	})
}

func TestComments_UnknownCommentIs404(t *testing.T) {
	app := commentsApp(t)

	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := doComments(t, app, method,
				"/api/v1/_comments/ticket/TKT-001/nosuchcomment", `{"body":"x"}`, "alice@example.com")
			require.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

// TestComments_UnknownTargetIs404 pins that a comment route cannot be used to
// probe for entities: an absent target answers exactly as a denied one would.
func TestComments_UnknownTargetIs404(t *testing.T) {
	app := commentsApp(t)

	rec := doComments(t, app, http.MethodGet, "/api/v1/_comments/ticket/TKT-999", "", "alice@example.com")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestComments_MethodNotAllowed(t *testing.T) {
	app := commentsApp(t)

	t.Run("collection", func(t *testing.T) {
		rec := doComments(t, app, http.MethodPut, "/api/v1/_comments/ticket/TKT-001", "", "alice@example.com")
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		require.Contains(t, rec.Header().Get("Allow"), "GET")
	})

	t.Run("item", func(t *testing.T) {
		rec := doComments(t, app, http.MethodGet, "/api/v1/_comments/ticket/TKT-001/abc", "", "alice@example.com")
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		require.Contains(t, rec.Header().Get("Allow"), "PATCH")
	})
}

// TestComments_ReadOnlyInstanceRefusesWrites pins that ReadOnlyACL — wired for
// "absolute confidence no writes happen" — covers comments too. They bypass the
// entitymanager, so its blanket write deny never sees them; this is where the
// guarantee is kept.
func TestComments_ReadOnlyInstanceRefusesWrites(t *testing.T) {
	app := commentsApp(t)
	app.acl = acl.ReadOnlyACL{}

	t.Run("add refused", func(t *testing.T) {
		rec := doComments(t, app, http.MethodPost,
			"/api/v1/_comments/ticket/TKT-001", addBody, "alice@example.com")
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), "read-only")
	})

	t.Run("update refused", func(t *testing.T) {
		rec := doComments(t, app, http.MethodPatch,
			"/api/v1/_comments/ticket/TKT-001/abc", `{"body":"x"}`, "alice@example.com")
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("delete refused", func(t *testing.T) {
		rec := doComments(t, app, http.MethodDelete,
			"/api/v1/_comments/ticket/TKT-001/abc", "", "alice@example.com")
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("reads still work", func(t *testing.T) {
		rec := doComments(t, app, http.MethodGet,
			"/api/v1/_comments/ticket/TKT-001", "", "alice@example.com")
		require.Equal(t, http.StatusOK, rec.Code)
	})
}
