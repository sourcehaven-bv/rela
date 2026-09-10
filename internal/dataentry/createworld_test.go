package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// facedCreateApp is a test app whose `ticket` type declares two faces and
// whose `editorial` world creates into the draft one. The shape mirrors an
// ISMS: the world heads the ADOPTED face for reads, so deriving the create
// face from `select[0]` would publish by the act of creating.
func facedCreateApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	meta := app.State().Meta
	meta.Worlds = map[string]metamodel.WorldDef{
		"editorial": {
			Select:    []string{"published", "draft"},
			Otherwise: metamodel.OtherwiseDefault,
			Create:    "draft",
		},
		"readonly": {
			Select:    []string{"published"},
			Otherwise: metamodel.OtherwiseExclude,
		},
	}
	td := meta.Entities["ticket"]
	td.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	meta.Entities["ticket"] = td
	// A second type declaring NO faces: a world-bound list carries these too,
	// and the world must not break their create.
	meta.Entities["note"] = metamodel.EntityDef{
		Label:      "Note",
		IDPrefix:   "NOTE-",
		Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
	}
	return app
}

// createdSelf reads `_self` off a create response.
func createdSelf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var got struct {
		Self string `json:"_self"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode create response: %v (body %s)", err, rec.Body)
	}
	return got.Self
}

// TestCreateWorld_ResolvesTheFaceServerSide is the point of the key: the
// client names the WORLD it created from and the server maps it to a face.
//
// The client cannot be asked to name the face itself. The create form is
// generic and reachable from several places, so which state a new entity
// starts in belongs to the workflow that opened it, not to the field layout —
// and a faced type has no default row to fall back to (BUG-HC6I2T), so
// omitting the face is a refusal rather than a silent default.
func TestCreateWorld_ResolvesTheFaceServerSide(t *testing.T) {
	app := facedCreateApp(t)

	rec := postCreate(t, app, "tickets",
		`{"world":"editorial","properties":{"title":"Drafted"}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create from a world declaring `create:` must succeed; got %d %s",
			rec.Code, rec.Body)
	}
	if self := createdSelf(t, rec); !strings.HasSuffix(self, "@draft") {
		t.Errorf("the world's `create: draft` must decide the face, NOT `select[0]` "+
			"(which is `published` here — deriving it from the chain would publish "+
			"by the act of creating); _self = %q", self)
	}
}

// TestCreateWorld_FacelessTypeIgnoresTheWorld keeps a world-bound list working
// for the types on it that declare no faces, which is most of them. Such a
// type has exactly one state and no name for it, so the world has nothing to
// contribute and the create is faceless whichever world it came from.
func TestCreateWorld_FacelessTypeIgnoresTheWorld(t *testing.T) {
	app := facedCreateApp(t) // `note` declares no faces

	rec := postCreate(t, app, "notes",
		`{"world":"editorial","properties":{"title":"Faceless"}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("a world must not break a create of a type declaring no faces; "+
			"got %d %s", rec.Code, rec.Body)
	}
	if self := createdSelf(t, rec); strings.Contains(self, "@") {
		t.Errorf("a faceless type stores one unnamed state; _self = %q", self)
	}
}

// TestCreateWorld_Refusals pins the three ways a create can name a world that
// cannot answer. Each names the schema key the operator must fix: a silent
// fallback to a faceless create would hand the request to the manager, which
// refuses with `face_required` and names no world, leaving the operator to
// guess which of the two settings was missing.
func TestCreateWorld_Refusals(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{{
		name: "world declaring no create face",
		body: `{"world":"readonly","properties":{"title":"x"}}`,
		want: "declares no `create:` face",
	}, {
		name: "undeclared world",
		body: `{"world":"nonesuch","properties":{"title":"x"}}`,
		want: `no world named \"nonesuch\" is declared`,
	}, {
		// `default` is implicit and total, so it is never in the Worlds map.
		// It declares no `create:` and cannot: it is the ABSENCE of a world.
		name: "the implicit default world",
		body: `{"world":"default","properties":{"title":"x"}}`,
		want: "the default world names no face",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postCreate(t, facedCreateApp(t), "tickets", tc.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d %s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("the refusal must name what to fix; want %q in %s",
					tc.want, rec.Body)
			}
		})
	}
}

// TestCreateWorld_FaceAndWorldAreExclusive refuses a request naming both
// rather than picking one. They can disagree, and silently honoring either
// would write a row the caller did not ask for.
func TestCreateWorld_FaceAndWorldAreExclusive(t *testing.T) {
	rec := postCreate(t, facedCreateApp(t), "tickets",
		`{"world":"editorial","face":"published","properties":{"title":"x"}}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for a request naming both, got %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "not both") {
		t.Errorf("the refusal must say the two are exclusive; got %s", rec.Body)
	}
}

// TestCreateWorld_ExplicitFaceStillWins is the compatibility floor: `face:`
// was the only spelling before this key and remains authoritative, so an API
// client naming a face directly is unaffected by any world configuration.
func TestCreateWorld_ExplicitFaceStillWins(t *testing.T) {
	rec := postCreate(t, facedCreateApp(t), "tickets",
		`{"face":"published","properties":{"title":"Direct"}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("an explicit face must still create; got %d %s", rec.Code, rec.Body)
	}
	if self := createdSelf(t, rec); !strings.HasSuffix(self, "@published") {
		t.Errorf("the named face must be honored; _self = %q", self)
	}
}

// TestCreateWorld_DryRunAgreesWithTheCreate pins the two halves of the create
// form to each other. The form dry-runs per keystroke to gate fields, and a
// verdict computed against a DIFFERENT face than the submit will write is
// worse than no verdict: it gates on the wrong row while looking authoritative.
func TestCreateWorld_DryRunAgreesWithTheCreate(t *testing.T) {
	app := facedCreateApp(t)
	body := `{"world":"editorial","properties":{"title":"Staged"}}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets?dry_run=true",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	dry := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(dry, req)
	if dry.Code != http.StatusOK {
		t.Fatalf("dry-run must answer; got %d %s", dry.Code, dry.Body)
	}

	live := postCreate(t, app, "tickets", body)
	if live.Code != http.StatusCreated {
		t.Fatalf("create must succeed; got %d %s", live.Code, live.Body)
	}

	var dryGot struct {
		Face string `json:"face"`
	}
	if err := json.Unmarshal(dry.Body.Bytes(), &dryGot); err != nil {
		t.Fatalf("decode dry-run: %v (body %s)", err, dry.Body)
	}
	self := createdSelf(t, live)
	if dryGot.Face != "" && !strings.HasSuffix(self, "@"+dryGot.Face) {
		t.Errorf("the dry-run judged face %q but the create wrote %q — the form "+
			"would gate fields against a row it is not writing",
			dryGot.Face, self)
	}
	if !strings.HasSuffix(self, "@draft") {
		t.Errorf("create wrote %q, want the world's `create:` face", self)
	}
}
