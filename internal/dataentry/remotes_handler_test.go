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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// fakeRemotes is a [RemoteService] over one fixed remote. It records the
// actions it was asked to take.
type fakeRemotes struct {
	status RemoteStatus
	err    error
	actErr error
	calls  []string
}

func (f *fakeRemotes) Remotes(context.Context) ([]RemoteStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []RemoteStatus{f.status}, nil
}

func (f *fakeRemotes) Resolve(_ context.Context, name string, key RemoteEntryKey, take string) (int, error) {
	f.calls = append(f.calls, fmt.Sprintf("resolve %s %s/%s %s", name, key.Local, key.Field, take))
	return 1, f.actErr
}

func (f *fakeRemotes) Dismiss(_ context.Context, name string, key RemoteEntryKey) (int, error) {
	f.calls = append(f.calls, fmt.Sprintf("dismiss %s %s/%s", name, key.Local, key.Field))
	return 1, f.actErr
}

func (f *fakeRemotes) Link(_ context.Context, name string, key RemoteEntryKey, address string) error {
	f.calls = append(f.calls, fmt.Sprintf("link %s %s %v %s", name, key.Local, key.Fields, address))
	return f.actErr
}

var remoteT0 = time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

// ticketEntry is an entry of the ticket local. An uncertain create has no
// remote address and names the fields it tried to send, as remotesync
// stores it.
func ticketEntry(local, field, reason string) RemoteEntry {
	e := RemoteEntry{
		RemoteRow: RemoteRow{Type: "ticket", Local: local}, Address: "ticket/" + local, Field: field,
		RemoteField: field, Reason: reason, Rule: "hold", Since: remoteT0, Seen: remoteT0,
		RemoteValue: json.RawMessage(`"Theirs"`),
	}
	if reason == "uncertain" {
		e.Address, e.RemoteValue, e.Fields = "", nil, []string{"title"}
	}
	return e
}

// remoteEnv is an app with a fake remote service, a compiled policy and a
// counting store under every read the routes make.
type remoteEnv struct {
	app      *App
	d        *acl.Declarative
	fake     *fakeRemotes
	counting *storetest.Counting
}

// newRemoteEnv seeds the tickets TKT-001 and TKT-002 and the feature
// FEAT-HID, which a role that reads only tickets cannot see, and wires
// role for alice.
func newRemoteEnv(t *testing.T, role acl.RoleDef, entries []RemoteEntry, seed func(app *App)) *remoteEnv {
	t.Helper()
	app := newTestAppV1(t)
	for _, id := range []string{"TKT-001", "TKT-002"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
	}
	seedEntity(app, &entity.Entity{ID: "FEAT-HID", Type: "feature",
		Properties: map[string]any{"title": "hidden"}})
	if seed != nil {
		seed(app)
	}
	counting := storetest.NewCounting(app.store)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"syncer": role},
		Assignments: map[string]string{"alice": "syncer"},
	}, counting)
	svc := appbuildtest.New(app.State().Meta, appbuildtest.WithStore(counting),
		appbuildtest.WithFS(app.fs, app.paths), appbuildtest.WithACL(d))
	rebindApp(app, app.fs, app.paths, svc)
	setTestACL(t, app, d)
	resolver, err := affordances.New(app.State().Meta, storeRelationLookup{st: counting}, d)
	require.NoError(t, err)
	setTestResolver(t, app, &policyResolver{inner: resolver})
	fake := &fakeRemotes{status: RemoteStatus{
		Remote: remotestatus.Remote{
			Name:                "upstream",
			Settings:            remotestatus.Settings{URL: "https://upstream.test", Conflict: "hold"},
			CredentialKind:      "env",
			SupersededRetention: 24 * time.Hour,
			Run:                 remotestatus.Run{LastRun: remoteT0, Failures: 2},
			Types: []remotestatus.MappedType{{Local: "ticket", Source: "ticket", Fields: []remotestatus.MappedField{
				{Local: "title", Remote: "title", Rule: "hold"},
			}}},
		},
		Entries: entries,
	}}
	require.NoError(t, SetRemotes(app, fake))
	return &remoteEnv{app: app, d: d, fake: fake, counting: counting}
}

// syncRole reads and updates tickets, not features, and holds
// remote:sync.
func syncRole() acl.RoleDef {
	return acl.RoleDef{
		Read:        []string{"ticket"},
		Update:      []string{"ticket"},
		Permissions: []string{acl.PermRemoteSync},
	}
}

func hiddenEntry() RemoteEntry {
	e := ticketEntry("FEAT-HID", "title", "both_changed")
	e.Type, e.Address = "feature", "feature/FEAT-HID"
	return e
}

func (e *remoteEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.app.remotes.handleV1Remotes(rec, asAlice(t, e.d, req))
	return rec
}

func decodeRemote[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func TestRemotes_NotWired(t *testing.T) {
	t.Parallel()
	var h *remotesHandler
	rec := httptest.NewRecorder()
	h.handleV1Remotes(rec, httptest.NewRequest(http.MethodGet, remotesPath, http.NoBody))
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Error(t, SetRemotes(nil, &fakeRemotes{}))
	require.Error(t, SetRemotes(newTestAppV1(t), nil))
}

func TestRemotes_NeedsPermission(t *testing.T) {
	t.Parallel()
	role := syncRole()
	role.Permissions = nil
	e := newRemoteEnv(t, role, []RemoteEntry{ticketEntry("TKT-001", "title", "both_changed")}, nil)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, remotesPath, ""},
		{http.MethodGet, remotesPath + "/upstream/conflicts", ""},
		{http.MethodPost, remotesPath + "/upstream/conflicts/_resolve", `{"local":"TKT-001","field":"title","take":"local"}`},
	} {
		rec := e.do(t, tc.method, tc.path, tc.body)
		require.Equal(t, http.StatusForbidden, rec.Code, tc.path)
		require.Contains(t, rec.Body.String(), "permission_required")
		require.Contains(t, rec.Body.String(), acl.PermRemoteSync)
	}
	require.Empty(t, e.fake.calls)
}

// TestRemotes_ListCountsAfterGating: a hidden row and a field hidden by
// `visible:` count nowhere, and the summary carries no credential value.
func TestRemotes_ListCountsAfterGating(t *testing.T) {
	t.Parallel()
	role := syncRole()
	role.Visible = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}
	entries := []RemoteEntry{
		ticketEntry("TKT-001", "title", "both_changed"),
		ticketEntry("TKT-001", "status", "both_changed"), // field hidden
		hiddenEntry(), // row hidden
		ticketEntry("TKT-GONE", "title", "both_changed"), // no such row
		ticketEntry("TKT-002", "", "uncertain"),
	}
	e := newRemoteEnv(t, role, entries, nil)
	e.fake.status.LinkedRows = []RemoteRow{
		{Type: "ticket", Local: "TKT-001"}, {Type: "feature", Local: "FEAT-HID"},
		{Type: "feature", Local: "TKT-002"}, // wrong type: not the row it names
	}
	rec := e.do(t, http.MethodGet, remotesPath, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeRemote[remoteList](t, rec)
	require.Len(t, got.Data, 1)
	s := got.Data[0]
	require.Equal(t, "upstream", s.Name)
	require.Equal(t, "env", s.CredentialKind)
	require.Equal(t, int64(86400), s.SupersededRetentionSeconds)
	require.Equal(t, 2, s.Failures)
	require.Equal(t, remoteT0, s.LastRun.UTC())
	require.NotContains(t, rec.Body.String(), "next_run", "a zero time is left out")
	require.Equal(t, 1, s.Linked)
	require.Equal(t, 2, s.Conflicts.Total)
	require.Equal(t, map[string]int{"both_changed": 1, "uncertain": 1}, s.Conflicts.ByReason)
	require.NotContains(t, rec.Body.String(), "FEAT-HID")
}

func TestRemotes_ConflictsPageAndActions(t *testing.T) {
	t.Parallel()
	role := syncRole()
	// alice may write status but not title.
	role.Fields = map[string][]acl.FieldGrant{"ticket": {{Field: "status"}}}
	sup := ticketEntry("TKT-002", "title", "superseded")
	sup.Rule = "latest"
	sup.Superseded = &remotestatus.Superseded{Winner: "remote", LocalUpdated: remoteT0, RemoteUpdated: remoteT0}
	entries := []RemoteEntry{
		ticketEntry("TKT-001", "title", "both_changed"),
		ticketEntry("TKT-001", "status", "both_changed"),
		hiddenEntry(),
		ticketEntry("TKT-002", "", "uncertain"),
		sup,
	}
	e := newRemoteEnv(t, role, entries, nil)

	rec := e.do(t, http.MethodGet, remotesPath+"/upstream/conflicts?per_page=2&page=2", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	page := decodeRemote[remoteConflictPage](t, rec)
	require.Equal(t, 4, page.Total)
	require.Equal(t, map[string]int{"both_changed": 2, "uncertain": 1, "superseded": 1}, page.Counts)
	require.Len(t, page.Data, 2)
	require.NotContains(t, rec.Body.String(), "FEAT-HID")

	uncertain, superseded := page.Data[0], page.Data[1]
	require.Equal(t, "uncertain", uncertain.Reason)
	require.Equal(t, map[string]bool{
		"resolve_local": true, "resolve_remote": false, "dismiss": false, "link": false,
	}, uncertain.Actions, "the create sent title, which alice may not edit, so she may not link it")
	require.Equal(t, map[string]bool{
		"resolve_local": false, "resolve_remote": false, "dismiss": true, "link": false,
	}, superseded.Actions)
	// Retention 0 kept no local value: the page says so.
	require.NotNil(t, superseded.Superseded)
	require.False(t, superseded.Superseded.LostValueKept)

	rec = e.do(t, http.MethodGet, remotesPath+"/upstream/conflicts", "")
	page = decodeRemote[remoteConflictPage](t, rec)
	byField := map[string]remoteEntryWire{}
	for _, w := range page.Data {
		if w.Local == "TKT-001" {
			byField[w.Field] = w
		}
	}
	require.False(t, byField["title"].Actions["resolve_remote"], "title is outside the fields grant")
	require.True(t, byField["status"].Actions["resolve_remote"])
	require.True(t, byField["title"].Actions["resolve_local"])

	rec = e.do(t, http.MethodGet, remotesPath+"/nope/conflicts", "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	// A page past the end is empty, however large its number.
	for _, p := range []string{"3", "9223372036854775807"} {
		rec = e.do(t, http.MethodGet, remotesPath+"/upstream/conflicts?per_page=2&page="+p, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		page = decodeRemote[remoteConflictPage](t, rec)
		require.Empty(t, page.Data, p)
		require.Equal(t, 4, page.Total)
	}
}

// TestRemotes_ActAgreesWithActions: every `_actions` verdict is what the
// POST answers, a hidden or missing entry is the same 404, and a bad body
// is a 400 that never reaches the service.
func TestRemotes_ActAgreesWithActions(t *testing.T) {
	t.Parallel()
	role := syncRole()
	role.Fields = map[string][]acl.FieldGrant{"ticket": {{Field: "status"}}}
	sup := ticketEntry("TKT-002", "title", "superseded")
	sup.Superseded = &remotestatus.Superseded{Winner: "local", LocalUpdated: remoteT0, RemoteUpdated: remoteT0}
	// An uncertain create that sent only the field the role may edit.
	writableCreate := ticketEntry("TKT-003", "", "uncertain")
	writableCreate.Fields = []string{"status"}
	entries := []RemoteEntry{
		ticketEntry("TKT-001", "title", "both_changed"),
		ticketEntry("TKT-001", "status", "both_changed"),
		hiddenEntry(),
		ticketEntry("TKT-002", "", "uncertain"),
		sup,
		writableCreate,
	}
	seed := func(app *App) {
		seedEntity(app, &entity.Entity{ID: "TKT-003", Type: "ticket", Properties: map[string]any{"title": "TKT-003"}})
	}
	const conflicts = remotesPath + "/upstream/conflicts/"
	tests := []struct {
		name, op, body string
		want           int
	}{
		{"take local", "_resolve", `{"local":"TKT-001","field":"title","take":"local"}`, http.StatusOK},
		{"take remote without edit right", "_resolve", `{"local":"TKT-001","field":"title","take":"remote"}`,
			http.StatusForbidden},
		{"take remote with edit right", "_resolve", `{"local":"TKT-001","field":"status","take":"remote"}`,
			http.StatusOK},
		{"hidden row", "_resolve", `{"local":"FEAT-HID","field":"title","take":"local"}`, http.StatusNotFound},
		{"missing entry", "_resolve", `{"local":"TKT-404","field":"title","take":"local"}`, http.StatusNotFound},
		{"resolve a superseded entry", "_resolve", `{"local":"TKT-002","field":"title","take":"local"}`,
			http.StatusConflict},
		{"take remote on an uncertain create", "_resolve", `{"local":"TKT-002","take":"remote"}`,
			http.StatusConflict},
		{"dismiss superseded", "_dismiss", `{"local":"TKT-002","field":"title"}`, http.StatusOK},
		{"dismiss a held entry", "_dismiss", `{"local":"TKT-001","field":"title"}`, http.StatusConflict},
		// A link makes the next pull take the remote row's values, so it
		// needs edit rights on every field the create sent.
		{"link without edit right", "_link", `{"local":"TKT-002","address":"ticket/R9"}`, http.StatusForbidden},
		{"link with edit right", "_link", `{"local":"TKT-003","address":"ticket/R9"}`, http.StatusOK},
		{"link a field entry", "_link", `{"local":"TKT-001","address":"ticket/R9"}`, http.StatusNotFound},
		{"bad take", "_resolve", `{"local":"TKT-001","field":"title","take":"both"}`, http.StatusBadRequest},
		{"no local", "_dismiss", `{"field":"title"}`, http.StatusBadRequest},
		{"unknown key", "_dismiss", `{"local":"TKT-002","field":"title","x":1}`, http.StatusBadRequest},
		{"address on resolve", "_resolve", `{"local":"TKT-001","field":"title","take":"local","address":"a/b"}`,
			http.StatusBadRequest},
		{"link with a field", "_link", `{"local":"TKT-002","field":"title","address":"ticket/R9"}`,
			http.StatusBadRequest},
		{"unknown op", "_merge", `{"local":"TKT-002"}`, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newRemoteEnv(t, role, entries, seed)
			rec := e.do(t, http.MethodPost, conflicts+tc.op, tc.body)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			if tc.want == http.StatusOK {
				require.Len(t, e.fake.calls, 1)
				require.JSONEq(t, `{"changed":1}`, rec.Body.String())
			} else {
				require.Empty(t, e.fake.calls)
			}
			require.NotContains(t, rec.Body.String(), "FEAT-HID")
		})
	}

	// The service gets the fields the handler checked, so it can refuse an
	// entry a sync replaced in between.
	e := newRemoteEnv(t, role, entries, seed)
	rec := e.do(t, http.MethodPost, conflicts+"_link", `{"local":"TKT-003","address":"ticket/R9"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []string{"link upstream TKT-003 [status] ticket/R9"}, e.fake.calls)

	// The hidden entry and a missing one answer byte for byte the same.
	e = newRemoteEnv(t, role, entries, nil)
	hidden := e.do(t, http.MethodPost, conflicts+"_resolve", `{"local":"FEAT-HID","field":"title","take":"local"}`)
	missing := e.do(t, http.MethodPost, conflicts+"_resolve", `{"local":"TKT-404","field":"title","take":"local"}`)
	require.Equal(t, missing.Body.String(), strings.ReplaceAll(hidden.Body.String(), "FEAT-HID", "TKT-404"))

	// GET on an action route is a 405.
	rec = e.do(t, http.MethodGet, conflicts+"_resolve", "")
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestRemotes_ServiceErrors(t *testing.T) {
	t.Parallel()
	entries := []RemoteEntry{ticketEntry("TKT-001", "title", "both_changed")}
	body := `{"local":"TKT-001","field":"title","take":"local"}`
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{fmt.Errorf("x: %w", ErrRemoteBusy), http.StatusConflict, "remote_busy"},
		{fmt.Errorf("x: %w", ErrRemoteRefused), http.StatusConflict, "invalid_state"},
		{fmt.Errorf("x: %w", ErrRemoteNoEntry), http.StatusNotFound, "not_found"},
		{fmt.Errorf("x: %w", ErrRemoteUnknown), http.StatusNotFound, "not_found"},
		{errors.New("token=secretvalue"), http.StatusInternalServerError, "remote_failed"},
	} {
		e := newRemoteEnv(t, syncRole(), entries, nil)
		e.fake.actErr = tc.err
		rec := e.do(t, http.MethodPost, remotesPath+"/upstream/conflicts/_resolve", body)
		require.Equal(t, tc.want, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), tc.code)
		require.NotContains(t, rec.Body.String(), "secretvalue")
	}
	e := newRemoteEnv(t, syncRole(), entries, nil)
	e.fake.err = errors.New("read failed: password=secretvalue")
	rec := e.do(t, http.MethodGet, remotesPath, "")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "secretvalue")
}

// TestRemotes_RelationEntries: a relation entry is visible when both
// endpoints are, and hidden when one is.
func TestRemotes_RelationEntries(t *testing.T) {
	t.Parallel()
	rel := func(from, to string) RemoteEntry {
		key := entity.RelationKey{From: from, Type: "blocks", To: to}
		return RemoteEntry{
			RemoteRow: RemoteRow{Kind: remoteKindRelation, Type: "blocks", Local: from + "--blocks--" + to,
				Relation: key},
			Reason: "refused", Cause: "refused", Status: 422, Rule: "hold", Since: remoteT0, Seen: remoteT0,
		}
	}
	e := newRemoteEnv(t, syncRole(), []RemoteEntry{rel("TKT-001", "TKT-002"), rel("TKT-001", "FEAT-HID")},
		func(app *App) {
			seedRelation(app, &entity.Relation{From: "TKT-001", Type: "blocks", To: "TKT-002"})
			seedRelation(app, &entity.Relation{From: "TKT-001", Type: "blocks", To: "FEAT-HID"})
		})
	rec := e.do(t, http.MethodGet, remotesPath+"/upstream/conflicts", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	page := decodeRemote[remoteConflictPage](t, rec)
	require.Equal(t, 1, page.Total)
	require.Equal(t, "TKT-001--blocks--TKT-002", page.Data[0].Local)
	require.Equal(t, 422, page.Data[0].Status)
	require.True(t, page.Data[0].Actions["resolve_local"])
	require.False(t, page.Data[0].Actions["resolve_remote"], "a refused relation has no address to take")
}

// TestRemotes_ReadBudget (collection-reads.md): the list and the conflict
// page read the store the same number of times for 10 entries as for 50,
// half of them on hidden rows, and read the same rows whether the store
// holds 20 or 200 tickets no entry names.
func TestRemotes_ReadBudget(t *testing.T) {
	t.Parallel()
	measure := func(t *testing.T, n, pad int, path string) (calls string, cost storetest.Cost) {
		t.Helper()
		var entries []RemoteEntry
		var linked []RemoteRow
		seed := func(app *App) {
			for i := range n {
				id, typ := fmt.Sprintf("TKT-%03d", 100+i), "ticket"
				if i%2 == 1 {
					typ = "feature"
				}
				seedEntity(app, &entity.Entity{ID: id, Type: typ, Properties: map[string]any{"title": id}})
			}
			for i := range pad {
				id := fmt.Sprintf("PAD-%05d", i)
				seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
			}
		}
		for i := range n {
			id, typ := fmt.Sprintf("TKT-%03d", 100+i), "ticket"
			if i%2 == 1 {
				typ = "feature"
			}
			e := ticketEntry(id, "title", "both_changed")
			e.Type = typ
			entries = append(entries, e)
			linked = append(linked, RemoteRow{Type: typ, Local: id})
		}
		e := newRemoteEnv(t, syncRole(), entries, seed)
		e.fake.status.LinkedRows = linked
		e.counting.Reset()
		rec := e.do(t, http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NotZero(t, e.counting.Reads(), "the budget measures nothing")
		return fmt.Sprint(e.counting.Calls()), e.counting.Cost()
	}
	for _, path := range []string{remotesPath, remotesPath + "/upstream/conflicts?per_page=100"} {
		small, _ := measure(t, 10, 0, path)
		large, _ := measure(t, 50, 0, path)
		require.Equal(t, small, large, path)
		assertScaleFreeIndexed(t, func(pad int) (storetest.Cost, string) {
			_, cost := measure(t, 10, pad, path)
			return cost, path
		})
	}
}

// TestRemotes_MeOffersPage: /_me says the Remotes link may show only when
// remotes are wired and the caller holds remote:sync.
func TestRemotes_MeOffersPage(t *testing.T) {
	t.Parallel()
	noPerm := syncRole()
	noPerm.Permissions = nil
	for _, tc := range []struct {
		name   string
		role   acl.RoleDef
		unwire bool
		want   bool
	}{
		{"holds remote:sync", syncRole(), false, true},
		{"lacks remote:sync", noPerm, false, false},
		{"no remote service", syncRole(), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newRemoteEnv(t, tc.role, nil, nil)
			if tc.unwire {
				e.app.remotes = nil
			}
			req := asAlice(t, e.d, httptest.NewRequest(http.MethodGet, "/api/v1/_me", http.NoBody))
			require.Equal(t, tc.want, decodeMe(t, meAs(req.Context(), t, e.app)).CanRemoteSync)
		})
	}
}

// TestRemotes_RemoteDeletedNeedsDelete: taking the remote side of a row the
// remote deleted deletes it here, so it needs the delete right (TKT-CVT9W1K).
func TestRemotes_RemoteDeletedNeedsDelete(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		delete bool
	}{{"update only", false}, {"with delete", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			role := syncRole()
			if tc.delete {
				role.Delete = []string{"ticket"}
			}
			entry := ticketEntry("TKT-001", "", "remote_deleted")
			entry.Cause = "edited"
			e := newRemoteEnv(t, role, []RemoteEntry{entry}, nil)
			rec := e.do(t, http.MethodGet, remotesPath+"/upstream/conflicts", "")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			page := decodeRemote[remoteConflictPage](t, rec)
			require.Len(t, page.Data, 1)
			require.True(t, page.Data[0].Actions["resolve_local"])
			require.Equal(t, tc.delete, page.Data[0].Actions["resolve_remote"])
		})
	}
}

// TestRemotes_RemoteDeletedRelationNeedsRemove: the same holds for a
// relation, judged as the relation DELETE route judges it.
func TestRemotes_RemoteDeletedRelationNeedsRemove(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		delete, remove bool
	}{{"no delete", false, true}, {"remove denied", true, false}, {"allowed", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			role := syncRole()
			if tc.delete {
				role.Delete = []string{"ticket"}
			}
			if !tc.remove {
				role.Relations = map[string][]acl.RelationGrant{"ticket": {{Relation: "blocks", Remove: new(bool)}}}
			}
			key := entity.RelationKey{From: "TKT-001", Type: "blocks", To: "TKT-002"}
			entry := RemoteEntry{
				RemoteRow: RemoteRow{Kind: remoteKindRelation, Type: "blocks", Local: "TKT-001--blocks--TKT-002",
					Relation: key},
				Address: "ticket/T-1/blocks/ticket/T-2", Reason: "remote_deleted", Cause: "edited", Rule: "hold",
				Since: remoteT0, Seen: remoteT0,
			}
			e := newRemoteEnv(t, role, []RemoteEntry{entry}, func(app *App) {
				seedRelation(app, &entity.Relation{From: "TKT-001", Type: "blocks", To: "TKT-002"})
			})
			rec := e.do(t, http.MethodGet, remotesPath+"/upstream/conflicts", "")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			page := decodeRemote[remoteConflictPage](t, rec)
			require.Len(t, page.Data, 1)
			require.Equal(t, tc.delete && tc.remove, page.Data[0].Actions["resolve_remote"])
		})
	}
}
