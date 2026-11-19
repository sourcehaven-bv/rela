//go:build sqlite

package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// asOfApp serves the data-entry API over a real sqlitestore holding one
// ticket as it stood at the returned instant. After the instant, TKT-1 was
// retitled "New" and TKT-2 was created.
func asOfApp(t *testing.T) (*App, time.Time) {
	t.Helper()
	ctx := context.Background()
	st := sqliteStore(t)
	svc, ok := any(st).(store.VersionServiceProvider)
	require.True(t, ok)

	tkt := entity.New("TKT-1", "ticket")
	tkt.Properties = map[string]any{"title": "Old"}
	require.NoError(t, st.CreateEntity(ctx, tkt))
	stored, err := st.GetEntity(ctx, tkt.Ref())
	require.NoError(t, err)
	at := stored.UpdatedAt
	// The sweep's capture of the create: the content became current when the
	// row was written.
	require.NoError(t, svc.VersionStore().WriteVersion(ctx, store.VersionInput{
		EntityID:   "TKT-1",
		Type:       "ticket",
		Op:         store.VersionOpCreate,
		Properties: map[string]any{"title": "Old"},
		SchemaHash: "schema-1",
		Projection: []byte(`{"v":1}`),
		ContentAt:  at,
	}))
	time.Sleep(10 * time.Millisecond)

	stored.Properties["title"] = "New"
	require.NoError(t, st.UpdateEntity(ctx, stored))
	later := entity.New("TKT-2", "ticket")
	later.Properties = map[string]any{"title": "Later"}
	require.NoError(t, st.CreateEntity(ctx, later))

	app := newAppFromParts(nil, testMeta(), newFixture(), appbuildtest.WithStore(st))
	return app, at
}

func getAsOf(t *testing.T, app *App, method, path string, at time.Time) *httptest.ResponseRecorder {
	t.Helper()
	target := path + "?" + url.Values{AsOfParam: {at.Format(time.RFC3339Nano)}}.Encode()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(method, target, http.NoBody))
	return rec
}

func TestAsOf_SQLite_ListReadsTheInstant(t *testing.T) {
	app, at := asOfApp(t)

	rec := getAsOf(t, app, http.MethodGet, "/api/v1/tickets", at)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data []struct {
			ID         string         `json:"id"`
			Properties map[string]any `json:"properties"`
		} `json:"data"`
		Actions map[string]bool `json:"_actions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1, rec.Body.String())
	require.Equal(t, "TKT-1", resp.Data[0].ID)
	require.Equal(t, "Old", resp.Data[0].Properties["title"])
	require.False(t, resp.Actions["create"], "nothing may be created in the past")

	// Without the instant the list is today's.
	live := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))
	require.Equal(t, http.StatusOK, live.Code)
	require.Contains(t, live.Body.String(), "TKT-2")
	require.Contains(t, live.Body.String(), "New")
}

func TestAsOf_SQLite_DetailReadsTheInstant(t *testing.T) {
	app, at := asOfApp(t)

	rec := getAsOf(t, app, http.MethodGet, "/api/v1/tickets/TKT-1", at)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got struct {
		Properties map[string]any  `json:"properties"`
		Actions    map[string]bool `json:"_actions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "Old", got.Properties["title"])
	for verb, allowed := range got.Actions {
		require.False(t, allowed, "_actions[%s] must be false under as_of", verb)
	}

	// An entity created after the instant did not exist then.
	require.Equal(t, http.StatusNotFound, getAsOf(t, app, http.MethodGet, "/api/v1/tickets/TKT-2", at).Code)
}

func TestAsOf_SQLite_WritesAreRefused(t *testing.T) {
	app, at := asOfApp(t)

	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		rec := getAsOf(t, app, method, "/api/v1/tickets/TKT-1", at)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, method)
		require.Contains(t, rec.Body.String(), "as_of_read_only", method)
	}

	got, err := app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
	require.NoError(t, err)
	require.Equal(t, "New", got.Properties["title"])
}

func TestAsOf_SQLite_SearchReadsTheInstant(t *testing.T) {
	app, at := asOfApp(t)

	old := getAsOf(t, app, http.MethodGet, "/api/v1/_search", at)
	require.Equal(t, http.StatusOK, old.Code, old.Body.String())

	search := func(q string) string {
		t.Helper()
		target := "/api/v1/_search?" + url.Values{"q": {q}, AsOfParam: {at.Format(time.RFC3339Nano)}}.Encode()
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return rec.Body.String()
	}
	require.Contains(t, search("old"), "TKT-1")
	require.NotContains(t, search("new"), "TKT-1", "the title was New only after the instant")
	require.NotContains(t, search("later"), "TKT-2", "TKT-2 did not exist at the instant")

	live := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=new", http.NoBody))
	require.Contains(t, live.Body.String(), "TKT-1", "today the title is New")
}

// TestAsOf_SQLite_DocumentReadsTheInstant renders a script document and a
// command document at the instant. The command render runs live first, so
// a disk-cache hit would serve today's title.
func TestAsOf_SQLite_DocumentReadsTheInstant(t *testing.T) {
	app, at := asOfApp(t)

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts", "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "docs", "report.lua"), []byte(
		`local e = rela.get_entity(rela.document.entry_id)
print("TITLE: " .. e.properties.title)
`), 0o644))
	deps := func() lua.WriteDeps {
		wd := app.luaWriteDeps()
		wd.ProjectRoot = root
		wd.Files = nil
		return wd
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "docs", "writer.lua"), []byte(
		`local ok, err = pcall(rela.history, "TKT-1")
print("GUARD: " .. tostring(ok) .. " " .. tostring(err))
`), 0o644))
	app.documents = newDocumentService(app.store, app.kv, root, script.NewEngine(), deps, nil)
	s := app.State()
	cfg := *s.Cfg
	cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"report": {EntityType: "ticket", Script: "docs/report.lua"},
		"raw":    {EntityType: "ticket", Command: []string{"cat", "{in}"}},
		"writer": {EntityType: "ticket", Script: "docs/writer.lua"},
	}
	app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap, StyledTypes: s.StyledTypes,
		OpenAPIGen: s.OpenAPIGen})

	live := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/api/v1/_documents/raw/TKT-1", http.NoBody))
	if live.Code == http.StatusOK {
		require.Contains(t, live.Body.String(), "New")
	}

	// A script cannot reach past the store's read-only seam under an instant.
	guard := getAsOf(t, app, http.MethodGet, "/api/v1/_documents/writer/TKT-1", at)
	require.Equal(t, http.StatusOK, guard.Code, guard.Body.String())
	require.Contains(t, guard.Body.String(), "GUARD: false")
	require.Contains(t, guard.Body.String(), "past instant")

	for doc, want := range map[string]string{"report": "TITLE: Old", "raw": "Old"} {
		rec := getAsOf(t, app, http.MethodGet, "/api/v1/_documents/"+doc+"/TKT-1", at)
		if doc == "raw" && rec.Code != http.StatusOK {
			t.Logf("command render unavailable on this host: %s", rec.Body)
			continue
		}
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), want, doc)
		require.NotContains(t, rec.Body.String(), "New", doc)
		require.Contains(t, rec.Body.String(), `"cached":false`, doc)
	}
}

// TestAsOf_SQLite_DashboardCardCountsTheInstant: a list card counts the rows
// that existed at the instant.
func TestAsOf_SQLite_DashboardCardCountsTheInstant(t *testing.T) {
	app, at := asOfApp(t)
	installDashboardCards(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "All", List: "all", Display: "count"})

	count := func(path string) int {
		t.Helper()
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var v struct {
			Total int `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v))
		return v.Total
	}
	require.Equal(t, 2, count("/api/v1/_dashboard/card?card=0"))
	require.Equal(t, 1, count("/api/v1/_dashboard/card?card=0&"+
		url.Values{AsOfParam: {at.Format(time.RFC3339Nano)}}.Encode()))
}

// TestAsOf_SQLite_AccessLostSinceFailsClosed: a past read is gated by today's
// ACL (DEC-TDEF3GD). Alice read TKT-1 through a role conferred by an `owns`
// edge. After the edge is removed, a read at an instant when she still held
// it shows her nothing: the list leaves the row out and the detail is a 404.
func TestAsOf_SQLite_AccessLostSinceFailsClosed(t *testing.T) {
	const aclYAML = `
roles:
  owner:
    read: [ticket]
  auditor:
    permissions: [history:read]
role_relations:
  owns:
    confers: owner
assignments:
  alice: auditor
`
	ctx := context.Background()
	st := sqliteStore(t)
	app := buildPolicyApp(t, aclYAML, nil, appbuildtest.WithStore(st))
	tkt := entity.New("TKT-1", "ticket")
	tkt.Properties = map[string]any{"title": "Owned"}
	require.NoError(t, st.CreateEntity(ctx, tkt))
	owns := entity.RelationKey{From: "alice", Type: "owns", To: "TKT-1"}
	_, err := st.CreateRelation(ctx, owns, &store.RelationData{})
	require.NoError(t, err)
	// The history records the edge from its creation, as the version sweep
	// would, so the edge exists at the instant read below.
	vs, ok := st.(store.VersionServiceProvider)
	require.True(t, ok)
	versions := vs.VersionStore()
	require.NoError(t, versions.WriteRelationVersion(ctx, store.RelationVersionInput{
		Key: owns, Op: store.VersionOpCreate, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
	}))
	time.Sleep(10 * time.Millisecond)
	at := time.Now()
	time.Sleep(10 * time.Millisecond)

	app.SetPrincipalResolver(EnvPrincipalResolver())
	t.Setenv(envDataEntryUser, "alice")
	asAlice := func(path string) *httptest.ResponseRecorder {
		target := path + "?" + url.Values{AsOfParam: {at.Format(time.RFC3339Nano)}}.Encode()
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
		return rec
	}

	search := func() *httptest.ResponseRecorder {
		target := "/api/v1/_search?" + url.Values{"q": {"Owned"}, AsOfParam: {at.Format(time.RFC3339Nano)}}.Encode()
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
		return rec
	}

	// Control: while she holds the edge, the past read shows the ticket.
	rec := asAlice("/api/v1/tickets/TKT-1")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = search()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "TKT-1", "control: search finds the ticket")

	require.NoError(t, versions.WriteRelationVersion(ctx, store.RelationVersionInput{
		Key: owns, Op: store.VersionOpDelete, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
	}))
	require.NoError(t, st.DeleteRelation(ctx, owns))

	rec = asAlice("/api/v1/tickets/TKT-1")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = asAlice("/api/v1/tickets/TKT-1/relations")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = asAlice("/api/v1/tickets")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "TKT-1")
	rec = search()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "TKT-1", "search fails closed too")
}

// TestAsOf_SQLite_RelationConditionedFieldFailsClosed: field visibility at an
// instant follows the history endpoint's rules (TKT-73C6B2). A `visible:`
// grant whose condition reads a relation shows its field on a live read and
// hides it on a past one, in the detail, the list and search alike, so a
// search cannot match on the hidden value.
func TestAsOf_SQLite_RelationConditionedFieldFailsClosed(t *testing.T) {
	ctx := context.Background()
	st := sqliteStore(t)
	for id, props := range map[string]map[string]any{
		"TKT-1": {"title": "Gated", "priority": "zanzibar", "status": "open"},
		"TKT-2": {"title": "Other", "status": "open"},
	} {
		tkt := entity.New(id, "ticket")
		tkt.Properties = props
		require.NoError(t, st.CreateEntity(ctx, tkt))
	}
	_, err := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "depends_on", To: "TKT-2"},
		&store.RelationData{})
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	at := time.Now()
	time.Sleep(10 * time.Millisecond)
	app := buildPolicyApp(t, triagerReadsHistory(t), nil, appbuildtest.WithStore(st))

	app.SetPrincipalResolver(EnvPrincipalResolver())
	t.Setenv(envDataEntryUser, "alice")
	get := func(path string, q url.Values) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?"+q.Encode(), http.NoBody))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return rec
	}
	past := url.Values{AsOfParam: {at.Format(time.RFC3339Nano)}}

	// Control: the live read shows the field, so the grant is real.
	require.Contains(t, get("/api/v1/tickets/TKT-1", nil).Body.String(), "zanzibar")

	detail := get("/api/v1/tickets/TKT-1", past).Body.String()
	require.Contains(t, detail, "Gated")
	require.NotContains(t, detail, "zanzibar")
	require.NotContains(t, get("/api/v1/tickets", past).Body.String(), "zanzibar")

	search := url.Values{"q": {"zanzibar"}, AsOfParam: past[AsOfParam]}
	require.NotContains(t, get("/api/v1/_search", search).Body.String(), "TKT-1")
	require.Contains(t, get("/api/v1/_search", url.Values{"q": {"zanzibar"}}).Body.String(), "TKT-1")
}

// triagerReadsHistory is historyRedactionACL with history:read granted to
// the triager, so alice may read the past under the same field grants.
func triagerReadsHistory(t *testing.T) string {
	t.Helper()
	const role = "  triager:\n"
	require.Contains(t, historyRedactionACL, role)
	return strings.Replace(historyRedactionACL, role, role+"    permissions: [history:read]\n", 1)
}

// TestAsOf_SQLite_NeedsHistoryRead: a past read holds content deleted since,
// which the history endpoint shows only to holders of history:read, so an
// as_of read without it is refused with a 403 that names the permission.
func TestAsOf_SQLite_NeedsHistoryRead(t *testing.T) {
	st := sqliteStore(t)
	app := buildPolicyApp(t, historyRedactionACL, nil, appbuildtest.WithStore(st))
	app.SetPrincipalResolver(EnvPrincipalResolver())
	t.Setenv(envDataEntryUser, "alice")
	for _, path := range []string{"/api/v1/tickets", "/api/v1/tickets/TKT-1", "/api/v1/_search"} {
		rec := httptest.NewRecorder()
		target := path + "?" + url.Values{AsOfParam: {time.Now().Format(time.RFC3339Nano)}}.Encode()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
		require.Equal(t, http.StatusForbidden, rec.Code, path)
		require.Contains(t, rec.Body.String(), "history:read", path)
	}
}

// heldLeases reports every asked id as held by alice.
type heldLeases struct{ stubLeases }

func (heldLeases) GetReadable(_ context.Context, ids []string) (map[string]lease.Lease, error) {
	out := make(map[string]lease.Lease, len(ids))
	for _, id := range ids {
		out[id] = lease.Lease{ID: id, Holder: lease.Holder{User: "alice"}, Since: time.Now()}
	}
	return out, nil
}

// TestAsOf_SQLite_NoLeasesInThePast: leases keep no history, so a read of a
// past instant shows none rather than today's.
func TestAsOf_SQLite_NoLeasesInThePast(t *testing.T) {
	app, at := asOfApp(t)
	installLeases(app, heldLeases{})

	live := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-1", http.NoBody))
	require.Equal(t, http.StatusOK, live.Code, live.Body.String())
	require.Contains(t, live.Body.String(), `"_lease"`, "a live read shows the lease")

	for _, path := range []string{"/api/v1/tickets/TKT-1", "/api/v1/tickets"} {
		rec := getAsOf(t, app, http.MethodGet, path, at)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), `"_lease"`, path)
	}
}

// TestAsOf_SQLite_GanttDrillFollowsThePastHierarchy: a drilled gantt
// (?root=) follows the hierarchy edges as they stood at the instant. PRJ-B
// was contained then and was moved out since; PRJ-C was moved in since.
func TestAsOf_SQLite_GanttDrillFollowsThePastHierarchy(t *testing.T) {
	ctx := context.Background()
	st := sqliteStore(t)
	app := newGanttTestAppWith(t, []appbuildtest.Option{appbuildtest.WithStore(st)})
	for _, id := range []string{"PRJ-A", "PRJ-B", "PRJ-C"} {
		e := entity.New(id, "project")
		e.Properties = map[string]any{"title": id, "planned_start": "2025-01-01", "planned_end": "2025-02-01"}
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	moved := entity.RelationKey{From: "PRJ-A", Type: "contains", To: "PRJ-B"}
	_, err := st.CreateRelation(ctx, moved, &store.RelationData{})
	require.NoError(t, err)
	vs, ok := st.(store.VersionServiceProvider)
	require.True(t, ok)
	require.NoError(t, vs.VersionStore().WriteRelationVersion(ctx, store.RelationVersionInput{
		Key: moved, Op: store.VersionOpCreate, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
	}))
	time.Sleep(10 * time.Millisecond)
	at := time.Now()
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, vs.VersionStore().WriteRelationVersion(ctx, store.RelationVersionInput{
		Key: moved, Op: store.VersionOpDelete, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
	}))
	require.NoError(t, st.DeleteRelation(ctx, moved))
	_, err = st.CreateRelation(ctx, entity.RelationKey{From: "PRJ-A", Type: "contains", To: "PRJ-C"}, &store.RelationData{})
	require.NoError(t, err)

	target := "/api/v1/_gantts/plan?" + url.Values{"root": {"PRJ-A"}, AsOfParam: {at.Format(time.RFC3339Nano)}}.Encode()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PRJ-B", "contained at the instant")
	require.NotContains(t, rec.Body.String(), "PRJ-C", "moved in since")
}
