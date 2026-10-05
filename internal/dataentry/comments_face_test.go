package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// commentFaceBody is the markdown seeded on every face of POL-1, so a
// suggestion's quote matches whichever face it lands on.
const commentFaceBody = "The retention period for audit records is five years.\n"

// rankedPolicyWorld ranks published above draft, so a world-ranked
// resolution of a bare POL-1 would pick the published face.
func rankedPolicyWorld() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
}

// facedCommentsApp is facedAppWith over facedMeta with commenting enabled on
// the faced `policy` and the faceless `feature`. POL-1 has a body on both
// faces and POL-2 exists only as a draft.
func facedCommentsApp(t *testing.T, policy *acl.Policy) (*App, *acl.Declarative) {
	t.Helper()
	meta := facedMeta(t)
	meta.Comments = &metamodel.CommentsConfig{Enabled: true, On: []string{"policy", "feature"}}
	var mkACL func(store.Store) *acl.Declarative
	if policy != nil {
		mkACL = func(st store.Store) *acl.Declarative { return mustNewACL(t, policy, st) }
	}
	app, d := facedAppWith(t, meta, rankedPolicyWorld(), mkACL)
	for _, f := range []entity.Face{"draft", "published"} {
		e, err := app.store.GetEntity(t.Context(), entity.Ref{ID: "POL-1", Face: f})
		require.NoError(t, err)
		e.Content = commentFaceBody
		require.NoError(t, app.store.UpdateEntity(t.Context(), e))
	}
	require.NoError(t, app.store.CreateEntity(t.Context(), &entity.Entity{
		ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "only a draft"},
	}))
	svc, err := comments.NewService(memcomments.New(), nil)
	require.NoError(t, err)
	app.SetComments(svc)
	return app, d
}

// doFacedComments runs a comments request as alice in the ranked world, the
// world attachWorld stamps for this deployment. Under d it carries the
// request-scoped ACL state the production middleware attaches.
func doFacedComments(
	t *testing.T, app *App, d *acl.Declarative, method, path, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	if d != nil {
		ctx = gateCtxFor(ctx, t, d)
	}
	ctx = withWorld(ctx, worldHandle{name: "published", scope: rankedPolicyWorld()})
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.comments.handleV1Comments(rec, req)
	return rec
}

// TestComments_WriteTargetsTheNamedFace pins that a comment write lands on
// the face the request names, never on the face the world ranks first, and
// that a bare id on a faced type is face_required (root CLAUDE.md: "Don't
// pick a face for a bare-id write by rank").
func TestComments_WriteTargetsTheNamedFace(t *testing.T) {
	const note = `{"anchor":{"kind":"property","ref":"title"},"body":"check this"}`
	readPublishedOnly := &acl.Policy{
		Roles: map[string]acl.RoleDef{"viewer": {
			Read:        []string{"policy@published", "feature"},
			Permissions: []string{"comment:read", "comment:add"},
		}},
		Assignments: map[string]string{"alice": "viewer"},
	}
	for _, tc := range []struct {
		name      string
		policy    *acl.Policy
		method    string
		path      string
		body      string
		wantCode  int
		wantFaces []string
	}{
		{name: "named lower-ranked face", method: http.MethodPost,
			path: "/api/v1/_comments/policy/POL-1@draft", body: note, wantCode: http.StatusCreated},
		{name: "bare id with two readable faces", method: http.MethodPost,
			path: "/api/v1/_comments/policy/POL-1", body: note,
			wantCode: http.StatusUnprocessableEntity, wantFaces: []string{"POL-1@draft", "POL-1@published"}},
		// A faced type has no implicit face, so even a single face must be
		// named: a write that works today must not break when a second face
		// is published.
		{name: "bare id with one face", method: http.MethodPost,
			path: "/api/v1/_comments/policy/POL-2", body: note,
			wantCode: http.StatusUnprocessableEntity, wantFaces: []string{"POL-2@draft"}},
		{name: "bare id with one readable face", policy: readPublishedOnly, method: http.MethodPost,
			path: "/api/v1/_comments/policy/POL-1", body: note,
			wantCode: http.StatusUnprocessableEntity, wantFaces: []string{"POL-1@published"}},
		{name: "bare id of a faceless type", method: http.MethodPost,
			path: "/api/v1/_comments/feature/FEAT-1", body: note, wantCode: http.StatusCreated},
		{name: "named face the principal may not read", policy: readPublishedOnly, method: http.MethodPost,
			path: "/api/v1/_comments/policy/POL-1@draft", body: note, wantCode: http.StatusNotFound},
		{name: "readable named face under the same policy", policy: readPublishedOnly, method: http.MethodPost,
			path: "/api/v1/_comments/policy/POL-1@published", body: note, wantCode: http.StatusCreated},
		{name: "update on a bare id", method: http.MethodPatch,
			path: "/api/v1/_comments/policy/POL-1/c1", body: `{"resolved":true}`,
			wantCode: http.StatusUnprocessableEntity, wantFaces: []string{"POL-1@draft", "POL-1@published"}},
		{name: "delete on a bare id", method: http.MethodDelete,
			path:     "/api/v1/_comments/policy/POL-1/c1",
			wantCode: http.StatusUnprocessableEntity, wantFaces: []string{"POL-1@draft", "POL-1@published"}},
		{name: "accept on a bare id", method: http.MethodPost,
			path:     "/api/v1/_comments/policy/POL-1/c1/accept",
			wantCode: http.StatusUnprocessableEntity, wantFaces: []string{"POL-1@draft", "POL-1@published"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := facedCommentsApp(t, tc.policy)
			rec := doFacedComments(t, app, d, tc.method, tc.path, tc.body)
			require.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
			if tc.wantFaces == nil {
				return
			}
			var problem v1.Error
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
			require.True(t, strings.HasSuffix(problem.Type, "/face_required"), problem.Type)
			require.Equal(t, tc.wantFaces, problem.Faces)
		})
	}
}

// TestComments_AcceptOnNamedFaceWritesThatFace pins that accepting a
// suggestion on POL-1@draft edits the draft, although the request's world
// ranks the published face first.
func TestComments_AcceptOnNamedFaceWritesThatFace(t *testing.T) {
	app, _ := facedCommentsApp(t, nil)
	const draftPath = "/api/v1/_comments/policy/POL-1@draft"

	rec := doFacedComments(t, app, nil, http.MethodPost, draftPath, suggestionJSON("five years", "seven years"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var c commentWire
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &c))

	rec = doFacedComments(t, app, nil, http.MethodPost, draftPath+"/"+c.ID+"/accept", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	draft, err := app.store.GetEntity(t.Context(), entity.Ref{ID: "POL-1", Face: "draft"})
	require.NoError(t, err)
	require.Contains(t, draft.Content, "seven years")
	published, err := app.store.GetEntity(t.Context(), entity.Ref{ID: "POL-1", Face: "published"})
	require.NoError(t, err)
	require.Equal(t, commentFaceBody, published.Content, "the higher-ranked face must be untouched")
}
