package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// worldsMeta is a metamodel with both content-state axes the demo project
// carries: a lifecycle (draft -> published, `otherwise: exclude`) and a
// language family (en/nl/fr, `otherwise: default`). The two differ in the
// answer they give to rule 3, which is the whole point of having both.
func worldsMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {
				Label: "Policy",
				Pointers: map[string]metamodel.PointerDef{
					"draft":     {Default: true},
					"published": {},
				},
			},
			"blog-post": {
				Label: "Blog post",
				Pointers: map[string]metamodel.PointerDef{
					"en": {Default: true},
					"nl": {},
					"fr": {},
				},
			},
			// A pointerless type, present so the tests cover resolution
			// rule 1 alongside the two pointered ones.
			"ticket": {Label: "Ticket"},
		},
		Worlds: map[string]metamodel.WorldDef{
			"published": {
				Select:    []string{"published"},
				Otherwise: metamodel.OtherwiseExclude,
			},
			"site-nl": {
				Select:    []string{"nl", "en"},
				Otherwise: metamodel.OtherwiseDefault,
				Overrides: map[string][]string{"policy": {"published"}},
			},
		},
	}
}

// TestSchemaWorlds_EnumeratesDeclaredPlusDefault pins TKT-WRLDAPI item 1: a
// client can discover every legal `?world=` value, including the implicit
// default world it would otherwise have to know by convention.
func TestSchemaWorlds_EnumeratesDeclaredPlusDefault(t *testing.T) {
	got := schemaWorlds(context.Background(), worldsMeta())

	for _, name := range []string{"default", "published", "site-nl"} {
		if _, ok := got[name]; !ok {
			t.Errorf("world %q missing from the enumeration; a client cannot "+
				"select a world it cannot discover", name)
		}
	}
	if len(got) != 3 {
		t.Errorf("got %d worlds, want exactly the 2 declared + the default one: %v",
			len(got), got)
	}

	def := got["default"]
	if !def.Default || !def.Readable {
		t.Errorf("the default world is always present and always selectable; got %+v", def)
	}
	if len(def.Select) != 0 || def.Otherwise != "" {
		t.Errorf("the default world resolves every entity to its default state by "+
			"construction and never reaches rule 3, so it carries no chain or "+
			"otherwise; got %+v", def)
	}

	pub := got["published"]
	if len(pub.Select) != 1 || pub.Select[0] != "published" {
		t.Errorf("published.select = %v, want [published]", pub.Select)
	}
	if pub.Otherwise != string(metamodel.OtherwiseExclude) {
		t.Errorf("published.otherwise = %q, want %q — a public world excludes, and "+
			"a client that renders this wrong tells the user an unpublished entity "+
			"is merely missing", pub.Otherwise, metamodel.OtherwiseExclude)
	}
	if pub.Default {
		t.Error("only the implicit default world carries default=true")
	}

	nl := got["site-nl"]
	if len(nl.Select) != 2 || nl.Select[0] != "nl" || nl.Select[1] != "en" {
		t.Errorf("site-nl.select = %v, want [nl en] — chain ORDER is the entire "+
			"semantic content of a world, so a serializer that reorders it "+
			"describes a different world", nl.Select)
	}
	if chain := nl.Overrides["policy"]; len(chain) != 1 || chain[0] != "published" {
		t.Errorf("site-nl.overrides[policy] = %v, want [published]", chain)
	}
}

// TestSchemaWorlds_DeclaredSetIsPrincipalIndependent pins the CLAUDE.md
// config-is-not-secret rule at this surface: a principal who may not read a
// world still SEES that it is declared, and learns only that they may not
// select it.
//
// The failure this guards is a well-meant "filter what they can't use", which
// would leave a UI unable to distinguish "no such world" from "not for you"
// while concealing nothing — the world names live in the operator's
// schema.yaml, routinely a public repo.
//
// Mutation-checked (Ruling 10): this is a NEGATIVE assertion about a security
// posture, so it would pass trivially against code that never ran the branch.
// Verified to die when schemaWorlds skips a world whose readable is false.
func TestSchemaWorlds_DeclaredSetIsPrincipalIndependent(t *testing.T) {
	// A gate that permits `published` and denies `site-nl`.
	ctx := withReadGate(context.Background(), worldGate{
		permit: map[string]bool{"published": true},
	})
	got := schemaWorlds(ctx, worldsMeta())

	if _, ok := got["site-nl"]; !ok {
		t.Fatal("a world the principal may not READ must still be ENUMERATED: " +
			"its name is operator-authored config, not a secret")
	}
	if got["site-nl"].Readable {
		t.Error("site-nl is denied by the gate, so readable must be false")
	}
	if !got["published"].Readable {
		t.Error("published is permitted by the gate, so readable must be true")
	}
	// The chain of a denied world is still described: it says which
	// coordinates the world would prefer, never which faces any row holds.
	if len(got["site-nl"].Select) != 2 {
		t.Error("a denied world still carries its declared chain — that is config, " +
			"and describing it discloses nothing about the world's CONTENTS")
	}
}

// TestSchemaWorlds_GateErrorFailsClosed pins the direction of the failure: an
// unanswerable grant check reports NOT readable.
//
// Reporting readable would be fail-open in effect rather than in form — the
// selector would offer a world whose every request then comes back empty,
// which a user reads as "nothing is published" rather than as the outage it
// is. That silent-in-the-direction-of-the-wrong-answer shape is the one this
// arc keeps hitting.
//
// Mutation-checked (Ruling 10): verified to die when the error arm is flipped
// to `readable = true`, and again when the arm drops the world entirely.
func TestSchemaWorlds_GateErrorFailsClosed(t *testing.T) {
	ctx := withReadGate(context.Background(), worldGate{fail: true})
	got := schemaWorlds(ctx, worldsMeta())

	if got["published"].Readable {
		t.Error("PermitsWorld failed, so readability is UNKNOWN and must be " +
			"reported as not-readable")
	}
	if _, ok := got["published"]; !ok {
		t.Error("a gate failure must not drop the world from the enumeration — " +
			"existence is config and does not depend on the gate")
	}
	if !got["default"].Readable {
		t.Error("the default world short-circuits the grant check entirely, so a " +
			"gate outage must not make today's graph unselectable")
	}
}

// TestSchemaWorlds_DefaultWorldAgreesWithTheRequestPath pins that the
// enumeration and the request path make the SAME decision about the default
// world.
//
// `resolveWorld` short-circuits `default` before any grant check, so a request
// for the default world is always served. This endpoint must therefore report
// it readable, or the selector contradicts the server about a request the
// server will in fact answer.
//
// The gate is NOT consulted, and that is the whole point of the test: a gate
// denying every world must still leave the default world readable here,
// because the request path never asks it.
//
// # What this test does NOT claim
//
// It does not claim a default-world denial is inexpressible. It is:
// `acl.roleGrantsWorldRead` has an explicit default arm and
// `compiledCeiling.permitsWorld` implements `deny_worlds: [default]`. The
// request path simply does not consult them — a pre-existing gap this
// enumeration mirrors rather than papers over.
//
// So this test pins AGREEMENT between two paths, not a claim about the ACL.
// When the request path starts honoring a default-world denial, this test
// SHOULD fail, and the correct response is to make this endpoint ask the gate
// too — not to weaken the assertion.
func TestSchemaWorlds_DefaultWorldAgreesWithTheRequestPath(t *testing.T) {
	denyAll := withReadGate(context.Background(), worldGate{permit: nil})
	got := schemaWorlds(denyAll, worldsMeta())

	if !got["default"].Readable {
		t.Error("the request path short-circuits `default` before any grant " +
			"check, so the enumeration must report it readable too — asking " +
			"the gate here reports today's graph as unreadable while every " +
			"request for it succeeds")
	}
	if got["published"].Readable {
		t.Error("precondition: this gate denies every DECLARED world, so the " +
			"default arm above is the only thing under test")
	}
}

// TestSchemaWorlds_DoesNotAliasTheMetamodel pins that a response cannot
// re-scope every reader's world.
//
// The metamodel is a pointer shared by every assembled Services
// (appbuild.SharedBase), so a serializer handing out its backing slices lets
// one consumer's in-place sort turn `select: [nl, en]` into `select: [en, nl]`
// for every tenant. That is the unbounded direction: it silently serves the
// wrong face.
//
// Mutation-checked (Ruling 10): a copy-assertion passes trivially if the
// mutation cannot reach the source. Verified to die when both Select and
// Overrides are assigned straight from the metamodel.
func TestSchemaWorlds_DoesNotAliasTheMetamodel(t *testing.T) {
	meta := worldsMeta()
	got := schemaWorlds(context.Background(), meta)

	got["site-nl"].Select[0] = "MUTATED"
	got["site-nl"].Overrides["policy"][0] = "MUTATED"

	if chain := meta.Worlds["site-nl"].Select; chain[0] != "nl" {
		t.Errorf("mutating the response re-scoped the shared metamodel: "+
			"select is now %v", chain)
	}
	if chain := meta.Worlds["site-nl"].Overrides["policy"]; chain[0] != "published" {
		t.Errorf("mutating the response re-scoped the shared metamodel: "+
			"override is now %v", chain)
	}
}

// TestSchemaPointers pins TKT-WRLDAPI item 3, including the byte-parity floor:
// a pointerless type must emit NO key, so a project that declares no content
// states is unchanged on the wire.
func TestSchemaPointers(t *testing.T) {
	meta := worldsMeta()

	policy := schemaPointers(meta.Entities["policy"])
	if len(policy) != 2 {
		t.Fatalf("policy declares draft+published; got %v", policy)
	}
	if !policy["draft"].Default {
		t.Error("draft is policy's default coordinate — the state a bare id addresses")
	}
	if policy["published"].Default {
		t.Error("only one coordinate per type may be the default")
	}

	if got := schemaPointers(meta.Entities["ticket"]); got != nil {
		t.Errorf("a pointerless type must emit no pointers key, keeping a "+
			"content-state-free project byte-identical on the wire; got %v", got)
	}
}

// TestSchemaEndpoint_ServesWorldsAndPointers is the wire-level check: the two
// additive keys actually reach a client, through the real router.
//
// It also pins that BOTH schema surfaces agree about a type's pointers. They
// were two inline copies of the same serializer before this change, and a
// client that discovers a type's content states from one endpoint and not the
// other has no way to tell which answer is authoritative.
func TestSchemaEndpoint_ServesWorldsAndPointers(t *testing.T) {
	app := newTestAppV1(t)
	meta := app.State().Meta
	meta.Worlds = map[string]metamodel.WorldDef{
		"published": {Select: []string{"published"}, Otherwise: metamodel.OtherwiseExclude},
	}
	td := meta.Entities["ticket"]
	td.Pointers = map[string]metamodel.PointerDef{
		"draft": {Default: true}, "published": {},
	}
	meta.Entities["ticket"] = td

	var schema v1.Schema
	getJSON(t, app, "/api/v1/_schema", &schema)

	if _, ok := schema.Worlds["published"]; !ok {
		t.Errorf("the schema handshake must enumerate declared worlds; got %v",
			schema.Worlds)
	}
	if _, ok := schema.Worlds["default"]; !ok {
		t.Error("the implicit default world must be enumerated too")
	}
	if got := schema.Entities["ticket"].Pointers; len(got) != 2 {
		t.Errorf("ticket declares draft+published; the schema served %v", got)
	}
	if !schema.Entities["ticket"].Pointers["draft"].Default {
		t.Error("draft is the default coordinate")
	}

	// The single-type route must agree, field for field.
	var single v1.EntityType
	getJSON(t, app, "/api/v1/_schema/types/ticket", &single)
	if len(single.Pointers) != len(schema.Entities["ticket"].Pointers) {
		t.Errorf("the two schema surfaces disagree about ticket's pointers: "+
			"/_schema says %v, /_schema/types/ticket says %v",
			schema.Entities["ticket"].Pointers, single.Pointers)
	}
	if single.Label != schema.Entities["ticket"].Label ||
		single.Plural != schema.Entities["ticket"].Plural ||
		len(single.Properties) != len(schema.Entities["ticket"].Properties) {

		t.Error("the two schema surfaces must render an entity type identically; " +
			"they share toV1EntityType precisely so they cannot drift")
	}
}

// TestSchemaEndpoint_PointerlessProjectUnchanged is the backward-compatibility
// floor: a project declaring no worlds and no pointers gains exactly ONE key
// (the default world) and no per-type ones.
func TestSchemaEndpoint_PointerlessProjectUnchanged(t *testing.T) {
	app := newTestAppV1(t) // fixture metamodel declares neither

	var schema v1.Schema
	getJSON(t, app, "/api/v1/_schema", &schema)

	if len(schema.Worlds) != 1 || !schema.Worlds["default"].Default {
		t.Errorf("a project with no worlds: block has exactly the default world; got %v",
			schema.Worlds)
	}
	for name, et := range schema.Entities {
		if et.Pointers != nil {
			t.Errorf("type %q declares no pointers, so the key must be absent; got %v",
				name, et.Pointers)
		}
	}
}

// --- item 2: face provenance -------------------------------------------

// TestResolutionRule pins the (scope, pointer) -> rule mapping that labels a
// served face (TKT-WRLDAPI item 2).
//
// The mapping is total over what the store can hand back, which is why it is a
// lookup rather than a chain walk: resolution already happened in the store,
// and a second walk here would be a second implementation of the semantics
// that decide which face a reader sees.
func TestResolutionRule(t *testing.T) {
	// `site-nl`: blog-post prefers nl then en, falling back to the default
	// state; policy is overridden to published-or-exclude.
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"blog-post": {
			Chain:    []entity.Pointer{"nl", "en"},
			Fallback: store.FallbackDefaultState,
		},
		"policy": {
			Chain:    []entity.Pointer{"published"},
			Fallback: store.FallbackExclude,
		},
	})

	tests := []struct {
		name       string
		scope      store.WorldScope
		entityType string
		pointer    entity.Pointer
		want       string
		why        string
	}{
		{
			name: "default world resolves everything unscoped",
			// The zero scope applies no per-type resolution, so every entity
			// arrives via its default state — matching worldreader.Rule,
			// which reports the default world identically.
			scope: store.DefaultWorld(), entityType: "blog-post", pointer: "",
			want: ruleUnscoped,
			why:  "the default world applies no resolution at all",
		},
		{
			name:  "pointerless type in a non-default world",
			scope: scope, entityType: "ticket", pointer: "",
			want: ruleUnscoped,
			why: "a type absent from the scope contributes its default state " +
				"in every world (rule 1) — absence is NOT exclusion",
		},
		{
			name:  "first chain coordinate",
			scope: scope, entityType: "blog-post", pointer: "nl",
			want: ruleChain,
			why:  "nl is what site-nl asked for",
		},
		{
			name:  "later chain coordinate is still the chain",
			scope: scope, entityType: "blog-post", pointer: "en",
			want: ruleChain,
			why: "en is DECLARED in site-nl's chain, so serving it is the world " +
				"getting its second choice — not a fallback. Reporting this as " +
				"fallback-default would tell a client the world was overridden " +
				"when it was obeyed",
		},
		{
			name:  "default state under otherwise:default is a fallback",
			scope: scope, entityType: "blog-post", pointer: "",
			want: ruleFallbackDefault,
			why: "no chain coordinate exists, so the default state stood in — " +
				"THE case this field exists for, since the bytes are " +
				"indistinguishable from a real nl face",
		},
		{
			name:  "override chain is honored",
			scope: scope, entityType: "policy", pointer: "published",
			want: ruleChain,
			why:  "policy's per-type override selects published",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolutionRule(tc.scope, tc.entityType, tc.pointer); got != tc.want {
				t.Errorf("resolutionRule(%q, %q) = %q, want %q — %s",
					tc.entityType, tc.pointer, got, tc.want, tc.why)
			}
		})
	}
}

// TestWorldProvenance_NamesTheWorld pins that provenance reports the world the
// request was resolved in, and that an unstamped context reads as the default
// world rather than as an empty name.
func TestWorldProvenance_NamesTheWorld(t *testing.T) {
	e := &entity.Entity{ID: "POST-1", Type: "blog-post", Pointer: "nl"}

	if got := worldProvenance(context.Background(), e); got.Name != defaultWorldName {
		t.Errorf("an unstamped context is the default world; got name %q", got.Name)
	}

	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"blog-post": {Chain: []entity.Pointer{"nl", "en"}, Fallback: store.FallbackDefaultState},
	})
	ctx := withWorld(context.Background(), worldHandle{name: "site-nl", scope: scope})
	got := worldProvenance(ctx, e)
	if got.Name != "site-nl" {
		t.Errorf("name = %q, want site-nl — the world NAME lives only on the "+
			"handle; a store.WorldScope carries none and it cannot be recovered "+
			"later", got.Name)
	}
	if got.Pointer != "nl" || got.Via != ruleChain {
		t.Errorf("got %+v, want pointer=nl via=chain", got)
	}

	if worldProvenance(ctx, nil) != nil {
		t.Error("a nil entity yields nil, so a not-found result passes through " +
			"without the caller branching")
	}
}

// TestGetEntity_ProvenanceDistinguishesFallbackFromSelectedFace is the
// load-bearing wire test for item 2, and the reason the field exists.
//
// Two entities of the SAME type, read through the SAME world, both served
// under a bare id with identical JSON shape. One is the world's actual choice;
// the other is a substitute the world fell back to. Before this change a
// client could not tell them apart at all — which is exactly the
// "published" vs "not published, showing you the draft" distinction Ruling 9
// needs to decide whether to offer an edit affordance.
//
// Mutation-checked per Ruling 10: this test's failure mode is "proves nothing"
// — a provenance block asserted against ONE entity passes trivially if the
// derivation is a constant. Verified to die in BOTH directions: with
// resolutionRule stubbed to return ruleChain unconditionally, and with it
// stubbed to return ruleFallbackDefault unconditionally. One direction would
// not have been enough; a constant is only caught by asserting two entities
// that must disagree, which is why both live in this one test rather than in
// two that could each pass against a different constant.
func TestGetEntity_ProvenanceDistinguishesFallbackFromSelectedFace(t *testing.T) {
	app := newTestAppV1(t)
	ctx := t.Context()

	// TKT-1 has a `published` face; TKT-2 has only its default state.
	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft face"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "only face"},
	})
	pubFace := &entity.Entity{
		ID: "TKT-1", Type: "ticket", Pointer: "published",
		Properties: map[string]any{"title": "published face"},
	}
	if err := app.store.CreateEntity(ctx, pubFace); err != nil {
		t.Fatalf("seed published face: %v", err)
	}

	// A world preferring `published` but falling back to the default state,
	// so BOTH entities resolve — which is what makes the two verdicts
	// comparable on the same wire shape.
	app.SetWorlds(fixedWorlds{scope: store.NewWorldScope(
		map[string]store.TypeResolution{
			"ticket": {
				Chain:    []entity.Pointer{"published"},
				Fallback: store.FallbackDefaultState,
			},
		})})

	var selected, fallback v1.Entity
	getJSON(t, app, "/api/v1/tickets/TKT-1?world=preview", &selected)
	getJSON(t, app, "/api/v1/tickets/TKT-2?world=preview", &fallback)

	// Precondition: without this the test compares two identical fallbacks and
	// proves nothing about the distinction it exists to pin.
	if selected.Properties["title"] != "published face" {
		t.Fatalf("TKT-1 must resolve to its published face; got %q — the world "+
			"is not resolving, so the provenance assertions below are vacuous",
			selected.Properties["title"])
	}
	if fallback.Properties["title"] != "only face" {
		t.Fatalf("TKT-2 must resolve via the fallback; got %q",
			fallback.Properties["title"])
	}

	if selected.World == nil || fallback.World == nil {
		t.Fatal("a per-entity GET carries face provenance")
	}
	if selected.World.Via != ruleChain {
		t.Errorf("TKT-1 was served the face the world SELECTED; via = %q, want %q",
			selected.World.Via, ruleChain)
	}
	if selected.World.Pointer != "published" {
		t.Errorf("TKT-1 pointer = %q, want published", selected.World.Pointer)
	}
	if fallback.World.Via != ruleFallbackDefault {
		t.Errorf("TKT-2 has no published face, so the world SUBSTITUTED its "+
			"default state; via = %q, want %q — reporting %q here would tell a "+
			"client an unpublished entity is published",
			fallback.World.Via, ruleFallbackDefault, selected.World.Via)
	}
	if fallback.World.Pointer != "" {
		t.Errorf("TKT-2 pointer = %q, want the empty default coordinate",
			fallback.World.Pointer)
	}
	// The distinction is the entire point: if these two ever agree, the field
	// conveys nothing and every consumer built on it is silently wrong.
	if selected.World.Via == fallback.World.Via {
		t.Error("both entities reported the same provenance, so the field cannot " +
			"distinguish a published face from a fallback — which is the ONLY " +
			"thing it exists to do")
	}
	if selected.World.Name != "preview" || fallback.World.Name != "preview" {
		t.Errorf("both were read through world `preview`; got %q and %q",
			selected.World.Name, fallback.World.Name)
	}
}

// TestGetEntity_ProvenanceInDefaultWorld pins that the field is present and
// honest on an ordinary, world-free request.
//
// Present rather than omitted, deliberately: a client cannot tell an absent
// `_world` on a default-world response apart from an absent one on a server
// too old to have the field, and would have to probe to find out.
func TestGetEntity_ProvenanceInDefaultWorld(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t"},
	})

	var got v1.Entity
	getJSON(t, app, "/api/v1/tickets/TKT-1", &got)

	if got.World == nil {
		t.Fatal("a default-world GET still carries provenance")
	}
	if got.World.Name != defaultWorldName {
		t.Errorf("name = %q, want %q", got.World.Name, defaultWorldName)
	}
	if got.World.Pointer != "" || got.World.Via != ruleUnscoped {
		t.Errorf("got %+v, want the default coordinate resolved unscoped", got.World)
	}
}

// --- helpers ------------------------------------------------------------

// worldGate is a readGate whose only interesting method is PermitsWorld.
// Everything else answers permissively, matching nopReadGate, so a test using
// it exercises the world decision and nothing else.
type worldGate struct {
	permit map[string]bool
	fail   bool
}

func (g worldGate) PermitsWorld(_ context.Context, world string) (bool, error) {
	if g.fail {
		return false, errWorldUnknown // any infrastructure-shaped failure
	}
	return g.permit[world], nil
}

func (worldGate) PermitsRead(context.Context, string, string) (bool, error) { return true, nil }

func (worldGate) PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error) {
	return nopReadGate{}.PermitsReadMany(ctx, entityType, ids)
}

func (worldGate) ReadQuery(ctx context.Context, entityType string) acl.ReadQueryResult {
	return nopReadGate{}.ReadQuery(ctx, entityType)
}

func (worldGate) SearchScope(ctx context.Context, types []string) map[string]search.TypeScope {
	return nopReadGate{}.SearchScope(ctx, types)
}

func (worldGate) HoldsPermission(context.Context, string) bool { return true }

// fixedWorlds resolves EVERY name to one scope, so a test may name its world
// freely without the name having to match a compiled map.
type fixedWorlds struct{ scope store.WorldScope }

func (f fixedWorlds) Lookup(string) (store.WorldScope, bool) { return f.scope, true }

// getJSON performs a GET through the real router and decodes a 200 body.
// Going through the router (not the handler) is what keeps the world
// middleware, the ACL middleware and the route table in the path.
func getJSON(t *testing.T, app *App, path string, into any) {
	t.Helper()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("GET %s: decode: %v (body %s)", path, err, rec.Body)
	}
}
