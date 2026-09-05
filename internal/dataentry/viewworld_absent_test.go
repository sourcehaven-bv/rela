package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestEntityView_DraftWithNoFaceInThisWorldIsNotAnError is the BUG-1
// reproduction: a draft that has no face in a FILTERING world must not render
// as a terminal error.
//
// Before the fix, `viewEntry` returned the same `errViewEntryNotFound` for two
// unrelated situations — "no such entity" and "this entity has no face in this
// world" — and the handler turned both into a 422 `view_execution_failed`. The
// second is the ordinary state of every unpublished draft, and with
// `app.default_world` set to a filtering world it made every newly created
// entity unreachable from creation until publication (which requires opening
// it).
func TestEntityView_DraftWithNoFaceInThisWorldIsNotAnError(t *testing.T) {
	app := newTestAppV1(t)
	// A draft with NO published face — the exact shape of a just-created
	// entity under `select: published, otherwise: exclude`.
	seedEntity(app, &entity.Entity{
		ID: "TKT-DRAFT", Type: "ticket",
		Properties: map[string]any{"title": "just created"},
	})
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})

	// Precondition: without a world the page renders. If this fails the
	// assertion below proves nothing.
	if code := viewStatus(t, app, "/api/v1/_views/ticket/TKT-DRAFT"); code != http.StatusOK {
		t.Fatalf("precondition: the default world must serve this draft; got %d", code)
	}

	rec := viewRecord(t, app, "/api/v1/_views/ticket/TKT-DRAFT?world=published")
	if rec.Code == http.StatusUnprocessableEntity {
		t.Fatalf("a draft with no face in this world rendered as a terminal 422 — "+
			"this is the ordinary state of every unpublished draft, not an error; body=%s",
			rec.Body)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with an absence marker; got %d %s", rec.Code, rec.Body)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	absent, ok := resp["_world_absent"].(bool)
	if !ok || !absent {
		t.Errorf("the response must MARK the absence so the SPA can offer the "+
			"face that does exist rather than rendering an error; got %v",
			resp["_world_absent"])
	}
}

// TestEntityView_NonexistentEntityStaysAnError is the other half: the fix must
// not turn a genuine miss into a soft absence. "No such entity" and "no face in
// this world" are different answers and must stay different.
//
// The status asserted is 422, which is what this surface has returned for a
// missing entry since before worlds existed (`3a846d8a`). This test pins that
// the BUG-1 fix did not widen that behavior to cover real misses — it is
// deliberately NOT a claim that 422 is the right code for a missing id, which
// is a separate pre-existing question.
func TestEntityView_NonexistentEntityStaysAnError(t *testing.T) {
	app := newTestAppV1(t)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})

	for _, path := range []string{
		"/api/v1/_views/ticket/NOPE-1",
		"/api/v1/_views/ticket/NOPE-1?world=published",
	} {
		rec := viewRecord(t, app, path)
		if rec.Code == http.StatusOK {
			t.Errorf("%s: a nonexistent id must NOT render as a soft absence — "+
				"that would make every typo look like an unpublished draft; body=%s",
				path, rec.Body)
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want the pre-existing 422 for a genuine miss; got %d",
				path, rec.Code)
		}
	}
}

func viewRecord(t *testing.T, app *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

func viewStatus(t *testing.T, app *App, path string) int {
	t.Helper()
	return viewRecord(t, app, path).Code
}

// TestEntityView_AbsenceNeverLeaksToADeniedPrincipal is the existence-oracle
// obligation for the BUG-1 fix.
//
// The new response distinguishes "exists but has no face in this world" from
// "no such entity". That distinction is only safe because it is drawn AFTER
// the ACL row gate, which is world-INDEPENDENT (guard rule 1): a principal who
// may not read the entity is refused with the uniform 404 by `gateRead` and
// never reaches `viewEntry` at all.
//
// This pins that ordering end-to-end through the real router. A principal
// denied `ticket` must receive the SAME 404 for an entity that exists as for
// one that does not — the absence marker must be unreachable for them, or the
// fix would have converted a filtering world into an id-enumeration oracle.
func TestEntityView_AbsenceNeverLeaksToADeniedPrincipal(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-SECRET", Type: "ticket",
		Properties: map[string]any{"title": "exists, but not for bob"},
	})
	// A policy that grants NOTHING on ticket, so every read of it is denied.
	app.acl = mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"nobody": {}},
		Assignments: map[string]string{"bob": "nobody"},
	}, app.store)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "bob", Tool: principal.ToolDataEntry}
	})

	existing := viewRecord(t, app, "/api/v1/_views/ticket/TKT-SECRET?world=published")
	missing := viewRecord(t, app, "/api/v1/_views/ticket/TKT-NOPE?world=published")

	if existing.Code != http.StatusNotFound {
		t.Fatalf("a denied principal must get the uniform 404, never the absence "+
			"marker — otherwise 'exists but not in this world' confirms the id "+
			"exists; got %d %s", existing.Code, existing.Body)
	}
	// `instance` echoes the request URL — the caller's own input, identical by
	// construction for two different ids — so it is excluded from the
	// comparison. Everything that could carry a server-side fact must match.
	if existing.Code != missing.Code ||
		bodyWithoutInstance(t, existing) != bodyWithoutInstance(t, missing) {

		t.Errorf("a denied-existing id must be INDISTINGUISHABLE from a "+
			"nonexistent one:\n existing: %d %s\n missing:  %d %s",
			existing.Code, existing.Body, missing.Code, missing.Body)
	}
	if strings.Contains(existing.Body.String(), "_world_absent") {
		t.Error("the absence marker reached a principal who may not read the entity")
	}
}

// bodyWithoutInstance renders a problem-details body with `instance` removed,
// so two responses to DIFFERENT urls can be compared for everything else.
func bodyWithoutInstance(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode problem body %q: %v", rec.Body, err)
	}
	delete(m, "instance")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	return string(out)
}
