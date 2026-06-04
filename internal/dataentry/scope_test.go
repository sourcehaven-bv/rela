package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// positionURL builds the _position request URL for a scope descriptor.
func positionURL(t *testing.T, id string, scope ScopeDescriptor) string {
	t.Helper()
	raw, err := json.Marshal(scope)
	if err != nil {
		t.Fatalf("marshal scope: %v", err)
	}
	q := url.Values{}
	q.Set("id", id)
	q.Set("scope", string(raw))
	return "/api/v1/_position?" + q.Encode()
}

func getPosition(t *testing.T, app *App, id string, scope ScopeDescriptor) (*httptest.ResponseRecorder, V1Position) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, positionURL(t, id, scope), http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)
	var pos V1Position
	if rec.Code == http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(&pos); err != nil {
			t.Fatalf("decode position: %v", err)
		}
	}
	return rec, pos
}

func seedTickets(app *App) {
	// Seeded out of title order to prove sort, not insertion order, wins.
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "B", "status": "open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A", "status": "open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-003", Type: "ticket", Properties: map[string]interface{}{"title": "C", "status": "closed"}})
}

func TestV1Position(t *testing.T) {
	listScope := func() ScopeDescriptor {
		return ScopeDescriptor{Source: "list", Type: "ticket", Sort: "title"}
	}

	t.Run("middle entity has prev and next", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)

		rec, pos := getPosition(t, app, "TKT-002", listScope())
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d", rec.Code)
		}
		// Sorted by title: [TKT-001(A), TKT-002(B), TKT-003(C)].
		if pos.Current != 2 || pos.Total != 3 {
			t.Errorf("current/total: got %d/%d, want 2/3", pos.Current, pos.Total)
		}
		if pos.Prev == nil || *pos.Prev != "TKT-001" {
			t.Errorf("prev: got %v, want TKT-001", pos.Prev)
		}
		if pos.Next == nil || *pos.Next != "TKT-003" {
			t.Errorf("next: got %v, want TKT-003", pos.Next)
		}
	})

	t.Run("first entity has no prev", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)

		_, pos := getPosition(t, app, "TKT-001", listScope())
		if pos.Current != 1 {
			t.Errorf("current: got %d, want 1", pos.Current)
		}
		if pos.Prev != nil {
			t.Errorf("prev: got %v, want nil", *pos.Prev)
		}
		if pos.Next == nil || *pos.Next != "TKT-002" {
			t.Errorf("next: got %v, want TKT-002", pos.Next)
		}
	})

	t.Run("last entity has no next", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)

		_, pos := getPosition(t, app, "TKT-003", listScope())
		if pos.Current != 3 {
			t.Errorf("current: got %d, want 3", pos.Current)
		}
		if pos.Next != nil {
			t.Errorf("next: got %v, want nil", *pos.Next)
		}
		if pos.Prev == nil || *pos.Prev != "TKT-002" {
			t.Errorf("prev: got %v, want TKT-002", pos.Prev)
		}
	})

	t.Run("scope respects property filter", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)

		// Filter to status=open → [TKT-001, TKT-002]; TKT-003 drops out.
		scope := ScopeDescriptor{
			Source:  "list",
			Type:    "ticket",
			Sort:    "title",
			Filters: map[string]string{"filter[status]": "open"},
		}
		_, pos := getPosition(t, app, "TKT-002", scope)
		if pos.Total != 2 {
			t.Errorf("total: got %d, want 2 (filtered)", pos.Total)
		}
		if pos.Current != 2 {
			t.Errorf("current: got %d, want 2", pos.Current)
		}
		if pos.Next != nil {
			t.Errorf("next: got %v, want nil (TKT-003 filtered out)", *pos.Next)
		}
	})

	t.Run("search scope honors q", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)
		// Searcher matches only TKT-001 and TKT-003; sort=title orders them.
		app.searcher = &fakeSearcher{hits: []search.Hit{
			{ID: "TKT-001", Type: "ticket"},
			{ID: "TKT-003", Type: "ticket"},
		}}

		scope := ScopeDescriptor{Source: "search", Type: "ticket", Sort: "title", Q: "ticket"}
		_, pos := getPosition(t, app, "TKT-001", scope)
		if pos.Total != 2 {
			t.Errorf("total: got %d, want 2 (search-narrowed)", pos.Total)
		}
		if pos.Current != 1 {
			t.Errorf("current: got %d, want 1", pos.Current)
		}
		if pos.Next == nil || *pos.Next != "TKT-003" {
			t.Errorf("next: got %v, want TKT-003", pos.Next)
		}
	})

	t.Run("id not in scope returns 404", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)

		rec, _ := getPosition(t, app, "TKT-999", listScope())
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status: got %d, want 404", rec.Code)
		}
	})

	t.Run("entity filtered out of scope returns 404", func(t *testing.T) {
		app := newTestAppV1(t)
		seedTickets(app)

		// TKT-003 is status=closed; filtering to open removes it from the set.
		scope := ScopeDescriptor{
			Source:  "list",
			Type:    "ticket",
			Filters: map[string]string{"filter[status]": "open"},
		}
		rec, _ := getPosition(t, app, "TKT-003", scope)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status: got %d, want 404", rec.Code)
		}
	})
}

func TestV1PositionBadRequest(t *testing.T) {
	app := newTestAppV1(t)
	seedTickets(app)

	cases := []struct {
		name string
		id   string
		// rawScope is the literal scope param; when scope is non-nil it is
		// JSON-marshaled instead.
		rawScope string
		scope    *ScopeDescriptor
		want     int
	}{
		{name: "missing id", id: "", scope: &ScopeDescriptor{Source: "list", Type: "ticket"}, want: http.StatusBadRequest},
		{name: "missing scope", id: "TKT-001", rawScope: "", want: http.StatusBadRequest},
		{name: "malformed json", id: "TKT-001", rawScope: "{not json", want: http.StatusBadRequest},
		{name: "unknown source", id: "TKT-001", scope: &ScopeDescriptor{Source: "bogus", Type: "ticket"}, want: http.StatusBadRequest},
		{name: "unknown type", id: "TKT-001", scope: &ScopeDescriptor{Source: "list", Type: "nope"}, want: http.StatusBadRequest},
		{name: "bad filter key", id: "TKT-001", scope: &ScopeDescriptor{Source: "list", Type: "ticket", Filters: map[string]string{"status": "open"}}, want: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := url.Values{}
			if tc.id != "" {
				q.Set("id", tc.id)
			}
			switch {
			case tc.scope != nil:
				raw, err := json.Marshal(tc.scope)
				if err != nil {
					t.Fatal(err)
				}
				q.Set("scope", string(raw))
			case tc.rawScope != "":
				q.Set("scope", tc.rawScope)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_position?"+q.Encode(), http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1EntityPosition(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status: got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestV1PositionMatchesListOrdering pins the core invariant: _position observes
// the same ordered set as the list endpoint for the same scope. If the two
// pipelines ever diverge, this fails.
func TestV1PositionMatchesListOrdering(t *testing.T) {
	app := newTestAppV1(t)
	seedTickets(app)

	// The ordered set the list endpoint produces for sort=title.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=title", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	var list V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}

	scope := ScopeDescriptor{Source: "list", Type: "ticket", Sort: "title"}
	for i, e := range list.Data {
		_, pos := getPosition(t, app, e.ID, scope)
		if pos.Current != i+1 {
			t.Errorf("%s: position current %d, list index %d", e.ID, pos.Current, i+1)
		}
		if pos.Total != len(list.Data) {
			t.Errorf("%s: position total %d, list len %d", e.ID, pos.Total, len(list.Data))
		}
	}
}
