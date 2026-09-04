package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Collection rows are content-free unless asked (rowcontent.go).

func rowContentApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "With body"}, Content: "# Body\n\nsecret-ish prose"})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket",
		Properties: map[string]any{"title": "Without body"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"*"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	return app, d
}

func TestListRows_OmitContentUnlessRequested(t *testing.T) {
	app, d := rowContentApp(t)
	for _, tc := range []struct {
		name  string
		query string
		want  bool
	}{
		{"default omits", "", false},
		{"opt in", "include_content=true", true},
		{"opt in numeric", "include_content=1", true},
		{"garbage is false", "include_content=maybe", false},
		{"explicit false", "include_content=false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", tc.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if len(resp.Data) != 2 {
				t.Fatalf("rows = %d, want 2", len(resp.Data))
			}
			for _, row := range resp.Data {
				got := row.Content != ""
				if row.ID == "TKT-001" && got != tc.want {
					t.Errorf("%s content present = %v, want %v (%q)", row.ID, got, tc.want, row.Content)
				}
				if row.ID == "TKT-002" && got {
					t.Errorf("%s has no body but content = %q", row.ID, row.Content)
				}
				if row.Title == "" || row.Properties["title"] == nil {
					t.Errorf("%s lost its title/properties: %+v", row.ID, row)
				}
			}
		})
	}
}

func TestSearchRows_OmitContentUnlessRequested(t *testing.T) {
	app, d := rowContentApp(t)
	for _, tc := range []struct {
		name  string
		query string
		want  bool
	}{
		{"default omits", "q=type%3Aticket", false},
		{"opt in", "q=type%3Aticket&include_content=true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?"+tc.query, http.NoBody)
			req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
			rec := httptest.NewRecorder()
			app.handleV1Search(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var resp v1.ListResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			found := false
			for _, row := range resp.Data {
				if row.ID != "TKT-001" {
					continue
				}
				found = true
				if got := row.Content != ""; got != tc.want {
					t.Errorf("content present = %v, want %v", got, tc.want)
				}
			}
			if !found {
				t.Fatal("TKT-001 missing from search results")
			}
		})
	}
}

// Content-free rows still paginate, sort and count exactly as whole rows
// did: a body was never an input to any of those.
func TestListRows_ContentFreePipelineKeepsOrderAndTotal(t *testing.T) {
	app, d := rowContentApp(t)
	// Titles sort "With body" before "Without body", so page 2 of one row is TKT-002.
	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "sort=title&per_page=1&page=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if resp.Meta.Total != 2 || len(resp.Data) != 1 || resp.Data[0].ID != "TKT-002" {
		t.Errorf("page 2 of sort=title: total=%d rows=%d data=%+v", resp.Meta.Total, len(resp.Data), resp.Data)
	}
}
