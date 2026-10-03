package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

func TestFieldToken(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{"same value", fieldToken("T-1", "t", "k", "x", true), fieldToken("T-1", "t", "k", "x", true), true},
		{"different value", fieldToken("T-1", "t", "k", "x", true), fieldToken("T-1", "t", "k", "y", true), false},
		{"absent is not null", fieldToken("T-1", "t", "k", nil, false), fieldToken("T-1", "t", "k", nil, true), false},
		{"absent is not empty", fieldToken("T-1", "t", "k", nil, false), fieldToken("T-1", "t", "k", "", true), false},
		{"other entity", fieldToken("T-1", "t", "k", "x", true), fieldToken("T-2", "t", "k", "x", true), false},
		{"other field", fieldToken("T-1", "t", "k", "x", true), fieldToken("T-1", "t", "j", "x", true), false},
		{"int and float agree", fieldToken("T-1", "t", "k", 5, true), fieldToken("T-1", "t", "k", 5.0, true), true},
		{
			"map key order", fieldToken("T-1", "t", "k", map[string]any{"a": 1, "b": 2}, true),
			fieldToken("T-1", "t", "k", map[string]any{"b": 2, "a": 1}, true), true,
		},
		{
			"relations are a set",
			relationsToken("T-1", "t", map[string][]string{"x": {"A", "B"}, "y": {"C"}}),
			relationsToken("T-1", "t", map[string][]string{"y": {"C"}, "x": {"B", "A"}}), true,
		},
		{
			"empty relation list is no relation",
			relationsToken("T-1", "t", map[string][]string{"x": {}}),
			relationsToken("T-1", "t", nil), true,
		},
		{
			"target moved between types",
			relationsToken("T-1", "t", map[string][]string{"x": {"A"}}),
			relationsToken("T-1", "t", map[string][]string{"y": {"A"}}), false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a == tc.b; got != tc.equal {
				t.Errorf("tokens %q and %q: equal=%v, want %v", tc.a, tc.b, got, tc.equal)
			}
			if len(tc.a) != 8 {
				t.Errorf("token %q: want 8 hex characters", tc.a)
			}
		})
	}
}

func TestValidatePreconditionScope(t *testing.T) {
	tok := "00000000"
	tests := []struct {
		name            string
		pre             v1.Preconditions
		props           map[string]any
		unset           []string
		content, rels   bool
		wantOK          bool
		wantPointerTail string
	}{
		{"written property", v1.Preconditions{Properties: map[string]string{"a": tok}},
			map[string]any{"a": 1}, nil, false, false, true, ""},
		{"unset property", v1.Preconditions{Properties: map[string]string{"a": tok}},
			nil, []string{"a"}, false, false, true, ""},
		{"unwritten property", v1.Preconditions{Properties: map[string]string{"a/b": tok}},
			map[string]any{"c": 1}, nil, false, false, false, "/preconditions/properties/a~1b"},
		{"content written", v1.Preconditions{Content: &tok}, nil, nil, true, false, true, ""},
		{"content unwritten", v1.Preconditions{Content: &tok}, nil, nil, false, true, false, "/preconditions/content"},
		{"relations written", v1.Preconditions{Relations: &tok}, nil, nil, false, true, true, ""},
		{"relations unwritten", v1.Preconditions{Relations: &tok}, nil, nil, true, false, false,
			"/preconditions/relations"},
		{"empty", v1.Preconditions{}, nil, nil, false, false, true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ptr, ok := validatePreconditionScope(&tc.pre, preconditionScope{
				props: tc.props, unset: tc.unset, content: tc.content, relations: tc.rels,
			})
			if ok != tc.wantOK || ptr != tc.wantPointerTail {
				t.Errorf("got (%q, %v), want (%q, %v)", ptr, ok, tc.wantPointerTail, tc.wantOK)
			}
		})
	}
}

// getVersions GETs an entity and returns its decoded body.
func getVersions(t *testing.T, app *App, id string) v1.Entity {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/"+id, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", id, rec.Code, rec.Body.String())
	}
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Versions == nil {
		t.Fatalf("GET %s: _versions missing: %s", id, rec.Body.String())
	}
	return got
}

// patchJSON sends a PATCH and returns the recorder.
func patchJSON(app *App, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/"+id, strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", id)
	return rec
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func decodeFieldProblem(t *testing.T, rec *httptest.ResponseRecorder) v1.Error {
	t.Helper()
	var p v1.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v (%s)", err, rec.Body.String())
	}
	return p
}

func TestV1GetEntity_Versions(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}}
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Content: "body",
		Properties: map[string]any{"title": "secret", "status": "open"},
	})

	got := getVersions(t, app, "TKT-001")
	v := got.Versions
	if _, ok := v.Properties["title"]; ok {
		t.Errorf("redacted property must have no token: %v", v.Properties)
	}
	if v.Properties["status"] != fieldToken("TKT-001", "ticket", "status", "open", true) {
		t.Errorf("status token: got %q", v.Properties["status"])
	}
	if v.Properties["docs"] != fieldToken("TKT-001", "ticket", "docs", nil, false) {
		t.Errorf("an unset declared property carries the absent token; got %q", v.Properties["docs"])
	}
	if v.Content == "" || v.Relations == "" {
		t.Errorf("content and relations tokens must be set: %+v", v)
	}
}

func TestV1UpdateEntity_Preconditions(t *testing.T) {
	seed := func(t *testing.T) (*App, *v1.FieldVersions) {
		t.Helper()
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{
			ID: "TKT-001", Type: "ticket", Content: "body",
			Properties: map[string]any{"title": "T", "status": "open"},
		})
		seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F"}})
		return app, getVersions(t, app, "TKT-001").Versions
	}

	t.Run("matching token writes and returns fresh tokens", func(t *testing.T) {
		app, v := seed(t)
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"properties":    map[string]any{"status": "done"},
			"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
		}))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
		}
		var got v1.Entity
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		after := getVersions(t, app, "TKT-001").Versions
		if got.Versions == nil || got.Versions.Properties["status"] != after.Properties["status"] {
			t.Errorf("PATCH _versions must match the next GET; patch=%+v get=%+v", got.Versions, after)
		}
		if after.Properties["status"] == v.Properties["status"] {
			t.Error("status token must change with its value")
		}
		if after.Properties["title"] != v.Properties["title"] || after.Content != v.Content {
			t.Error("tokens of unwritten fields must not change")
		}
	})

	t.Run("a write to another field does not conflict", func(t *testing.T) {
		app, v := seed(t)
		if rec := patchJSON(app, "TKT-001", `{"properties":{"title":"T2"}}`); rec.Code != http.StatusOK {
			t.Fatalf("setup PATCH: %d", rec.Code)
		}
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"properties":    map[string]any{"status": "done"},
			"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
		}))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("stale property token is refused with the conflict", func(t *testing.T) {
		app, v := seed(t)
		if rec := patchJSON(app, "TKT-001", `{"properties":{"status":"blocked"}}`); rec.Code != http.StatusOK {
			t.Fatalf("setup PATCH: %d", rec.Code)
		}
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"properties":    map[string]any{"status": "done"},
			"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
		}))
		if rec.Code != http.StatusPreconditionFailed {
			t.Fatalf("PATCH: got %d, want 412: %s", rec.Code, rec.Body.String())
		}
		p := decodeFieldProblem(t, rec)
		c, ok := p.Conflicts.Properties["status"]
		if p.Conflicts == nil || !ok || c.Expected != v.Properties["status"] {
			t.Fatalf("conflicts: %+v", p.Conflicts)
		}
		want := fieldToken("TKT-001", "ticket", "status", "blocked", true)
		if c.Actual != want || p.Versions == nil || p.Versions.Properties["status"] != want {
			t.Errorf("actual/versions must carry the stored token %q: %+v %+v", want, c, p.Versions)
		}
		stored, _ := app.reader.getEntity(context.Background(), "TKT-001")
		if stored.Properties["status"] != "blocked" {
			t.Errorf("a refused write must not land: status=%v", stored.Properties["status"])
		}
	})

	t.Run("stale content token is refused", func(t *testing.T) {
		app, v := seed(t)
		if rec := patchJSON(app, "TKT-001", `{"content":"theirs"}`); rec.Code != http.StatusOK {
			t.Fatalf("setup PATCH: %d", rec.Code)
		}
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"content": "mine", "preconditions": map[string]any{"content": v.Content},
		}))
		if rec.Code != http.StatusPreconditionFailed || decodeFieldProblem(t, rec).Conflicts.Content == nil {
			t.Fatalf("PATCH: got %d, want 412 with a content conflict: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("stale relations token is refused", func(t *testing.T) {
		app, v := seed(t)
		link := `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`
		if rec := patchJSON(app, "TKT-001", link); rec.Code != http.StatusOK {
			t.Fatalf("setup PATCH: %d %s", rec.Code, rec.Body.String())
		}
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"relations":     map[string]any{"implements": map[string]any{"data": []any{}}},
			"preconditions": map[string]any{"relations": v.Relations},
		}))
		if rec.Code != http.StatusPreconditionFailed || decodeFieldProblem(t, rec).Conflicts.Relations == nil {
			t.Fatalf("PATCH: got %d, want 412 with a relations conflict: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("absent token guards a field both sides fill", func(t *testing.T) {
		app, _ := seed(t)
		seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "T"}})
		v := getVersions(t, app, "TKT-002").Versions
		if rec := patchJSON(app, "TKT-002", `{"properties":{"status":"theirs"}}`); rec.Code != http.StatusOK {
			t.Fatalf("setup PATCH: %d %s", rec.Code, rec.Body.String())
		}
		rec := patchJSON(app, "TKT-002", mustJSON(t, map[string]any{
			"properties":    map[string]any{"status": "mine"},
			"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
		}))
		if rec.Code != http.StatusPreconditionFailed {
			t.Fatalf("PATCH: got %d, want 412: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("precondition on an unwritten field is a 400", func(t *testing.T) {
		app, v := seed(t)
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"properties":    map[string]any{"status": "done"},
			"preconditions": map[string]any{"properties": map[string]string{"title": v.Properties["title"]}},
		}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("PATCH: got %d, want 400: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestV1UpdateEntity_PreconditionOnHiddenFieldIsNotAnOracle pins that a
// precondition cannot be used to test guesses against a redacted value: the
// affordance gate refuses the write before any token is compared, so the
// answer is the same whatever token is sent.
func TestV1UpdateEntity_PreconditionOnHiddenFieldIsNotAnOracle(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "secret"}})

	codes := map[int]bool{}
	for _, tok := range []string{fieldToken("TKT-001", "ticket", "title", "secret", true), "00000000"} {
		rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
			"properties":    map[string]any{"title": "x"},
			"preconditions": map[string]any{"properties": map[string]string{"title": tok}},
		}))
		if rec.Code == http.StatusOK || rec.Code == http.StatusPreconditionFailed {
			t.Fatalf("write to a hidden field: got %d: %s", rec.Code, rec.Body.String())
		}
		codes[rec.Code] = true
	}
	if len(codes) != 1 {
		t.Errorf("right and wrong guesses must be indistinguishable; got statuses %v", codes)
	}
}

// racingMutator lands a concurrent write just before the handler's own
// patch, so the handler's store compare-and-swap loses.
type racingMutator struct {
	entityMutator
	race func()
}

func (m racingMutator) PatchEntity(
	ctx context.Context, id string, p entity.Patch,
) (*entity.UpdateResult, error) {
	m.race()
	return m.entityMutator.PatchEntity(ctx, id, p)
}

func TestV1UpdateEntity_LostRaceReturnsFreshTokens(t *testing.T) {
	tests := []struct {
		name         string
		raced        map[string]any
		wantConflict bool
	}{
		{"race on another field: empty conflicts", map[string]any{"title": "T2"}, false},
		{"race on the same field: conflict", map[string]any{"status": "blocked"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			seedEntity(app, &entity.Entity{
				ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T", "status": "open"},
			})
			v := getVersions(t, app, "TKT-001").Versions
			inner := app.write.manager
			app.write.manager = racingMutator{entityMutator: inner, race: func() {
				if _, err := inner.PatchEntity(context.Background(), "TKT-001",
					entity.Patch{Properties: tc.raced}); err != nil {
					t.Fatalf("racing write: %v", err)
				}
			}}

			rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
				"properties":    map[string]any{"status": "done"},
				"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
			}))
			if rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("PATCH: got %d, want 412: %s", rec.Code, rec.Body.String())
			}
			p := decodeFieldProblem(t, rec)
			if p.Conflicts == nil || p.Versions == nil {
				t.Fatalf("a lost race must carry conflicts and versions: %s", rec.Body.String())
			}
			_, conflicted := p.Conflicts.Properties["status"]
			if conflicted != tc.wantConflict {
				t.Errorf("status conflict: got %v, want %v (%s)", conflicted, tc.wantConflict, rec.Body.String())
			}
			app.write.manager = inner
			if got := getVersions(t, app, "TKT-001").Versions; got.Properties["title"] != p.Versions.Properties["title"] {
				t.Errorf("412 versions must be the stored tokens: %+v vs %+v", p.Versions, got)
			}
		})
	}
}

// hidingGate permits every read until hidden is set, standing in for a
// concurrent write that changes what the row gate depends on (an owner, a
// status-conditioned grant).
type hidingGate struct {
	fakeGate
	hidden *bool
}

func (g hidingGate) PermitsRead(context.Context, string, string) (bool, error) {
	return !*g.hidden, nil
}

func (g hidingGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = !*g.hidden
	}
	return m, nil
}

func (g hidingGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: !*g.hidden, DenyAll: *g.hidden}
}

// TestV1UpdateEntity_LostRaceToHidingWriteCarriesNoTokens pins that the 412
// for a lost race re-reads through the row gate. The winning write hid the
// row from the caller, so its tokens (brute-forceable for a low-entropy
// value) must not be served: a GET would answer 404.
func TestV1UpdateEntity_LostRaceToHidingWriteCarriesNoTokens(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T", "status": "open"},
	})
	v := getVersions(t, app, "TKT-001").Versions
	hidden := false
	inner := app.write.manager
	app.write.manager = racingMutator{entityMutator: inner, race: func() {
		if _, err := inner.PatchEntity(context.Background(), "TKT-001",
			entity.Patch{Properties: map[string]any{"status": "blocked"}}); err != nil {
			t.Fatalf("racing write: %v", err)
		}
		hidden = true
	}}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", strings.NewReader(mustJSON(t, map[string]any{
		"properties":    map[string]any{"status": "done"},
		"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
	})))
	req = req.WithContext(withReadGate(req.Context(), hidingGate{hidden: &hidden}))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("PATCH: got %d, want 412: %s", rec.Code, rec.Body.String())
	}
	p := decodeFieldProblem(t, rec)
	if p.Versions != nil || p.Conflicts != nil {
		t.Errorf("a row hidden by the winning write must not carry tokens: %s", rec.Body.String())
	}
}

// hidingMutator applies the patch, then hides the row, standing in for a
// write that moves the row out of the caller's read grant.
type hidingMutator struct {
	entityMutator
	hidden *bool
}

func (m hidingMutator) PatchEntity(
	ctx context.Context, id string, p entity.Patch,
) (*entity.UpdateResult, error) {
	res, err := m.entityMutator.PatchEntity(ctx, id, p)
	*m.hidden = true
	return res, err
}

// TestV1UpdateEntity_WriteThatHidesRowCarriesNoTokens pins that the PATCH
// success path re-reads through the row gate too: the tokens describe the
// stored row, which may by then hold another node's values.
func TestV1UpdateEntity_WriteThatHidesRowCarriesNoTokens(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"status": "open"}})
	hidden := false
	app.write.manager = hidingMutator{entityMutator: app.write.manager, hidden: &hidden}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"properties":{"status":"done"}}`))
	req = req.WithContext(withReadGate(req.Context(), hidingGate{hidden: &hidden}))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Versions != nil {
		t.Errorf("a row the write hid must not carry tokens: %+v", got.Versions)
	}
}

// TestV1UpdateEntity_VersionsSurviveFSReformat pins the reason the PATCH
// response re-reads the row: fsstore reflows the body on write, so a token
// of the body the manager returned would never match the next GET.
func TestV1UpdateEntity_VersionsSurviveFSReformat(t *testing.T) {
	app := newTestAppV1(t)
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	if err != nil {
		t.Fatal(err)
	}
	fss, err := fsstore.New(fsstore.Config{
		FS: fs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations", AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}, "feature": {Plural: "features"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fss.Close() })
	paths := &project.Context{Root: "/"}
	rebindApp(app, fs, paths, appbuildtest.New(app.Meta(), appbuildtest.WithStore(fss), appbuildtest.WithFS(fs, paths)))
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})
	v := getVersions(t, app, "TKT-001").Versions

	body := "*   " + strings.Repeat("word ", 40) + "\n"
	rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
		"content": body, "preconditions": map[string]any{"content": v.Content},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
	}
	var patched v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	after := getVersions(t, app, "TKT-001")
	if after.Content == body {
		t.Fatal("fsstore did not reformat the body; this fixture no longer exercises the re-read")
	}
	if patched.Versions == nil || patched.Versions.Content != after.Versions.Content {
		t.Fatalf("PATCH content token must match the next GET: patch=%+v get=%+v", patched.Versions, after.Versions)
	}

	rec = patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
		"content": "next", "preconditions": map[string]any{"content": patched.Versions.Content},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("a save based on the PATCH response must succeed: %d %s", rec.Code, rec.Body.String())
	}
}

// TestV1Views_EntryVersionsMatchGet pins that the view's entry carries the
// GET's property and content tokens, so the entity page's autosave is
// guarded, and no relations token, because the entry is not serialized with
// the neighbor filter a PATCH checks against.
func TestV1Views_EntryVersionsMatchGet(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T", "status": "open"}, Content: "body\n",
	})
	get := getVersions(t, app, "TKT-001").Versions

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.views.handleV1Views(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("view: got %d: %s", rec.Code, rec.Body.String())
	}
	var resp v1.ViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	got := resp.Entry.Versions
	if got == nil {
		t.Fatal("view entry carries no _versions")
	}
	if got.Content != get.Content || got.Properties["status"] != get.Properties["status"] {
		t.Errorf("entry tokens differ from GET: %+v vs %+v", got, get)
	}
	if got.Relations != "" {
		t.Errorf("entry must carry no relations token, got %q", got.Relations)
	}
}

// TestV1UpdateEntity_OtherChannelDoesNotCollide pins AC11 (RR-DBL90Y): a
// property save does not change the content token, so a body save issued
// with the token read before it succeeds.
func TestV1UpdateEntity_OtherChannelDoesNotCollide(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"status": "open"}, Content: "body\n",
	})
	v := getVersions(t, app, "TKT-001").Versions

	rec := patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
		"properties":    map[string]any{"status": "done"},
		"preconditions": map[string]any{"properties": map[string]string{"status": v.Properties["status"]}},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("property PATCH: got %d: %s", rec.Code, rec.Body.String())
	}
	rec = patchJSON(app, "TKT-001", mustJSON(t, map[string]any{
		"content": "new body\n", "preconditions": map[string]any{"content": v.Content},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("content PATCH after a property save: got %d: %s", rec.Code, rec.Body.String())
	}
}
