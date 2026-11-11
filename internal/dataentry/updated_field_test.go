package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// updatedACL: alice sees only the title of a ticket and the reason of a
// depends_on edge; bob sees everything. Both may update tickets.
const updatedACL = `
roles:
  partial:
    read: [ticket]
    update: [ticket]
    visible:
      ticket:
        - field: title
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
  full:
    read: [ticket]
    update: [ticket]
assignments:
  alice: partial
  bob: full
`

func updatedApp(t *testing.T) *App {
	t.Helper()
	app := buildPolicyApp(t, updatedACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "a", "status": "open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	if _, err := app.store.CreateRelation(context.Background(),
		entity.RelationKey{From: "TKT-001", Type: "depends_on", To: "TKT-002"},
		&store.RelationData{Properties: map[string]any{"reason": "r", "secret": "s"}}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	return app
}

func boundAs(t *testing.T, app *App, req *http.Request, user string) *http.Request {
	t.Helper()
	return req.WithContext(acltest.Bind(principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}), t, app.acl))
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	return body
}

// `_updated` rides a per-entity read only when the reader sees every field:
// next to a hidden field it would say when that field may have changed.
func TestUpdated_EntityOnlyWhenNothingHidden(t *testing.T) {
	app := updatedApp(t)
	for _, tc := range []struct {
		user string
		want bool
	}{{"bob", true}, {"alice", false}} {
		t.Run(tc.user, func(t *testing.T) {
			req := boundAs(t, app, httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody), tc.user)
			rec := httptest.NewRecorder()
			app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET = %d %s", rec.Code, rec.Body)
			}
			body := decodeBody(t, rec)
			if props, _ := body["properties"].(map[string]any); (props["status"] != nil) != tc.want {
				t.Fatalf("fixture: status visible = %v, want %v", props["status"] != nil, tc.want)
			}
			got, ok := body["_updated"].(string)
			if ok != tc.want {
				t.Fatalf("_updated present = %v, want %v (%s)", ok, tc.want, rec.Body)
			}
			if ok && !strings.HasSuffix(got, "Z") {
				t.Errorf("_updated = %q, want RFC 3339 in UTC", got)
			}
		})
	}
}

func TestUpdated_RelationOnlyWhenNothingHidden(t *testing.T) {
	app := updatedApp(t)
	for _, tc := range []struct {
		user string
		want bool
	}{{"bob", true}, {"alice", false}} {
		t.Run(tc.user, func(t *testing.T) {
			req := boundAs(t, app, httptest.NewRequest(http.MethodGet,
				"/api/v1/tickets/TKT-001/relations/depends_on/TKT-002", http.NoBody), tc.user)
			rec := httptest.NewRecorder()
			handleV1GetRelationTarget(app, rec, req, "ticket", "TKT-001", "depends_on", "TKT-002")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET = %d %s", rec.Code, rec.Body)
			}
			if _, ok := decodeBody(t, rec)["_updated"]; ok != tc.want {
				t.Fatalf("_updated present = %v, want %v (%s)", ok, tc.want, rec.Body)
			}
		})
	}
}

// patchRelation PATCHes TKT-001's depends_on edge as user.
func patchRelation(t *testing.T, app *App, user, body, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/tickets/TKT-001/relations/depends_on/TKT-002", strings.NewReader(body))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateRelation(rec, boundAs(t, app, req, user), "ticket", "TKT-001", "depends_on", "TKT-002")
	return rec
}

func getRelationETag(t *testing.T, app *App, user string) string {
	t.Helper()
	req := boundAs(t, app, httptest.NewRequest(http.MethodGet,
		"/api/v1/tickets/TKT-001/relations/depends_on/TKT-002", http.NoBody), user)
	rec := httptest.NewRecorder()
	handleV1GetRelationTarget(app, rec, req, "ticket", "TKT-001", "depends_on", "TKT-002")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	return rec.Header().Get("ETag")
}

func TestRelationPatch_IfMatch(t *testing.T) {
	for _, user := range []string{"bob", "alice"} {
		t.Run(user, func(t *testing.T) {
			app := updatedApp(t)
			etag := getRelationETag(t, app, user)

			rec := patchRelation(t, app, user, `{"meta":{"reason":"x"}}`, etag)
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH with the GET's ETag = %d %s", rec.Code, rec.Body)
			}
			next := rec.Header().Get("ETag")
			if next == "" || next == etag {
				t.Fatalf("PATCH ETag = %q, want a new one (was %q)", next, etag)
			}
			if got := getRelationETag(t, app, user); got != next {
				t.Errorf("PATCH ETag %s != GET ETag %s", next, got)
			}
			if user == "alice" && strings.Contains(rec.Body.String(), `"secret"`) {
				t.Errorf("PATCH response serves hidden meta: %s", rec.Body)
			}

			if rec = patchRelation(t, app, user, `{"meta":{"reason":"y"}}`, etag); rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("PATCH with a stale ETag = %d %s, want 412", rec.Code, rec.Body)
			}
			if rec = patchRelation(t, app, user, `{"meta":{"reason":"z"}}`, ""); rec.Code != http.StatusOK {
				t.Fatalf("PATCH without If-Match = %d %s", rec.Code, rec.Body)
			}
		})
	}
}

// A hidden meta value changing does not move alice's ETag, so her If-Match
// still holds: she cannot learn of the change through a 412.
func TestRelationPatch_IfMatchIgnoresHiddenMeta(t *testing.T) {
	app := updatedApp(t)
	etag := getRelationETag(t, app, "alice")
	if rec := patchRelation(t, app, "bob", `{"meta":{"secret":"t"}}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("bob PATCH = %d %s", rec.Code, rec.Body)
	}
	if rec := patchRelation(t, app, "alice", `{"meta":{"reason":"x"}}`, etag); rec.Code != http.StatusOK {
		t.Fatalf("alice PATCH after a hidden change = %d %s, want 200", rec.Code, rec.Body)
	}
}
