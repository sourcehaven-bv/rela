package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const taskItemHandlerSchema = `version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    id_type: manual
    properties:
      title: {type: string, required: true}
      project: {type: string}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      project: {type: string}
      owner: {type: string, required: true}
relations:
  has-subtask:
    label: Has subtask
    from: [ticket]
    to: [task]
    max_outgoing: 1
task_item_conversions:
  subtask:
    from: ticket
    to: task
    label: Convert to task
    item: remove
    relation: {type: has-subtask, as: to}
    fields:
      title: "{{item.text}}"
      project: "{{source.project}}"
`

// taskItemHandlerPolicy: viewer reads but may not convert; blind cannot read
// tickets at all.
const taskItemHandlerPolicy = `
roles:
  editor:
    read: [ticket, task]
    create: [ticket, task]
    update: [ticket, task]
  viewer:
    read: [ticket, task]
  blind:
    read: [task]
    create: [task]
assignments:
  editor: editor
  viewer: viewer
  blind: blind
`

type taskItemHandlerFixture struct {
	app *App
	st  store.Store
	d   *acl.Declarative
}

func newTaskItemHandlerFixture(t *testing.T) taskItemHandlerFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(taskItemHandlerSchema))
	require.NoError(t, err)
	st := memstore.New()
	src := entity.New("TKT-1", "ticket")
	src.SetString("title", "Parent")
	src.SetString("project", "apollo")
	src.Content = "- [ ] Write the migration\n"
	require.NoError(t, st.CreateEntity(t.Context(), src))
	policy, err := acl.LoadPolicyBytes([]byte(taskItemHandlerPolicy))
	require.NoError(t, err)
	d, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st)
	require.NoError(t, err)
	app := newAppFromParts(&Config{App: AppConfig{Name: "Task item test"}}, meta, newFixture(),
		appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	require.NotNil(t, app.taskItems)
	return taskItemHandlerFixture{app: app, st: st, d: d}
}

func (f taskItemHandlerFixture) ctx(t *testing.T, user string) context.Context {
	t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
	return gateCtxFor(ctx, t, f.d)
}

func (f taskItemHandlerFixture) send(t *testing.T, user, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/_task_item_conversions/"+path,
		strings.NewReader(body)).WithContext(f.ctx(t, user))
	rec := httptest.NewRecorder()
	f.app.taskItems.handleV1TaskItemConversions(rec, req)
	return rec
}

func TestTaskItemHandler_Converts(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	body := `{"source_id":"TKT-1","text":"Write the migration","properties":{"owner":"bob"}}`
	rec := f.send(t, "editor", http.MethodPost, "subtask/preflight", `{"source_id":"TKT-1","text":"x"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var preview v1.TaskItemConversionPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Equal(t, v1.TaskItemConversionPreview{TargetType: "task", Required: []string{"owner"}}, preview)

	rec = f.send(t, "editor", http.MethodPost, "subtask", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res v1.TaskItemConversionResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	require.Equal(t, "task", res.EntityType)
	require.Equal(t, "Write the migration", res.Title)
	require.Equal(t, metamodel.TaskItemRemove, res.Item)
	created, err := f.st.GetEntity(t.Context(), entity.Ref{ID: res.EntityID})
	require.NoError(t, err)
	require.Equal(t, "apollo", created.GetString("project"))
	_, err = f.st.GetRelation(t.Context(), entity.RelationKey{From: "TKT-1", Type: "has-subtask", To: res.EntityID})
	require.NoError(t, err)
}

// The title in the result is read back through the read gate, so a caller
// who may not read the new entity's title gets none.
func TestTaskItemHandler_TitleIsGated(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	setTestResolver(t, f.app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}})
	rec := f.send(t, "editor", http.MethodPost, "subtask",
		`{"source_id":"TKT-1","text":"Secret plan","properties":{"owner":"bob"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res v1.TaskItemConversionResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	require.NotEmpty(t, res.EntityID)
	require.NotContains(t, rec.Body.String(), "Secret plan")
}

func TestTaskItemHandler_Refusals(t *testing.T) {
	t.Parallel()
	valid := `{"source_id":"TKT-1","text":"x","properties":{"owner":"bob"}}`
	for _, tc := range []struct {
		name, user, method, path, body string
		want                           int
	}{
		{"wrong method", "editor", http.MethodGet, "subtask", "", http.StatusMethodNotAllowed},
		{"unknown recipe", "editor", http.MethodPost, "nope", valid, http.StatusNotFound},
		{"nested path", "editor", http.MethodPost, "subtask/x/preflight", valid, http.StatusNotFound},
		{"bad json", "editor", http.MethodPost, "subtask", "{", http.StatusBadRequest},
		{"no source", "editor", http.MethodPost, "subtask", `{"text":"x"}`, http.StatusBadRequest},
		{"no rights", "viewer", http.MethodPost, "subtask", valid, http.StatusForbidden},
		{"empty text", "editor", http.MethodPost, "subtask", `{"source_id":"TKT-1","text":"  "}`,
			http.StatusUnprocessableEntity},
		{"control character", "editor", http.MethodPost, "subtask",
			`{"source_id":"TKT-1","text":"a\u0007b","properties":{"owner":"bob"}}`, http.StatusUnprocessableEntity},
		{"mapped field supplied", "editor", http.MethodPost, "subtask",
			`{"source_id":"TKT-1","text":"x","properties":{"owner":"bob","project":"other"}}`,
			http.StatusUnprocessableEntity},
		{"required field missing", "editor", http.MethodPost, "subtask", `{"source_id":"TKT-1","text":"x"}`,
			http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskItemHandlerFixture(t)
			rec := f.send(t, tc.user, tc.method, tc.path, tc.body)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			for _, err := range f.st.ListEntities(t.Context(), store.EntityQuery{Type: "task", Faces: store.AllFaces()}) {
				require.NoError(t, err)
				t.Fatal("a refused request created a task")
			}
		})
	}
}

// A relation at its limit is a 422 naming the bound, not a 500, and leaves
// no entity behind.
func TestTaskItemHandler_CardinalityIs422(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	body := `{"source_id":"TKT-1","text":"x","properties":{"owner":"bob"}}`
	rec := f.send(t, "editor", http.MethodPost, "subtask", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.send(t, "editor", http.MethodPost, "subtask", body)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "cardinality_exceeded")
	tasks := 0
	for _, err := range f.st.ListEntities(t.Context(), store.EntityQuery{Type: "task", Faces: store.AllFaces()}) {
		require.NoError(t, err)
		tasks++
	}
	require.Equal(t, 1, tasks)
}

// A hidden source answers exactly like an absent one.
func TestTaskItemHandler_HiddenSourceIsUniform404(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	for _, path := range []string{"subtask/preflight", "subtask"} {
		hidden := f.send(t, "blind", http.MethodPost, path, `{"source_id":"TKT-1","text":"x"}`)
		absent := f.send(t, "blind", http.MethodPost, path, `{"source_id":"TKT-404","text":"x"}`)
		require.Equal(t, http.StatusNotFound, hidden.Code, hidden.Body.String())
		require.Equal(t, absent.Code, hidden.Code)
		require.Equal(t, absent.Body.String(), hidden.Body.String())
	}
}

// staleTaskItemConverter holds a recipe that differs from the schema the
// handler serves, as after a reload.
type staleTaskItemConverter struct{ taskItemConverter }

func (staleTaskItemConverter) Definition(string) (metamodel.TaskItemConversionDef, bool) {
	return metamodel.TaskItemConversionDef{Label: "old"}, true
}

func TestTaskItemHandler_SchemaChanged(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	h, err := newTaskItemConversionHandler(staleTaskItemConverter{}, f.app.State, viewMention(f.app))
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_task_item_conversions/subtask",
		strings.NewReader(`{"source_id":"TKT-1","text":"x"}`)).WithContext(f.ctx(t, "editor"))
	rec := httptest.NewRecorder()
	h.handleV1TaskItemConversions(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "schema_changed")
}

// scriptedTaskItemConverter answers Convert with a fixed result or error,
// and everything else as the real converter does.
type scriptedTaskItemConverter struct {
	taskItemConverter
	res *entitymanager.TaskItemResult
	err error
}

func (c scriptedTaskItemConverter) Convert(
	context.Context, entitymanager.TaskItemRequest,
) (*entitymanager.TaskItemResult, error) {
	return c.res, c.err
}

func (f taskItemHandlerFixture) sendTo(t *testing.T, conv taskItemConverter, body string) *httptest.ResponseRecorder {
	t.Helper()
	h, err := newTaskItemConversionHandler(conv, f.app.State, viewMention(f.app))
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_task_item_conversions/subtask",
		strings.NewReader(body)).WithContext(f.ctx(t, "editor"))
	rec := httptest.NewRecorder()
	h.handleV1TaskItemConversions(rec, req)
	return rec
}

// TestTaskItemHandler_CreateErrors pins the answer to each create error the
// conversion can return. A refusal is never a 500, and a store error's text
// never reaches the wire.
func TestTaskItemHandler_CreateErrors(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	kept := &entitymanager.KeptEntityError{
		Entity: entity.New("TASK-9", "task"), Err: errors.New("relations disk full"),
	}
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
		want   string
	}{
		{"illegal entry", fmt.Errorf("status: %w", statemachine.ErrIllegalEntry),
			http.StatusUnprocessableEntity, "conversion_failed", ""},
		{"owning rule", fmt.Errorf("%w: owned twice", entitymanager.ErrOwningRule),
			http.StatusUnprocessableEntity, "conversion_failed", ""},
		{"relation exists", entitymanager.ErrRelationAlreadyExists, http.StatusConflict, "conflict", ""},
		{"face", entitymanager.ErrFaceNotDeclared, http.StatusUnprocessableEntity, "conversion_failed", ""},
		{"kept entity", kept, http.StatusInternalServerError, "conversion_incomplete", `"entity_id":"TASK-9"`},
		{"store error", errors.New("disk full"), http.StatusInternalServerError, "conversion_failed", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := f.sendTo(t, scriptedTaskItemConverter{taskItemConverter: f.app.taskItems.conv, err: tc.err},
				`{"source_id":"TKT-1","text":"x"}`)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "/errors/"+tc.code+`"`)
			require.NotContains(t, rec.Body.String(), "disk full")
			if tc.want != "" {
				require.Contains(t, rec.Body.String(), tc.want)
			}
		})
	}
}

// TestTaskItemHandler_FollowUpFailureIsAWarning pins that a failure after
// the commit answers 200 with the created entity, so the editor still
// updates the item instead of inviting a retry that would make a second
// entity.
func TestTaskItemHandler_FollowUpFailureIsAWarning(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	created := entity.New("TASK-9", "task")
	soft := entity.Warning{Code: "required_property_unset", Detail: "stage"}
	res := &entitymanager.TaskItemResult{
		Definition:  "subtask",
		Entity:      created,
		Create:      &entity.CreateResult{Entity: created, Warnings: []entity.Warning{soft}},
		FollowUpErr: errors.New("cascade failed"),
	}
	rec := f.sendTo(t, scriptedTaskItemConverter{taskItemConverter: f.app.taskItems.conv, res: res},
		`{"source_id":"TKT-1","text":"x"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out v1.TaskItemConversionResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, created.ID, out.EntityID)
	codes := make([]string, 0, len(out.Warnings))
	for _, w := range out.Warnings {
		codes = append(codes, w.Code)
	}
	require.Equal(t, []string{soft.Code, warnTaskItemFollowUpFailed}, codes)
	require.NotContains(t, rec.Body.String(), "cascade failed")
}

// TestTaskItemHandler_BodyIsBounded pins that an over-large body is refused
// before it is decoded.
func TestTaskItemHandler_BodyIsBounded(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	body := `{"source_id":"TKT-1","text":"` + strings.Repeat("x", taskItemBodyLimit) + `"}`
	rec := f.send(t, "editor", http.MethodPost, "subtask", body)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestNewTaskItemConversionHandler_RejectsNil(t *testing.T) {
	t.Parallel()
	conv := staleTaskItemConverter{}
	schema := func() *Schema { return nil }
	mention := func(context.Context, *metamodel.Metamodel, entity.Ref) (v1.Mention, bool) { return v1.Mention{}, false }
	for name, call := range map[string]func() error{
		"converter": func() error { _, err := newTaskItemConversionHandler(nil, schema, mention); return err },
		"schema":    func() error { _, err := newTaskItemConversionHandler(conv, nil, mention); return err },
		"mention":   func() error { _, err := newTaskItemConversionHandler(conv, schema, nil); return err },
	} {
		require.Error(t, call(), name)
	}
}

func TestTaskItemOffers_RideTheEntityResponse(t *testing.T) {
	t.Parallel()
	f := newTaskItemHandlerFixture(t)
	src, err := f.st.GetEntity(t.Context(), entity.Ref{ID: "TKT-1"})
	require.NoError(t, err)
	for _, tc := range []struct {
		user    string
		allowed bool
	}{
		{"editor", true},
		{"viewer", false},
	} {
		var result v1.Entity
		f.app.affordances.attachEntityAffordances(f.ctx(t, tc.user), src, &result)
		require.NotNil(t, result.TaskItemConversions, tc.user)
		offers := *result.TaskItemConversions
		require.Len(t, offers, 1)
		require.Equal(t, "subtask", offers[0].Name)
		require.Equal(t, metamodel.TaskItemRemove, offers[0].Item)
		require.Equal(t, tc.allowed, offers[0].Allowed, "%s: %s", tc.user, offers[0].Reason)
	}
	task := entity.New("TASK-9", "task")
	var result v1.Entity
	f.app.affordances.attachEntityAffordances(f.ctx(t, "editor"), task, &result)
	require.Nil(t, result.TaskItemConversions, "a type no recipe starts from carries no offers")
}

var _ taskItemConverter = (*entitymanager.TaskItemConverter)(nil)
