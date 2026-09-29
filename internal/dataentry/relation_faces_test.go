package dataentry

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Relation routes read their endpoints through the resolver (design 8.2 of
// TKT-2528AB): the tail is the row the path address names, so its face gate
// applies; the head is named by bare id, so some face of it must be readable.

// asAlice gates req as alice under d.
func asAlice(t *testing.T, d *acl.Declarative, req *http.Request) *http.Request {
	t.Helper()
	alice := principal.With(req.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	return req.WithContext(gateCtxFor(alice, t, d))
}

// relationAs sends one relation-route request as alice, through the same
// dispatch the router uses.
func relationAs(
	t *testing.T, app *App, d *acl.Declarative,
	method, typeName, plural, addr, relType, target, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	path := "/api/v1/" + plural + "/" + addr + "/relations/" + relType
	if target != "" {
		path += "/" + target
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = asAlice(t, d, req)
	rec := httptest.NewRecorder()
	if target == "" {
		app.write.handleV1CreateRelation(rec, req, typeName, addr, relType)
	} else {
		app.handleV1RelationTarget(rec, req, typeName, addr, relType, target)
	}
	return rec
}

func draftOnly(id string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "policy", Face: "draft", Properties: map[string]any{"title": "DRAFT"}}
}

func cloneAs(t *testing.T, app *App, d *acl.Declarative, addr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/policys/"+addr+"/_actions/clone", http.NoBody)
	req = asAlice(t, d, req)
	rec := httptest.NewRecorder()
	app.write.handleV1CloneEntity(rec, req, "policy", addr)
	return rec
}

// publishedEditor may read and write POL-1's published face only.
func publishedEditor(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"published-editor": {
				Read:   []string{"policy@published", "feature"},
				Create: []string{"policy@published", "feature"},
				Update: []string{"policy@published", "feature"},
				Delete: []string{"policy@published", "feature"},
			}},
			Assignments: map[string]string{"alice": "published-editor"},
		}, st)
	})
	return app, d
}

// Every relation write and the clone read their path entity through the
// resolver, so a face the caller may not read is the same 404 as a face that
// does not exist, and the write never runs.
func TestRelationWrites_DeniedFaceIsTheUniformMiss(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	// An edge on the denied face, so update and delete have something to hit.
	if _, err := app.store.CreateRelation(ctx, entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "FEAT-1"}, &store.RelationData{}); err != nil {
		t.Fatalf("seed draft-tailed edge: %v", err)
	}

	if rec := relationAs(t, app, d, http.MethodPost, "policy", "policys", "POL-1@published",
		"cites", "", `{"id":"FEAT-1"}`); rec.Code != http.StatusCreated {
		t.Fatalf("control: a relation on the granted face = %d %s, want 201", rec.Code, rec.Body)
	}

	for _, tc := range []struct {
		name string
		do   func(addr string) *httptest.ResponseRecorder
	}{
		{"create", func(addr string) *httptest.ResponseRecorder {
			return relationAs(t, app, d, http.MethodPost, "policy", "policys", addr, "cites", "", `{"id":"FEAT-1"}`)
		}},
		{"update", func(addr string) *httptest.ResponseRecorder {
			return relationAs(t, app, d, http.MethodPatch, "policy", "policys", addr, "cites", "FEAT-1",
				`{"meta":{}}`)
		}},
		{"delete", func(addr string) *httptest.ResponseRecorder {
			return relationAs(t, app, d, http.MethodDelete, "policy", "policys", addr, "cites", "FEAT-1", "")
		}},
		{"get", func(addr string) *httptest.ResponseRecorder {
			return relationAs(t, app, d, http.MethodGet, "policy", "policys", addr, "cites", "FEAT-1", "")
		}},
		{"clone", func(addr string) *httptest.ResponseRecorder { return cloneAs(t, app, d, addr) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			denied, missing := tc.do("POL-1@draft"), tc.do("POL-1@nope")
			if denied.Code != http.StatusNotFound {
				t.Fatalf("denied face = %d %s, want 404", denied.Code, denied.Body)
			}
			if problemShape(t, denied.Body.Bytes()) != problemShape(t, missing.Body.Bytes()) {
				t.Errorf("denied face body %q differs from missing face body %q", denied.Body, missing.Body)
			}
		})
	}
	assertEdgeTail(ctx, t, app, "POL-1", "draft", "cites", "FEAT-1")
}

// An edge tailed on one face is invisible from a reader of another face only
// (8.2): the tail's face gate decides, not the bare id's readability.
func TestRelationGet_EdgeOnAnotherFaceIsNotFound(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	if _, err := app.store.CreateRelation(ctx, entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "FEAT-1"}, &store.RelationData{Content: "DRAFT EDGE BODY"}); err != nil {
		t.Fatalf("seed draft-tailed edge: %v", err)
	}
	for _, addr := range []string{"POL-1@draft", "POL-1@published"} {
		rec := relationAs(t, app, d, http.MethodGet, "policy", "policys", addr, "cites", "FEAT-1", "")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "DRAFT EDGE BODY") {
			t.Errorf("%s: a draft-tailed edge served to a policy@published reader: %d %s", addr, rec.Code, rec.Body)
		}
	}
}

// PATCH and DELETE gate the peer before touching the edge: an edge to a head
// with no readable face answers exactly as an edge to an absent id, and is
// left in place.
func TestRelationWrites_HiddenPeerIsTheUniformMiss(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	if err := app.store.CreateEntity(ctx, draftOnly("POL-2")); err != nil {
		t.Fatalf("seed POL-2: %v", err)
	}
	if _, err := app.store.CreateRelation(ctx, entity.RelationKey{From: "FEAT-1", Type: "governs", To: "POL-2"}, nil); err != nil {
		t.Fatalf("seed edge to the hidden peer: %v", err)
	}
	for _, tc := range []struct{ method, body string }{
		{http.MethodPatch, `{"meta":{}}`},
		{http.MethodDelete, ""},
	} {
		t.Run(tc.method, func(t *testing.T) {
			hidden := relationAs(t, app, d, tc.method, "feature", "features", "FEAT-1", "governs", "POL-2", tc.body)
			absent := relationAs(t, app, d, tc.method, "feature", "features", "FEAT-1", "governs", "POL-9", tc.body)
			if hidden.Code != http.StatusNotFound ||
				problemShape(t, hidden.Body.Bytes()) != problemShape(t, absent.Body.Bytes()) {

				t.Errorf("hidden peer must match an absent one:\n hidden: %d %s\n absent: %d %s",
					hidden.Code, hidden.Body, absent.Code, absent.Body)
			}
		})
	}
	if _, err := app.store.GetRelation(ctx, entity.RelationKey{From: "FEAT-1", Type: "governs", To: "POL-2"}); err != nil {
		t.Errorf("the refused delete must leave the edge: %v", err)
	}
}

// A faced head has no zero-face row, so a create or GET naming it by bare id
// must check its family, not read a row at the zero face.
func TestRelation_FacedHeadByBareID(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	// POL-2 exists only as a draft, which alice may not read.
	if err := app.store.CreateEntity(ctx, draftOnly("POL-2")); err != nil {
		t.Fatalf("seed POL-2: %v", err)
	}

	rec := relationAs(t, app, d, http.MethodPost, "feature", "features", "FEAT-1", "governs", "",
		`{"id":"POL-1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create to a faced head with a readable face = %d %s, want 201", rec.Code, rec.Body)
	}
	if rec := relationAs(t, app, d, http.MethodGet, "feature", "features", "FEAT-1", "governs", "POL-1",
		""); rec.Code != http.StatusOK {
		t.Fatalf("GET of an edge to a faced head = %d %s, want 200", rec.Code, rec.Body)
	}

	hidden := relationAs(t, app, d, http.MethodPost, "feature", "features", "FEAT-1", "governs", "",
		`{"id":"POL-2"}`)
	absent := relationAs(t, app, d, http.MethodPost, "feature", "features", "FEAT-1", "governs", "",
		`{"id":"POL-9"}`)
	if hidden.Code != http.StatusNotFound || hidden.Body.String() != absent.Body.String() {
		t.Errorf("a head with no readable face must match an absent one:\n hidden: %d %s\n absent: %d %s",
			hidden.Code, hidden.Body, absent.Code, absent.Body)
	}
}

// A read-gate fault while checking a relation peer fails the write with the
// gate error, instead of reporting a live peer as target_not_found.
func TestRelationPeerGateFault_FailsTheWrite(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})
	ctx := withReadGate(context.Background(), fakeGate{permitsErr: errors.New(`pq: relation "secret" missing`)})
	desired := map[string]v1.RelationsUpdate{"implements": {
		DataPresent: true,
		Data:        []v1.ResourceIdentifier{{Type: "feature", ID: "FEAT-001"}},
	}}
	_, err := app.write.validateRelationsModern(ctx, "TKT-001", "ticket", desired)
	var gerr *gateFaultError
	if !errors.As(err, &gerr) {
		t.Fatalf("validateRelationsModern err = %v, want a gateFaultError", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.write.writeRelationsValidationError(rec, req, err)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "acl_query_failed") ||
		strings.Contains(rec.Body.String(), "secret") {

		t.Errorf("gate fault = %d %s, want an opaque 500 acl_query_failed", rec.Code, rec.Body)
	}
}

// listFailingStore fails every entity listing, so the peer batch's header
// read fails. It hides any header projection of the wrapped store.
type listFailingStore struct{ store.Store }

func (listFailingStore) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) { yield(nil, errors.New(`pq: relation "secret" missing`)) }
}

// A failed peer header read fails the write like a gate fault, instead of
// reporting a live peer as target_not_found.
func TestRelationPeerReadFault_FailsTheWrite(t *testing.T) {
	app := newTestAppV1(t)
	vr, err := newVisibleReader(listFailingStore{app.store})
	if err != nil {
		t.Fatal(err)
	}
	app.write.visible = vr
	desired := map[string]v1.RelationsUpdate{"implements": {
		DataPresent: true,
		Data:        []v1.ResourceIdentifier{{Type: "feature", ID: "FEAT-001"}},
	}}
	_, err = app.write.validateRelationsModern(context.Background(), "TKT-001", "ticket", desired)
	var gerr *gateFaultError
	if !errors.As(err, &gerr) {
		t.Fatalf("validateRelationsModern err = %v, want a gateFaultError", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.write.writeRelationsValidationError(rec, req, err)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") ||
		strings.Contains(rec.Body.String(), "FEAT-001") {

		t.Errorf("read fault = %d %s, want an opaque 500", rec.Code, rec.Body)
	}
}

// An identity-scoped edge attaches to the entity, so it is found from any
// readable face of a faced source, bare or addressed.
func TestRelationGet_IdentityEdgeOnFacedSource(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	if _, err := app.store.CreateRelation(ctx, entity.RelationKey{From: "POL-1", Type: "implements", To: "FEAT-1"}, nil); err != nil {
		t.Fatalf("seed identity edge: %v", err)
	}
	for _, addr := range []string{"POL-1", "POL-1@published"} {
		rec := relationAs(t, app, d, http.MethodGet, "policy", "policys", addr, "implements", "FEAT-1", "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: identity edge = %d %s, want 200", addr, rec.Code, rec.Body)
		}
	}
	if rec := relationAs(t, app, d, http.MethodGet, "policy", "policys", "POL-1@draft", "implements", "FEAT-1",
		""); rec.Code != http.StatusNotFound {
		t.Errorf("identity edge through a denied face = %d, want 404", rec.Code)
	}
}
