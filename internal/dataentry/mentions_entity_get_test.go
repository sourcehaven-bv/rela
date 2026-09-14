package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// decodeEntityResponse reads a v1.Entity out of a recorder, failing the test
// on a non-200 or unparseable body.
func decodeEntityResponse(t *testing.T, rec *httptest.ResponseRecorder) v1.Entity {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET entity: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode entity response: %v; body=%s", err, rec.Body)
	}
	return got
}

// TestEntityGetMentions_ResolvesReadableRef: the edit form needs the same
// titled-link data the read surface has, so a readable code-span reference
// resolves to its title on the single-entity GET.
func TestEntityGetMentions_ResolvesReadableRef(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "readable feature"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "a ticket"},
		Content:    "implements `FEAT-001`"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	got := decodeEntityResponse(t, getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", ""))
	m, ok := got.Mentions["FEAT-001"]
	if !ok {
		t.Fatalf("expected a mention for FEAT-001, got %+v", got.Mentions)
	}
	if m.Title != "readable feature" {
		t.Errorf("mention title = %q, want %q", m.Title, "readable feature")
	}
	if m.Type != "feature" {
		t.Errorf("mention type = %q, want %q", m.Type, "feature")
	}
}

// TestEntityGetMentions_DropsUnreadable is the row-level half of the read
// gate on this new surface: an entity the principal may not read produces NO
// mention, so the editor renders the bare code span. Anything else would make
// the edit form an existence oracle for entities the caller cannot see.
func TestEntityGetMentions_DropsUnreadable(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "SECRET-FEATURE-TITLE"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "a ticket"},
		Content:    "implements `FEAT-001`"})

	// alice may read tickets but NOT features.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "")
	got := decodeEntityResponse(t, rec)
	if _, ok := got.Mentions["FEAT-001"]; ok {
		t.Errorf("LEAK: unreadable entity resolved as a mention: %+v", got.Mentions)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-FEATURE-TITLE") {
		t.Errorf("LEAK: unreadable title in the entity response: %s", body)
	}
}

// TestEntityGetMentions_RedactsHiddenPrimaryTitle is the field-level half: a
// readable entity whose display-title property is redacted must fall back to
// its id and be flagged, never expose the hidden value.
func TestEntityGetMentions_RedactsHiddenPrimaryTitle(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}}
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "SECRET-FEATURE-TITLE"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Content: "implements `FEAT-001`"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "")
	if body := rec.Body.String(); strings.Contains(body, "SECRET-FEATURE-TITLE") {
		t.Errorf("LEAK: redacted display title in the entity response: %s", body)
	}
}

// TestEntityGetMentions_MatchesViewSurface pins the property that made this
// worth adding: the editor and the rendered view must agree. The same content
// scanned through both handlers must produce the same mention map, or a title
// appears in one surface and not the other.
func TestEntityGetMentions_MatchesViewSurface(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "readable feature"}})
	const content = "implements `FEAT-001` and `FEAT-MISSING`"
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Content: content})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	fromGet := decodeEntityResponse(t,
		getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "")).Mentions
	direct := collectMentions(gateCtxFor(aliceCtx(), t, d), app.store, app.viewReader,
		app.State().Meta, content)

	if len(fromGet) != len(direct) {
		t.Fatalf("entity GET and direct scan disagree: %+v vs %+v", fromGet, direct)
	}
	for id, want := range direct {
		if got := fromGet[id]; got != want {
			t.Errorf("mention %q: GET has %+v, direct scan has %+v", id, got, want)
		}
	}
	if _, ok := fromGet["FEAT-MISSING"]; ok {
		t.Errorf("a reference to a nonexistent entity must not resolve: %+v", fromGet)
	}
}

// TestEntityGetMentions_AbsentWithoutRefs: a body with no entity references
// emits no `mentions` key at all, so the common case costs nothing on the
// wire.
func TestEntityGetMentions_AbsentWithoutRefs(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Content: "plain body with `go test ./...` and no refs"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "")
	if got := decodeEntityResponse(t, rec); len(got.Mentions) != 0 {
		t.Errorf("expected no mentions, got %+v", got.Mentions)
	}
	if strings.Contains(rec.Body.String(), `"mentions"`) {
		t.Errorf("empty mentions must be omitted from the wire: %s", rec.Body)
	}
}
