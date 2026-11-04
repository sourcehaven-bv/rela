package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newBatchTestApp is newTestAppV1 with the batch endpoint served. The
// fixture store does not roll back, so the tests below that need a real
// rollback live in batch_handler_sqlite_test.go; these cover the request
// handling, which does not depend on it.
func newBatchTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	app.batch.atomic = true
	return app
}

// newBatchACLApp builds the newTestAppV1 metamodel over a store whose
// manager enforces p, as TestHandler_ACLDeny_Returns403Structured does.
func newBatchACLApp(t *testing.T, p *acl.Policy, seed ...*entity.Entity) (*App, *acl.Declarative) {
	t.Helper()
	meta := newTestAppV1(t).Meta()
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	require.NoError(t, fs.MkdirAll(paths.CacheDir, 0o755))
	bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
	for _, e := range seed {
		require.NoError(t, bootstrap.Store().CreateEntity(context.Background(), e))
	}
	d := mustNewACL(t, p, bootstrap.Store())
	svc := appbuildtest.New(meta,
		appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(bootstrap.Store()),
		appbuildtest.WithDeclarative(d),
	)
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, d)
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	app.broker = newEventBroker()
	app.batch.atomic = true
	return app, d
}

func postBatch(ctx context.Context, t *testing.T, app *App, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, batchPath, strings.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.batch.handleV1Batch(rec, req)
	return rec
}

// postBatchAs posts body as alice, bound to d.
func postBatchAs(t *testing.T, app *App, d *acl.Declarative, body string) *httptest.ResponseRecorder {
	t.Helper()
	return postBatch(gateCtxFor(aliceCtx(), t, d), t, app, body)
}

func decodeBatchError(t *testing.T, rec *httptest.ResponseRecorder) v1.Error {
	t.Helper()
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"), rec.Body.String())
	var e v1.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e), rec.Body.String())
	return e
}

// countOf counts the stored entities of typ.
func countOf(t *testing.T, st store.Store, typ string) int {
	t.Helper()
	n := 0
	for e, err := range st.ListEntities(context.Background(),
		store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}) {
		require.NoError(t, err)
		if e.Type == typ {
			n++
		}
	}
	return n
}

func countRelations(t *testing.T, st store.Store) int {
	t.Helper()
	n := 0
	for _, err := range st.ListRelations(context.Background(), store.RelationQuery{}) {
		require.NoError(t, err)
		n++
	}
	return n
}

func TestBatch_UnsupportedStoreIs501(t *testing.T) {
	app := newTestAppV1(t)
	rec := postBatch(context.Background(), t, app, `{"entities":[{"type":"ticket","properties":{"title":"a"}}]}`)
	require.Equal(t, http.StatusNotImplemented, rec.Code, rec.Body.String())
	require.Equal(t, "https://rela.dev/errors/atomic_batch_unsupported", decodeBatchError(t, rec).Type)
	require.Zero(t, countOf(t, app.store, "ticket"))

	sch := httptest.NewRecorder()
	app.handleV1Schema(sch, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	var got v1.Schema
	require.NoError(t, json.Unmarshal(sch.Body.Bytes(), &got))
	require.False(t, got.Capabilities.AtomicBatch)
	require.Contains(t, sch.Body.String(), `"capabilities":{"atomic_batch":false}`)
}

// TestBatch_CrossOriginRefused: a cookie-bearing request keeps the
// same-origin check; the shape a script sends (no cookie, no Origin)
// reaches the handler, which on the fixture store answers 501.
func TestBatch_CrossOriginRefused(t *testing.T) {
	h := newSecuredTestApp(t).NewRouter()
	send := func(hdr map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, batchPath, strings.NewReader(`{}`))
		r.Host = "127.0.0.1:8080"
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := send(map[string]string{"Origin": "https://evil.example", "Cookie": "session=abc"}); w.Code != http.StatusForbidden {
		t.Errorf("cross-origin = %d %s, want 403", w.Code, w.Body)
	}
	if w := send(map[string]string{"Cookie": "session=abc"}); w.Code != http.StatusForbidden {
		t.Errorf("origin-less cookie = %d %s, want 403", w.Code, w.Body)
	}
	if w := send(nil); w.Code != http.StatusNotImplemented {
		t.Errorf("script request = %d %s, want 501", w.Code, w.Body)
	}
	require.True(t, isCSRFExempt(httptest.NewRequest(http.MethodPost, batchPath, http.NoBody)))
}

func TestBatch_MethodNotAllowed(t *testing.T) {
	app := newBatchTestApp(t)
	req := httptest.NewRequest(http.MethodGet, batchPath, http.NoBody)
	rec := httptest.NewRecorder()
	app.batch.handleV1Batch(rec, req)
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, http.MethodPost, rec.Header().Get("Allow"))
}

func TestBatch_CreatesEntitiesAndRelations(t *testing.T) {
	app := newBatchTestApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "old"}})

	rec := postBatch(context.Background(), t, app, `{
		"entities": [
			{"ref": "t", "type": "ticket", "properties": {"title": "new ticket"}},
			{"ref": "f", "type": "feature", "properties": {"title": "new feature"}}
		],
		"relations": [
			{"from": {"ref": "t"}, "type": "implements", "to": {"ref": "f"}},
			{"from": {"id": "TKT-001"}, "type": "blocks", "to": {"ref": "t"}}
		]
	}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got v1.BatchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Entities, 2)
	ticket, feature := got.Entities[0], got.Entities[1]
	require.Equal(t, "t", ticket.Ref)
	require.True(t, strings.HasPrefix(ticket.ID, "TKT-"), ticket.ID)
	require.NotEqual(t, "TKT-001", ticket.ID)
	require.True(t, strings.HasPrefix(feature.ID, "FEAT-"), feature.ID)
	require.NotNil(t, ticket.Entity)
	require.Equal(t, "new ticket", ticket.Entity.Properties["title"])

	require.Equal(t, []v1.BatchRelation{
		{From: v1.BatchEnd{ID: ticket.ID}, Type: "implements", To: v1.BatchEnd{ID: feature.ID}},
		{From: v1.BatchEnd{ID: "TKT-001"}, Type: "blocks", To: v1.BatchEnd{ID: ticket.ID}},
	}, got.Relations)
	require.Equal(t, 2, countOf(t, app.store, "ticket"))
	require.Equal(t, 1, countOf(t, app.store, "feature"))
	require.Equal(t, 2, countRelations(t, app.store))
}

func TestBatch_RequestLimits(t *testing.T) {
	many := make([]string, entityLimitPlusOne())
	for i := range many {
		many[i] = `{"type":"ticket","properties":{"title":"x"}}`
	}
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"empty batch", `{}`, http.StatusBadRequest, "batch_invalid"},
		{"over the operation limit", `{"entities":[` + strings.Join(many, ",") + `]}`,
			http.StatusBadRequest, "batch_invalid"},
		{"body over 1 MiB", `{"entities":[{"type":"ticket","content":"` +
			strings.Repeat("a", batchBodyLimit) + `"}]}`, http.StatusRequestEntityTooLarge, "body_too_large"},
		{"unknown field", `{"entities":[{"type":"ticket","skip_automation":true}]}`,
			http.StatusBadRequest, "invalid_json"},
		{"trailing data", `{"entities":[{"type":"ticket"}]} {}`, http.StatusBadRequest, "invalid_json"},
		{"not JSON", `nope`, http.StatusBadRequest, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newBatchTestApp(t)
			rec := postBatch(context.Background(), t, app, tc.body)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			require.Equal(t, "https://rela.dev/errors/"+tc.code, decodeBatchError(t, rec).Type)
			require.Zero(t, countOf(t, app.store, "ticket"))
		})
	}
}

func entityLimitPlusOne() int { return 101 }

func TestBatch_ShapeErrorsNamePointer(t *testing.T) {
	tests := []struct {
		name      string
		relations string
		entities  string
		status    int
		pointer   string
		kind      string
		index     int
	}{
		{name: "duplicate ref", entities: `{"ref":"a","type":"ticket"},{"ref":"a","type":"ticket"}`,
			status: http.StatusBadRequest, pointer: "/entities/1/ref", kind: "entity", index: 1},
		{name: "unknown ref", relations: `{"from":{"ref":"a"},"type":"blocks","to":{"ref":"zz"}}`,
			status: http.StatusBadRequest, pointer: "/relations/0/to/ref", kind: "relation"},
		{name: "end with ref and id", relations: `{"from":{"ref":"a","id":"TKT-001"},"type":"blocks","to":{"ref":"a"}}`,
			status: http.StatusBadRequest, pointer: "/relations/0/from", kind: "relation"},
		{name: "end with neither", relations: `{"from":{},"type":"blocks","to":{"ref":"a"}}`,
			status: http.StatusBadRequest, pointer: "/relations/0/from", kind: "relation"},
		{name: "face on a ref end", relations: `{"from":{"ref":"a","face":"draft"},"type":"blocks","to":{"ref":"a"}}`,
			status: http.StatusBadRequest, pointer: "/relations/0/from/face", kind: "relation"},
		{name: "face on the head", relations: `{"from":{"ref":"a"},"type":"blocks","to":{"id":"TKT-001","face":"x"}}`,
			status: http.StatusBadRequest, pointer: "/relations/0/to/face", kind: "relation"},
		{name: "face inside an id", relations: `{"from":{"ref":"a"},"type":"cites","to":{"id":"FEAT-1@nl"}}`,
			status: http.StatusBadRequest, pointer: "/relations/0/to/id", kind: "relation"},
		{name: "unknown type", entities: `{"ref":"a","type":"ticket"},{"type":"nope"}`,
			status: http.StatusUnprocessableEntity, pointer: "/entities/1/type", kind: "entity", index: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newBatchTestApp(t)
			entities := tc.entities
			if entities == "" {
				entities = `{"ref":"a","type":"ticket","properties":{"title":"x"}}`
			}
			body := `{"entities":[` + entities + `],"relations":[` + tc.relations + `]}`
			rec := postBatch(context.Background(), t, app, body)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			e := decodeBatchError(t, rec)
			require.NotNil(t, e.Operation, rec.Body.String())
			require.Equal(t, tc.kind, e.Operation.Kind)
			require.Equal(t, tc.index, e.Operation.Index)
			require.Len(t, e.Errors, 1)
			require.Equal(t, tc.pointer, e.Errors[0].Source.Pointer)
			require.Zero(t, countOf(t, app.store, "ticket"))
		})
	}
}

// TestBatch_FieldGateDenialWritesNothing: the create field gate runs on
// every entity before anything is written, and its denial names the
// operation and is audited as on a single create.
func TestBatch_FieldGateDenialWritesNothing(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)
	app.batch.atomic = true
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: newTestAppV1(t).Meta()})
	setTestResolver(t, app, newVerdicts().ReadOnly("status").Build())

	rec := postBatch(context.Background(), t, app, `{"entities":[
		{"type":"ticket","properties":{"title":"ok"}},
		{"ref":"second","type":"ticket","properties":{"title":"ok","status":"open"}}
	]}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	e := decodeBatchError(t, rec)
	require.Equal(t, &v1.ErrorOperation{Kind: "entity", Index: 1, Ref: "second"}, e.Operation)
	require.Equal(t, "affordance", e.RuleKind)
	require.Equal(t, "field-affordance:read-only:status", e.RuleID)
	require.Zero(t, countOf(t, app.store, "ticket"))

	records := sink.Records()
	require.Len(t, records, 1)
	require.Equal(t, audit.OpDeniedWrite, records[0].Op)
}

func TestBatch_TypeDisallowedEdgeFailsWholeBatch(t *testing.T) {
	app := newBatchTestApp(t)
	rec := postBatch(context.Background(), t, app, `{
		"entities": [
			{"ref": "t", "type": "ticket", "properties": {"title": "a"}},
			{"ref": "f", "type": "feature", "properties": {"title": "b"}}
		],
		"relations": [{"from": {"ref": "f"}, "type": "implements", "to": {"ref": "t"}}]
	}`)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	e := decodeBatchError(t, rec)
	require.Equal(t, &v1.ErrorOperation{Kind: "relation", Index: 0}, e.Operation)
	require.Equal(t, "https://rela.dev/errors/source_type_not_allowed", e.Type)
	require.Zero(t, countOf(t, app.store, "ticket"))
	require.Zero(t, countOf(t, app.store, "feature"))
	require.Zero(t, countRelations(t, app.store))
}

func TestBatch_IDOptionsCheckedPerEntity(t *testing.T) {
	app := newBatchTestApp(t)
	rec := postBatch(context.Background(), t, app, `{"entities":[
		{"type":"feature","properties":{"title":"a"}},
		{"type":"ticket","id":"TKT-001","properties":{"title":"b"}}
	]}`)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	e := decodeBatchError(t, rec)
	require.Equal(t, &v1.ErrorOperation{Kind: "entity", Index: 1}, e.Operation)
	require.Equal(t, "/entities/1/id", e.Errors[0].Source.Pointer)
	require.Zero(t, countOf(t, app.store, "feature"))
}

// noteMeta is the newTestAppV1 metamodel plus a manual-id type, so a test
// can name the id a create collides with.
func noteMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta := newTestAppV1(t).Meta()
	meta.Entities["note"] = metamodel.EntityDef{
		Label: "Note", IDType: metamodel.IDTypeManual,
		Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
	}
	return meta
}

// TestBatch_KeptRowsAre500WithoutContent covers the answer to a batch the
// store kept part of. The endpoint refuses such a store (501), so this
// forces it on: the answer is a bare 500 that names no row.
func TestBatch_KeptRowsAre500WithoutContent(t *testing.T) {
	app := newAppFromParts(nil, noteMeta(t), newFixture())
	app.broker = newEventBroker()
	app.batch.atomic = true
	seedEntity(app, &entity.Entity{ID: "N-1", Type: "note", Properties: map[string]any{"title": "old"}})

	rec := postBatch(context.Background(), t, app, `{"entities":[
		{"type":"note","id":"N-2","properties":{"title":"a"}},
		{"ref":"dup","type":"note","id":"N-1","properties":{"title":"b"}}
	]}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "batch_partially_written")
	require.NotContains(t, rec.Body.String(), "N-2")
}

// batchPolicy lets alice create tickets and features but read only
// tickets: a feature she creates comes back redacted, and an existing
// feature is, to her, missing.
func batchPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"author": {
				Read:   []string{"ticket"},
				Create: []string{"ticket", "feature"},
				Update: []string{"ticket", "feature"},
			},
		},
		Assignments: map[string]string{"alice": "author"},
	}
}

func TestBatch_HiddenEndIsIndistinguishableFromMissing(t *testing.T) {
	app, d := newBatchACLApp(t, batchPolicy(),
		&entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "secret"}})

	body := func(target string) string {
		return fmt.Sprintf(`{"entities":[{"ref":"t","type":"ticket","properties":{"title":"a"}}],
			"relations":[{"from":{"ref":"t"},"type":"implements","to":{"id":%q}}]}`, target)
	}
	hidden := postBatchAs(t, app, d, body("FEAT-001"))
	missing := postBatchAs(t, app, d, body("FEAT-404"))
	require.Equal(t, http.StatusUnprocessableEntity, hidden.Code, hidden.Body.String())
	require.Equal(t, missing.Code, hidden.Code)

	he, me := decodeBatchError(t, hidden), decodeBatchError(t, missing)
	require.Equal(t, "https://rela.dev/errors/target_not_found", he.Type)
	require.Equal(t, "/relations/0/to", he.Errors[0].Source.Pointer)
	// The only difference is the id the caller sent.
	require.Equal(t, strings.ReplaceAll(missing.Body.String(), "FEAT-404", "FEAT-001"), hidden.Body.String())
	require.Equal(t, me.Operation, he.Operation)
	require.Zero(t, countOf(t, app.store, "ticket"))
	require.Zero(t, countRelations(t, app.store))
}

// TestBatch_HiddenSourceIsIndistinguishableFromMissing is the source-end
// twin of the test above: alice reads features but not tickets. The
// relation ACL runs before the ends are looked up (BUG-K6FEVB) and judges
// an unreadable source like an absent one, so both answer the same 403.
func TestBatch_HiddenSourceIsIndistinguishableFromMissing(t *testing.T) {
	p := batchPolicy()
	p.Roles["author"] = acl.RoleDef{
		Read: []string{"feature"}, Create: []string{"ticket", "feature"}, Update: []string{"ticket", "feature"},
	}
	app, d := newBatchACLApp(t, p,
		&entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "secret"}})

	body := func(source string) string {
		return fmt.Sprintf(`{"entities":[{"ref":"f","type":"feature","properties":{"title":"a"}}],
			"relations":[{"from":{"id":%q},"type":"implements","to":{"ref":"f"}}]}`, source)
	}
	hidden := postBatchAs(t, app, d, body("TKT-001"))
	missing := postBatchAs(t, app, d, body("TKT-404"))
	require.Equal(t, missing.Code, hidden.Code, hidden.Body.String())
	require.Equal(t, strings.ReplaceAll(missing.Body.String(), "TKT-404", "TKT-001"), hidden.Body.String())
	require.NotContains(t, hidden.Body.String(), "secret")
	require.Zero(t, countOf(t, app.store, "feature"))
}

func TestBatch_ACLDenialNamesOperation(t *testing.T) {
	p := batchPolicy()
	p.Roles["author"] = acl.RoleDef{Read: []string{"ticket"}, Create: []string{"ticket"}}
	app, d := newBatchACLApp(t, p)

	rec := postBatchAs(t, app, d, `{"entities":[
		{"type":"ticket","properties":{"title":"a"}},
		{"ref":"f","type":"feature","properties":{"title":"b"}}
	]}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	e := decodeBatchError(t, rec)
	require.Equal(t, &v1.ErrorOperation{Kind: "entity", Index: 1, Ref: "f"}, e.Operation)
	require.NotEmpty(t, e.RuleKind)
	require.Zero(t, countOf(t, app.store, "ticket"))
}

// TestBatch_UnreadableCreateIsRedacted: a row the caller may create but not
// read comes back as its id only (DEC-ODU65KR item 5).
func TestBatch_UnreadableCreateIsRedacted(t *testing.T) {
	app, d := newBatchACLApp(t, batchPolicy())

	rec := postBatchAs(t, app, d, `{
		"entities":[
			{"ref":"t","type":"ticket","properties":{"title":"a"}},
			{"ref":"f","type":"feature","properties":{"title":"secret"}}
		],
		"relations":[{"from":{"ref":"t"},"type":"implements","to":{"ref":"f"}}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "secret")

	var got v1.BatchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Entities[0].Entity)
	require.Nil(t, got.Entities[1].Entity)
	require.True(t, strings.HasPrefix(got.Entities[1].ID, "FEAT-"))
	require.Equal(t, 1, countOf(t, app.store, "feature"))
}
