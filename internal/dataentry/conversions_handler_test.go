package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestConversionHandlerACLAndFaceScopedOverrides(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"note": {
				Label: "Note", IDPrefix: "NOTE-", IDType: metamodel.IDTypeManual,
				Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true}, "body": {Type: "string"},
				},
			},
			"document": {
				Label: "Document", IDPrefix: "DOC-", IDType: metamodel.IDTypeManual,
				Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true}, "summary": {Type: "string"}, "editor_note": {Type: "string"},
				},
			},
		},
		Conversions: map[string]metamodel.ConversionDef{
			"note-to-document": {
				From: "note", To: "document",
				Fields: map[string]string{"title": "{{source.title}}", "summary": "{{source.body}}"},
				Faces:  map[string][]string{"draft": {"draft"}, "published": {"published"}},
			},
		},
	}
	st := memstore.New()
	for _, row := range []struct {
		face, title, body, content string
	}{
		{"draft", "Draft title", "Draft summary", "Original draft"},
		{"published", "Published title", "Published summary", "Original published"},
	} {
		e := entity.New("NOTE-1", "note")
		e.Face = entity.Face(row.face)
		e.SetString("title", row.title)
		e.SetString("body", row.body)
		e.Content = row.content
		require.NoError(t, st.CreateEntity(t.Context(), e))
	}
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reader":    {Read: []string{"note@draft"}},
			"converter": {Read: []string{"note@draft"}, Convert: []string{"note"}},
			"hidden":    {Convert: []string{"note"}},
		},
		Assignments: map[string]string{"reader": "reader", "converter": "converter", "hidden": "hidden"},
	}
	d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(st), st)
	require.NoError(t, err)
	app := newAppFromParts(&Config{App: AppConfig{Name: "Conversion test"}}, meta, newFixture(),
		appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	manager, ok := app.write.manager.(*entitymanager.Manager)
	require.True(t, ok, "test fixture exposes the concrete entity manager")
	app.conversions = newConversionHandler(manager)
	app.conversions.schema = app.State
	app.conversions.validateFields = func(ctx context.Context, e *entity.Entity, set map[string]any) error {
		if denial := app.affordances.validateCreateFieldWrite(ctx, e, set); denial != nil {
			return denial
		}
		return nil
	}

	send := func(user, body string) *httptest.ResponseRecorder {
		t.Helper()
		ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
		ctx = gateCtxFor(ctx, t, d)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/_conversions/note-to-document", strings.NewReader(body)).WithContext(ctx)
		rec := httptest.NewRecorder()
		app.conversions.handleV1Conversions(rec, req)
		return rec
	}
	body := `{"source_id":"NOTE-1","source_face":"draft","target_id":"DOC-1","faces":{"draft":"draft","published":"published"},"properties":{"editor_note":"draft-only"},"content":"Edited draft"}`

	require.Equal(t, http.StatusForbidden, send("reader", body).Code, "visible source without convert grant")
	require.Equal(t, http.StatusNotFound, send("hidden", body).Code, "convert grant without any readable source face")
	hiddenFaceBody := strings.Replace(body, `"source_face":"draft"`, `"source_face":"published"`, 1)
	require.Equal(t, http.StatusNotFound, send("converter", hiddenFaceBody).Code,
		"client-authored values cannot target a face the caller cannot read")
	rec := send("converter", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	draft, err := st.GetEntity(t.Context(), entity.Ref{ID: "DOC-1", Face: "draft"})
	require.NoError(t, err)
	published, err := st.GetEntity(t.Context(), entity.Ref{ID: "DOC-1", Face: "published"})
	require.NoError(t, err)
	require.Equal(t, "Edited draft", draft.Content)
	require.Equal(t, "draft-only", draft.GetString("editor_note"))
	require.Equal(t, "Original published", published.Content)
	require.Empty(t, published.GetString("editor_note"))
	require.Equal(t, "Published summary", published.GetString("summary"), "operator mapping still applies to every face")
}
