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
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const conversionHandlerSchema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: manual
    faces: {draft: {label: Draft}, published: {label: Published}}
    properties:
      title: {type: string, required: true}
      body: {type: string}
  document:
    label: Document
    id_prefix: "DOC-"
    id_type: manual
    faces: {draft: {label: Draft}, published: {label: Published}}
    properties:
      title: {type: string, required: true}
      summary: {type: string}
      editor_note: {type: string}
      code: {type: string, unique: true}
  vault:
    label: Vault
    id_prefix: "VAULT-"
    id_type: manual
    properties: {title: {type: string}}
relations:
  stored-in:
    label: Stored in
    from: [note, document]
    to: [vault]
conversions:
  note-to-document:
    from: note
    to: document
    fields:
      title: "{{source.title}}"
      summary: "{{source.body}}"
    relations: {stored-in: drop}
    faces:
      draft: [draft]
      published: [published]
`

// conversionHandlerPolicy: nobody but admin reads vaults, so an edge to one
// is hidden from every other role.
const conversionHandlerPolicy = `
roles:
  reader:
    read: [note, document]
  converter:
    read: [note, document]
    convert: [note]
  editor:
    read: [note, document]
    delete: ["note@draft", "note@published"]
    convert: [note]
  drafter:
    read: ["note@draft", document]
    convert: [note]
  redacted:
    read: [note, document]
    convert: [note]
    visible:
      note: [{field: title}]
assignments:
  reader: reader
  converter: converter
  editor: editor
  drafter: drafter
  redacted: redacted
`

type conversionHandlerFixture struct {
	app *App
	st  store.Store
	d   *acl.Declarative
}

func newConversionHandlerFixture(t *testing.T) conversionHandlerFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(conversionHandlerSchema))
	require.NoError(t, err)
	st := memstore.New()
	for _, face := range []string{"draft", "published"} {
		e := entity.New("NOTE-1", "note")
		e.Face = entity.Face(face)
		e.SetString("title", "Title "+face)
		e.SetString("body", "Body "+face)
		e.Content = "Original " + face
		require.NoError(t, st.CreateEntity(t.Context(), e))
	}
	hidden := entity.New("NOTE-2", "note")
	hidden.Face = "published"
	hidden.SetString("title", "Hidden")
	require.NoError(t, st.CreateEntity(t.Context(), hidden))
	policy, err := acl.LoadPolicyBytes([]byte(conversionHandlerPolicy))
	require.NoError(t, err)
	d, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st)
	require.NoError(t, err)
	app := newAppFromParts(&Config{App: AppConfig{Name: "Conversion test"}}, meta, newFixture(),
		appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	require.NotNil(t, app.conversions)
	return conversionHandlerFixture{app: app, st: st, d: d}
}

func (f conversionHandlerFixture) send(t *testing.T, user, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
	ctx = gateCtxFor(ctx, t, f.d)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_conversions/note-to-document"+path,
		strings.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	f.app.conversions.handleV1Conversions(rec, req)
	return rec
}

const conversionHandlerBody = `{"source_id":"NOTE-1","source_face":"draft","target_id":"DOC-1",` +
	`"faces":{"draft":"draft","published":"published"},"properties":{"editor_note":"draft-only"},` +
	`"content":"Edited draft"}`

// hiddenNoteBody converts NOTE-2, stored only at a face the drafter cannot
// read; absentNoteBody converts an id nothing stores. The two must answer
// alike.
var (
	hiddenNoteBody = `{"source_id":"NOTE-2","target_id":"DOC-2"}`
	absentNoteBody = `{"source_id":"NOTE-404","target_id":"DOC-2"}`
)

func TestConversionHandler_Refusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, user, body string
		want             int
		// absent is text the response must not contain.
		absent string
	}{
		{"visible source without convert grant", "reader", conversionHandlerBody, http.StatusForbidden, ""},
		{"no readable face", "drafter", hiddenNoteBody, http.StatusNotFound, ""},
		{"absent source", "converter", absentNoteBody, http.StatusNotFound, ""},
		{"a face is unreadable", "drafter", conversionHandlerBody, http.StatusUnprocessableEntity, "published"},
		{"a mapped field is unreadable", "redacted", conversionHandlerBody, http.StatusUnprocessableEntity, ""},
		{"source face not stored", "converter",
			strings.Replace(conversionHandlerBody, `"source_face":"draft"`, `"source_face":"archived"`, 1),
			http.StatusUnprocessableEntity, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newConversionHandlerFixture(t)
			for _, path := range []string{"/preflight", ""} {
				rec := f.send(t, tc.user, path, tc.body)
				require.Equal(t, tc.want, rec.Code, "%s: %s", path, rec.Body.String())
				if tc.absent != "" {
					require.NotContains(t, rec.Body.String(), tc.absent)
				}
			}
			_, err := f.st.GetEntity(t.Context(), entity.Ref{ID: "NOTE-1", Face: "draft"})
			require.NoError(t, err, "a refused conversion left the source in place")
		})
	}
}

func TestConversionHandler_ConvertsFaceScopedValues(t *testing.T) {
	t.Parallel()
	f := newConversionHandlerFixture(t)
	rec := f.send(t, "converter", "/preflight", conversionHandlerBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var preview v1.ConversionPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Equal(t, []v1.ConversionMove{
		{SourceFace: "draft", TargetFace: "draft"}, {SourceFace: "published", TargetFace: "published"},
	}, preview.Moves)

	rec = f.send(t, "converter", "", conversionHandlerBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	draft, err := f.st.GetEntity(t.Context(), entity.Ref{ID: "DOC-1", Face: "draft"})
	require.NoError(t, err)
	published, err := f.st.GetEntity(t.Context(), entity.Ref{ID: "DOC-1", Face: "published"})
	require.NoError(t, err)
	require.Equal(t, "Edited draft", draft.Content)
	require.Equal(t, "draft-only", draft.GetString("editor_note"))
	require.Equal(t, "Original published", published.Content)
	require.Empty(t, published.GetString("editor_note"))
	require.Equal(t, "Body published", published.GetString("summary"), "operator mapping still applies to every face")
}

// A relation the caller cannot see is never named. Dropping it is a
// relation delete, so it needs the grant a delete of that edge needs; a
// caller without it is refused with the generic one-bit answer.
func TestConversionHandler_HiddenRelationIsDroppedUnnamed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		user string
		want int
	}{
		{"editor", http.StatusOK},
		{"converter", http.StatusForbidden},
	} {
		t.Run(tc.user, func(t *testing.T) {
			t.Parallel()
			f := newConversionHandlerFixture(t)
			require.NoError(t, f.st.CreateEntity(t.Context(), entity.New("VAULT-1", "vault")))
			key := entity.RelationKey{From: "NOTE-1", FromFace: "draft", Type: "stored-in", To: "VAULT-1"}
			_, err := f.st.CreateRelation(t.Context(), key, &store.RelationData{})
			require.NoError(t, err)

			rec := f.send(t, tc.user, "/preflight", conversionHandlerBody)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			for _, rec := range []*httptest.ResponseRecorder{rec, f.send(t, tc.user, "", conversionHandlerBody)} {
				require.NotContains(t, rec.Body.String(), "stored-in")
				require.NotContains(t, rec.Body.String(), "VAULT-1")
				if rec.Code != http.StatusOK {
					require.Equal(t, tc.want, rec.Code, rec.Body.String())
					return
				}
			}
			require.Equal(t, tc.want, http.StatusOK)
			_, err = f.st.GetRelation(t.Context(),
				entity.RelationKey{From: "DOC-1", FromFace: "draft", Type: "stored-in", To: "VAULT-1"})
			require.Error(t, err, "the hidden relation was dropped")
		})
	}
}

func TestConversionHandler_UniqueConflictIs422(t *testing.T) {
	t.Parallel()
	f := newConversionHandlerFixture(t)
	holder := entity.New("DOC-9", "document")
	holder.Face = "draft"
	holder.SetString("title", "Holder")
	holder.SetString("code", "C-1")
	require.NoError(t, f.st.CreateEntity(t.Context(), holder))
	body := strings.Replace(conversionHandlerBody, `"editor_note":"draft-only"`, `"code":"C-1"`, 1)
	rec := f.send(t, "converter", "", body)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	_, err := f.st.GetEntity(t.Context(), entity.Ref{ID: "NOTE-1", Face: "draft"})
	require.NoError(t, err)
}
