package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestACLListRegression_NopACL_ListUnchanged pins TKT-VMD8 AC11 for
// the list endpoint: with no ACL configured (nopReadGate via the ctx
// fallback → AllowAll → the pre-ACL listFromStoreByTypes path), the
// response carries exactly today's shape — full data set, full
// totals, full pagination headers, Link header, _actions present.
func TestACLListRegression_NopACL_ListUnchanged(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "T2"}})
	seedEntity(app, &entity.Entity{ID: "TKT-003", Type: "ticket", Properties: map[string]any{"title": "T3"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?per_page=2&page=1", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")

	if rec.Code != http.StatusOK {
		t.Fatalf("NopACL list: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var resp V1ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body)
	}
	if len(resp.Data) != 2 {
		t.Errorf("data.length = %d, want 2", len(resp.Data))
	}
	if resp.Meta.Total != 3 || !resp.Meta.HasMore || resp.Meta.Page != 1 || resp.Meta.PerPage != 2 {
		t.Errorf("meta = %+v, want {Total:3 Page:1 PerPage:2 HasMore:true}", resp.Meta)
	}
	if got := rec.Header().Get("X-Total-Count"); got != "3" {
		t.Errorf("X-Total-Count = %q, want 3", got)
	}
	link := rec.Header().Get("Link")
	if !strings.Contains(link, `rel="next"`) || !strings.Contains(link, `rel="last"`) {
		t.Errorf("Link header lost pagination rels: %s", link)
	}
	if resp.Actions == nil {
		t.Error("_actions absent from NopACL list response")
	} else if create, ok := resp.Actions["create"]; !ok || !create {
		t.Errorf("_actions.create = %v (present=%v), want true under NopACL", create, ok)
	}
}

// TestACLListRegression_NopACL_FreeTextStillWorks pins that the ?q=
// pipeline (search intersection) is untouched on the AllowAll path —
// the gate resolves before search but must not perturb it.
func TestACLListRegression_NopACL_FreeTextStillWorks(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha match"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "beta other"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=alpha", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")

	if rec.Code != http.StatusOK {
		t.Fatalf("NopACL list q=: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var resp V1ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Errorf("free-text filter broken under NopACL: got %+v, want [TKT-001]", resp.Data)
	}
}
