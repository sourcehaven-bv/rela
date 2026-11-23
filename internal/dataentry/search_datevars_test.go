package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
)

// TestV1Search_DateVariables pins that `$today` and `$now` resolve in a
// search query (`/_search`, dashboard `query:` cards and nav scopes share
// this parse), at the request's read time (TKT-08ZI5XN). Before, the
// variable failed to parse as a date and every row was dropped.
func TestV1Search_DateVariables(t *testing.T) {
	app := newTestAppV1(t)
	ticket := app.Meta().Entities["ticket"]
	ticket.Properties["due"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeDate}
	ticket.Properties["valid_until"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeDatetime}

	for id, props := range map[string]map[string]any{
		"TKT-001": {"due": "2026-11-15", "valid_until": "2026-11-16T09:00:00Z"},
		"TKT-002": {"due": "2026-11-16", "valid_until": "2026-11-16T11:00:00Z"},
		"TKT-003": {"due": time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)},
		"TKT-004": {},
		// A YAML-decoded date equal to the read day: the store's text form
		// is not "2026-11-16", so equality must not be pushed down.
		"TKT-005": {"due": time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)},
	} {
		props["title"] = "Ticket " + id
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: props})
	}
	readAt := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		q    string
		want []string
	}{
		{"type:ticket prop:due<$today", []string{"TKT-001"}},
		{"type:ticket prop:due=$today", []string{"TKT-002", "TKT-005"}},
		{"type:ticket prop:due>=$today", []string{"TKT-002", "TKT-003", "TKT-005"}},
		{"type:ticket prop:due<=$today+7d", []string{"TKT-001", "TKT-002", "TKT-003", "TKT-005"}},
		{"type:ticket prop:due<$today-1w", nil},
		{"type:ticket prop:due=$yesterday", []string{"TKT-001"}},
		{"type:ticket prop:valid_until<$now", []string{"TKT-001"}},
		{"type:ticket prop:valid_until>=$today", []string{"TKT-001", "TKT-002"}},
	}
	for _, tc := range tests {
		t.Run(tc.q, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q="+url.QueryEscape(tc.q), http.NoBody)
			req = req.WithContext(readtime.With(req.Context(), readAt))
			rec := httptest.NewRecorder()
			app.entities.handleV1Search(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var resp v1.ListResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, e := range resp.Data {
				got = append(got, e.ID)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAttachReadTime pins one read time per request: every reader in the
// handler sees the same value, taken from nowFunc once at the edge.
func TestAttachReadTime(t *testing.T) {
	calls := 0
	prev := nowFunc
	nowFunc = func() time.Time {
		calls++
		return time.Date(2026, 11, 16, 23, 59, 59, 900, time.FixedZone("X", 3600))
	}
	t.Cleanup(func() { nowFunc = prev })

	var first, second time.Time
	h := attachReadTime(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		first = requestReadTime(r.Context())
		second = requestReadTime(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	want := time.Date(2026, 11, 16, 22, 59, 59, 0, time.UTC)
	if !first.Equal(want) || !second.Equal(want) || calls != 1 {
		t.Errorf("read times %v, %v after %d clock reads; want %v once", first, second, calls, want)
	}
}

// TestResolvedDateVariables pins what the CalDAV watermark ctag folds in: a
// collection whose `where:` reads `$today` gets a new tag each day.
func TestResolvedDateVariables(t *testing.T) {
	where := []string{"status!=done", "due>=$today", "start<=$today+7d", "not a clause ("}
	mon := resolvedDateVariables(where, time.Date(2026, 11, 16, 23, 0, 0, 0, time.UTC))
	tue := resolvedDateVariables(where, time.Date(2026, 11, 17, 0, 0, 1, 0, time.UTC))
	if !slices.Equal(mon, []string{"2026-11-16", "2026-11-23"}) {
		t.Errorf("monday = %v", mon)
	}
	if slices.Equal(mon, tue) {
		t.Error("the values must change with the day")
	}
	if got := resolvedDateVariables([]string{"status!=done"}, time.Now()); got != nil {
		t.Errorf("no variables: %v", got)
	}
}
