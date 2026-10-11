package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/kvpiles"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newPilesService is a piles service over an in-memory KV.
func newPilesService(t *testing.T) *piles.Service {
	t.Helper()
	mem := storage.NewMemFS()
	if err := mem.MkdirAll("/root", 0o755); err != nil {
		t.Fatal(err)
	}
	rfs, err := storage.NewRootedFS(mem, "/root")
	if err != nil {
		t.Fatal(err)
	}
	st, err := kvpiles.New(state.NewFSKV(rfs), kvpiles.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := piles.NewService(st, piles.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// pilesTestMeta is facedMeta with a display property and two transforms.
func pilesTestMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta := facedMeta(t)
	for _, typ := range []string{"policy", "feature"} {
		def := meta.Entities[typ]
		def.DisplayProperty = "title"
		meta.Entities[typ] = def
	}
	meta.Transforms = map[string]metamodel.TransformDef{
		"copy":  {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
		"other": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
	}
	return meta
}

// pilesPolicy grants alice and bob the reader role over read.
func pilesPolicy(t *testing.T, st store.Store, read ...string) *acl.Declarative {
	t.Helper()
	return mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: read}},
		Assignments: map[string]string{"alice": "reader", "bob": "reader"},
	}, st)
}

// newPilesApp is facedApp (POL-1 at draft and published, FEAT-1) with piles
// wired and every type readable.
func newPilesApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app, d := facedAppWith(t, pilesTestMeta(t), policyPublishedScope(), func(st store.Store) *acl.Declarative {
		return pilesPolicy(t, st, "policy", "feature")
	})
	if err := app.SetPiles(newPilesService(t), stubScriptPiles{}); err != nil {
		t.Fatal(err)
	}
	return app, d
}

// publishedWorld is a request world that is not the generated default.
func publishedWorld() *worldHandle {
	return &worldHandle{name: "published", scope: policyPublishedScope()}
}

// pileCall describes one request to the piles routes.
type pileCall struct {
	user   string
	d      *acl.Declarative
	world  *worldHandle
	method string
	path   string
	body   string
}

// pileContext is the context a request in c runs under: the principal, its
// read gate, and the world.
func pileContext(t *testing.T, c pileCall) context.Context {
	t.Helper()
	ctx := context.Background()
	if c.user != "" {
		ctx = principalCtx(c.user)
	}
	// A principal with no identity gets no gate: the ACL refuses to build
	// one, and the piles routes answer before reading anything.
	if c.d != nil && c.user != "" && c.user != principal.Unknown {
		ctx = gateCtxFor(ctx, t, c.d)
	}
	if c.world != nil {
		ctx = withWorld(ctx, *c.world)
	}
	return ctx
}

// doPile runs c through the piles handler.
func doPile(t *testing.T, app *App, c pileCall) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body)).WithContext(pileContext(t, c))
	rec := httptest.NewRecorder()
	app.piles.handleV1Piles(rec, req)
	return rec
}

// createPile makes a pile as user and returns it.
func createPile(t *testing.T, app *App, d *acl.Declarative, user, name string, items ...string) v1.Pile {
	t.Helper()
	body, _ := json.Marshal(v1.PileCreateRequest{Name: name, Items: items})
	rec := doPile(t, app, pileCall{user: user, d: d, method: http.MethodPost, path: pilesPath, body: string(body)})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var p v1.Pile
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func decodePile(t *testing.T, rec *httptest.ResponseRecorder) v1.Pile {
	t.Helper()
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var p v1.Pile
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func listPiles(t *testing.T, app *App, c pileCall) v1.PileList {
	t.Helper()
	c.method, c.path = http.MethodGet, pilesPath
	rec := doPile(t, app, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var out v1.PileList
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func addresses(p v1.Pile) []string {
	out := make([]string, len(p.Items))
	for i, it := range p.Items {
		out[i] = it.Address
	}
	return out
}

// pilePosition asks _position about id in pile's scope.
func pilePosition(t *testing.T, app *App, c pileCall, pileID, id string) *httptest.ResponseRecorder {
	t.Helper()
	scope, _ := json.Marshal(ScopeDescriptor{Source: "pile", Pile: pileID})
	target := "/api/v1/_position?id=" + url.QueryEscape(id) + "&scope=" + url.QueryEscape(string(scope))
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody).WithContext(pileContext(t, c))
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)
	return rec
}

func TestPiles_CreateListAndOrder(t *testing.T) {
	app, d := newPilesApp(t)
	p := createPile(t, app, d, "alice", "Reading", "FEAT-1", "POL-1@published")
	if p.Count != 2 || p.Icon != piles.DefaultIcon || !strings.HasPrefix(p.ID, "PIL-") {
		t.Fatalf("created pile = %+v", p)
	}
	if got := addresses(p); strings.Join(got, ",") != "FEAT-1,POL-1@published" {
		t.Fatalf("items = %v, want the first item newest", got)
	}
	pub := p.Items[1]
	if pub.ID != "POL-1" || pub.Face != "published" || pub.Type != "policy" || pub.Title != "PUBLISHED TEXT" {
		t.Fatalf("faced item = %+v", pub)
	}

	// A later add goes on top; a duplicate is not added again.
	rec := doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodPost,
		path: pilesPath + "/" + p.ID + "/items", body: `{"items":["POL-1@draft","FEAT-1"]}`})
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"added":1}` {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	got := decodePile(t, doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet, path: pilesPath + "/" + p.ID}))
	if strings.Join(addresses(got), ",") != "POL-1@draft,FEAT-1,POL-1@published" {
		t.Fatalf("items after add = %v", addresses(got))
	}

	list := listPiles(t, app, pileCall{user: "alice", d: d})
	if len(list.Piles) != 1 || list.Piles[0].Count != 3 || len(list.Icons) != len(piles.Icons) {
		t.Fatalf("list = %+v", list)
	}

	// Rename and delete.
	rec = doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodPatch,
		path: pilesPath + "/" + p.ID, body: `{"name":"Renamed","icon":"star"}`})
	if got := decodePile(t, rec); got.Name != "Renamed" || got.Icon != "star" {
		t.Fatalf("patch = %+v", got)
	}
	rec = doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodDelete, path: pilesPath + "/" + p.ID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if list := listPiles(t, app, pileCall{user: "alice", d: d}); len(list.Piles) != 0 {
		t.Fatalf("piles after delete = %+v", list.Piles)
	}
}

func TestPiles_ErrorMapping(t *testing.T) {
	app, d := newPilesApp(t)
	p := createPile(t, app, d, "alice", "Reading")
	itemsPath := pilesPath + "/" + p.ID + "/items"
	tooMany := `{"items":["FEAT-1"` + strings.Repeat(`,"FEAT-1"`, piles.MaxItems) + `]}`
	cases := []struct {
		name       string
		call       pileCall
		wantStatus int
		wantType   string
	}{
		{"duplicate name, other case", pileCall{method: http.MethodPost, path: pilesPath, body: `{"name":"READING"}`},
			http.StatusConflict, "pile_name_taken"},
		{"bad icon", pileCall{method: http.MethodPost, path: pilesPath, body: `{"name":"x","icon":"nope"}`},
			http.StatusBadRequest, "invalid_request"},
		{"empty name", pileCall{method: http.MethodPost, path: pilesPath, body: `{"name":""}`},
			http.StatusBadRequest, "invalid_request"},
		{"bad json", pileCall{method: http.MethodPost, path: pilesPath, body: `{`},
			http.StatusBadRequest, "invalid_request"},
		{"too many items", pileCall{method: http.MethodPost, path: itemsPath, body: tooMany},
			http.StatusBadRequest, "invalid_request"},
		{"body too large", pileCall{method: http.MethodPost, path: itemsPath,
			body: `{"items":["` + strings.Repeat("A", pilesBodyLimit) + `"]}`},
			http.StatusRequestEntityTooLarge, "body_too_large"},
		{"unknown item", pileCall{method: http.MethodPost, path: itemsPath, body: `{"items":["FEAT-404"]}`},
			http.StatusNotFound, "item_not_found"},
		{"unknown face", pileCall{method: http.MethodPost, path: itemsPath, body: `{"items":["POL-1@nope"]}`},
			http.StatusNotFound, "item_not_found"},
		{"ambiguous bare id", pileCall{method: http.MethodPost, path: itemsPath, body: `{"items":["POL-1"]}`},
			http.StatusConflict, "ambiguous_address"},
		{"malformed pile id", pileCall{method: http.MethodGet, path: pilesPath + "/not-a-pile"},
			http.StatusNotFound, "pile_not_found"},
		{"unknown route", pileCall{method: http.MethodGet, path: pilesPath + "/" + p.ID + "/nope"},
			http.StatusNotFound, "not_found"},
		{"wrong method", pileCall{method: http.MethodPut, path: pilesPath},
			http.StatusMethodNotAllowed, "method_not_allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.call.user, tc.call.d = "alice", d
			rec := doPile(t, app, tc.call)
			if rec.Code != tc.wantStatus || !strings.Contains(rec.Body.String(), "/errors/"+tc.wantType+`"`) {
				t.Fatalf("got %d %s, want %d %s", rec.Code, rec.Body, tc.wantStatus, tc.wantType)
			}
		})
	}

	t.Run("ambiguous names the faces", func(t *testing.T) {
		rec := doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodPost, path: itemsPath,
			body: `{"items":["POL-1"]}`})
		var e v1.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if strings.Join(e.Faces, ",") != "POL-1@draft,POL-1@published" || !strings.Contains(e.Detail, "POL-1@draft") {
			t.Fatalf("ambiguous body = %+v", e)
		}
	})

	t.Run("limit", func(t *testing.T) {
		for i := 1; i < piles.MaxPiles; i++ {
			createPile(t, app, d, "alice", "p"+strings.Repeat("x", i))
		}
		rec := doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodPost, path: pilesPath,
			body: `{"name":"one too many"}`})
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "/errors/pile_limit") {
			t.Fatalf("51st pile: %d %s", rec.Code, rec.Body)
		}
	})
}

// A user without an owner identity has no piles: 403 on every route and
// piles_available false on the sidebar.
func TestPiles_NoOwner(t *testing.T) {
	app, d := newPilesApp(t)
	for _, user := range []string{"", principal.Unknown} {
		rec := doPile(t, app, pileCall{user: user, d: d, method: http.MethodGet, path: pilesPath})
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "/errors/no_owner") {
			t.Fatalf("user %q: %d %s", user, rec.Code, rec.Body)
		}
		if app.piles.available(pileContext(t, pileCall{user: user})) {
			t.Fatalf("user %q: piles available", user)
		}
	}
	if !app.piles.available(principalCtx("alice")) {
		t.Fatal("alice: piles not available")
	}
}

func TestPiles_SidebarAndConfigBootstrap(t *testing.T) {
	app, d := newPilesApp(t)
	s := app.State()
	cfg := *s.Cfg
	cfg.Piles = &dataentryconfig.PilesConfig{Export: []string{"copy"}}
	app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta})

	sidebar := func(user string) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).
			WithContext(pileContext(t, pileCall{user: user, d: d}))
		rec := httptest.NewRecorder()
		app.views.handleV1Sidebar(rec, req)
		var out map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("sidebar: %v %s", err, rec.Body)
		}
		return out
	}
	got := sidebar("alice")
	if string(got["piles_available"]) != "true" || string(got["piles"]) != `{"actions":[],"export":["copy"]}` {
		t.Fatalf("alice sidebar piles = %s %s", got["piles_available"], got["piles"])
	}
	if got := sidebar(principal.Unknown); string(got["piles_available"]) != "false" {
		t.Fatalf("unknown sidebar piles_available = %s", got["piles_available"])
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Config(rec, req)
	var conf map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &conf); err != nil {
		t.Fatal(err)
	}
	if string(conf["piles"]) != `{"actions":[],"export":["copy"]}` {
		t.Fatalf("_config piles = %s", conf["piles"])
	}

	// Without the service: unavailable, and the routes 404.
	app.piles.svc = nil
	if got := sidebar("alice"); string(got["piles_available"]) != "false" {
		t.Fatalf("unwired piles_available = %s", got["piles_available"])
	}
	if rec := doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet, path: pilesPath}); rec.Code != http.StatusNotFound {
		t.Fatalf("unwired list: %d", rec.Code)
	}
}

func TestSetPiles_RejectsNil(t *testing.T) {
	app, _ := newPilesApp(t)
	if err := app.SetPiles(nil, nil); err == nil {
		t.Fatal("SetPiles(nil) succeeded")
	}
}

// normalizedBody replaces the pile id in a response body, so a foreign
// pile's answer can be compared with an unknown one's byte for byte.
func normalizedBody(rec *httptest.ResponseRecorder, id string) string {
	return strconv.Itoa(rec.Code) + strings.ReplaceAll(rec.Body.String(), id, "PIL-X")
}

// Another user's pile answers exactly like a pile that does not exist, on
// every route.
func TestPiles_OwnerIsolation(t *testing.T) {
	requireCp(t)
	app, d := newPilesApp(t)
	mine := createPile(t, app, d, "alice", "Private", "FEAT-1")
	const unknown = "PIL-ZZZZZZZZ"
	routes := []struct {
		name, method, suffix, body string
	}{
		{"get", http.MethodGet, "", ""},
		{"patch", http.MethodPatch, "", `{"name":"stolen"}`},
		{"delete", http.MethodDelete, "", ""},
		{"add", http.MethodPost, "/items", `{"items":["FEAT-1"]}`},
		{"remove", http.MethodPost, "/items/_remove", `{"items":["FEAT-1"]}`},
		{"export", http.MethodGet, "/_export?transform=copy", ""},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			foreign := doPile(t, app, pileCall{user: "bob", d: d, method: rt.method,
				path: pilesPath + "/" + mine.ID + rt.suffix, body: rt.body})
			missing := doPile(t, app, pileCall{user: "bob", d: d, method: rt.method,
				path: pilesPath + "/" + unknown + rt.suffix, body: rt.body})
			if foreign.Code != http.StatusNotFound {
				t.Fatalf("foreign: %d %s", foreign.Code, foreign.Body)
			}
			if normalizedBody(foreign, mine.ID) != normalizedBody(missing, unknown) {
				t.Fatalf("foreign and unknown differ:\n%s\n%s", foreign.Body, missing.Body)
			}
		})
	}
	t.Run("scope", func(t *testing.T) {
		foreign := pilePosition(t, app, pileCall{user: "bob", d: d}, mine.ID, "FEAT-1")
		missing := pilePosition(t, app, pileCall{user: "bob", d: d}, unknown, "FEAT-1")
		if foreign.Code != http.StatusNotFound || foreign.Body.String() != missing.Body.String() {
			t.Fatalf("scope: %d %s vs %s", foreign.Code, foreign.Body, missing.Body)
		}
	})
	// The pile is untouched, and bob's list holds nothing of alice's.
	got := decodePile(t, doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet, path: pilesPath + "/" + mine.ID}))
	if got.Name != "Private" || got.Count != 1 {
		t.Fatalf("alice's pile after bob = %+v", got)
	}
	theirs := createPile(t, app, d, "bob", "Private", "FEAT-1")
	if list := listPiles(t, app, pileCall{user: "bob", d: d}); len(list.Piles) != 1 || list.Piles[0].ID != theirs.ID {
		t.Fatalf("bob sees %+v", list.Piles)
	}
}

// A hidden item is absent from every read, counts included, and comes back
// when access returns. Removing it answers like removing anything else.
func TestPiles_HiddenItem(t *testing.T) {
	requireCp(t)
	app, all := newPilesApp(t)
	p := createPile(t, app, all, "alice", "Mixed", "FEAT-1", "POL-1@published")
	policyOnly := pilesPolicy(t, app.store, "policy")

	check := func(d *acl.Declarative, want string) {
		t.Helper()
		got := decodePile(t, doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet, path: pilesPath + "/" + p.ID}))
		if strings.Join(addresses(got), ",") != want || got.Count != len(got.Items) {
			t.Fatalf("items = %v (count %d), want %s", addresses(got), got.Count, want)
		}
		list := listPiles(t, app, pileCall{user: "alice", d: d})
		if list.Piles[0].Count != got.Count {
			t.Fatalf("list count %d, pile count %d", list.Piles[0].Count, got.Count)
		}
		rec := pilePosition(t, app, pileCall{user: "alice", d: d}, p.ID, "POL-1@published")
		var pos v1.Position
		if err := json.Unmarshal(rec.Body.Bytes(), &pos); err != nil || pos.Total != got.Count {
			t.Fatalf("position: %d %s, want total %d", rec.Code, rec.Body, got.Count)
		}
		rec = doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet,
			path: pilesPath + "/" + p.ID + "/_export?transform=copy"})
		if rec.Code != http.StatusOK {
			t.Fatalf("export: %d %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "FEAT-1") != strings.Contains(want, "FEAT-1") {
			t.Fatalf("export = %s, want items %s", rec.Body, want)
		}
	}
	check(policyOnly, "POL-1@published")
	if rec := pilePosition(t, app, pileCall{user: "alice", d: policyOnly}, p.ID, "FEAT-1"); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden item position: %d %s", rec.Code, rec.Body)
	}
	// An add naming the hidden item is the uniform 404.
	rec := doPile(t, app, pileCall{user: "alice", d: policyOnly, method: http.MethodPost,
		path: pilesPath + "/" + p.ID + "/items", body: `{"items":["FEAT-1"]}`})
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "/errors/item_not_found") {
		t.Fatalf("hidden add: %d %s", rec.Code, rec.Body)
	}
	check(all, "FEAT-1,POL-1@published")

	// _remove: hidden, deleted and never added answer identically.
	if err := app.store.CreateEntity(context.Background(), &entity.Entity{ID: "FEAT-2", Type: "feature"}); err != nil {
		t.Fatal(err)
	}
	removeBody := func(addr string) string {
		rec := doPile(t, app, pileCall{user: "alice", d: policyOnly, method: http.MethodPost,
			path: pilesPath + "/" + p.ID + "/items/_remove", body: `{"items":["` + addr + `"]}`})
		return strconv.Itoa(rec.Code) + rec.Body.String()
	}
	hidden, never := removeBody("FEAT-1"), removeBody("FEAT-2")
	if _, err := app.store.DeleteFamily(context.Background(), "FEAT-2", false); err != nil {
		t.Fatal(err)
	}
	deleted, malformed := removeBody("FEAT-2"), removeBody("not an id")
	if hidden != "204" || never != hidden || deleted != hidden || malformed != hidden {
		t.Fatalf("remove answers differ: %q %q %q %q", hidden, never, deleted, malformed)
	}
	check(all, "POL-1@published")
}

// _remove trims each address, as an add does, so an address pasted with
// surrounding whitespace removes the item it names.
func TestPiles_RemoveTrimsAddresses(t *testing.T) {
	app, d := newPilesApp(t)
	p := createPile(t, app, d, "alice", "Trim", "FEAT-1", "POL-1@published")
	rec := doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodPost,
		path: pilesPath + "/" + p.ID + "/items/_remove", body: `{"items":["  FEAT-1\t", " POL-1@published "]}`})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	got := decodePile(t, doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet, path: pilesPath + "/" + p.ID}))
	if len(got.Items) != 0 {
		t.Fatalf("items = %v, want none left", addresses(got))
	}
}

// A pile holds faces: stepping walks them by address, and the panel count
// equals the _position total in a non-default world.
func TestPiles_PositionAcrossFaces(t *testing.T) {
	app, d := newPilesApp(t)
	p := createPile(t, app, d, "alice", "Faces", "FEAT-1", "POL-1@published", "POL-1@draft")
	call := pileCall{user: "alice", d: d, world: publishedWorld()}

	rec := pilePosition(t, app, call, p.ID, "POL-1@published")
	var pos v1.Position
	if err := json.Unmarshal(rec.Body.Bytes(), &pos); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("position: %d %s", rec.Code, rec.Body)
	}
	if pos.Current != 2 || pos.Prev == nil || pos.Prev.Address != "FEAT-1" ||
		pos.Next == nil || pos.Next.Address != "POL-1@draft" || pos.Next.ID != "POL-1" || pos.Next.Type != "policy" {

		t.Fatalf("position = %+v prev %+v next %+v", pos, pos.Prev, pos.Next)
	}
	list := listPiles(t, app, call)
	if list.Piles[0].Count != pos.Total {
		t.Fatalf("panel count %d, position total %d", list.Piles[0].Count, pos.Total)
	}
	got := call
	got.method, got.path = http.MethodGet, pilesPath+"/"+p.ID
	if one := decodePile(t, doPile(t, app, got)); one.Count != pos.Total || len(one.Items) != pos.Total {
		t.Fatalf("pile count %d (%d items), position total %d", one.Count, len(one.Items), pos.Total)
	}
	rec = pilePosition(t, app, call, p.ID, "POL-1@draft")
	if err := json.Unmarshal(rec.Body.Bytes(), &pos); err != nil || pos.Current != 3 || pos.Next != nil {
		t.Fatalf("draft position: %d %s", rec.Code, rec.Body)
	}
}

func TestPiles_ExportTransformAllowlist(t *testing.T) {
	requireCp(t)
	app, d := newPilesApp(t)
	p := createPile(t, app, d, "alice", "Out", "FEAT-1")
	s := app.State()
	cfg := *s.Cfg
	cfg.Piles = &dataentryconfig.PilesConfig{Export: []string{"copy"}}
	app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta})

	export := func(transform string) *httptest.ResponseRecorder {
		return doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet,
			path: pilesPath + "/" + p.ID + "/_export?transform=" + transform})
	}
	rec := export("copy")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "| FEAT-1 | Feature | f |") {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("export headers = %v", rec.Header())
	}
	for _, name := range []string{"other", "missing"} {
		if rec := export(name); rec.Code != http.StatusNotFound ||
			!strings.Contains(rec.Body.String(), "/errors/unknown_transform") {

			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestScopeFromParam_Pile(t *testing.T) {
	meta := facedMeta(t)
	cases := []struct {
		raw    string
		wantOK bool
	}{
		{`{"source":"pile","pile":"PIL-ABCD"}`, true},
		{`{"source":"pile"}`, false},
		{`{"source":"pile","pile":"PIL-ABCD","type":"policy"}`, false},
		{`{"source":"pile","pile":"PIL-ABCD","q":"x"}`, false},
		{`{"source":"pile","pile":"PIL-ABCD","filters":{"filter[a]":"b"}}`, false},
		{`{"source":"pile","pile":"PIL-ABCD","sort":"title"}`, false},
		{`{"source":"list","type":"policy","pile":"PIL-ABCD"}`, false},
		{`{"source":"search","q":"x","pile":"PIL-ABCD"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if _, ok, reason := scopeFromParam(tc.raw, meta); ok != tc.wantOK {
				t.Fatalf("ok = %v (%s), want %v", ok, reason, tc.wantOK)
			}
		})
	}
}

// stubScriptPiles satisfies ScriptPiles for tests that exercise the HTTP
// routes only.
type stubScriptPiles struct{}

func (stubScriptPiles) ListPiles(context.Context) ([]lua.PileSummary, error) { return nil, nil }

func (stubScriptPiles) FindPile(context.Context, string) (lua.PileSummary, error) {
	return lua.PileSummary{}, piles.ErrNotFound
}

func (stubScriptPiles) PushToPile(context.Context, lua.PilePush) (int, error) { return 0, nil }

func (stubScriptPiles) RemoveFromPile(context.Context, string, []entity.Ref) error { return nil }
