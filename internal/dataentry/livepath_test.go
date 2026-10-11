package dataentry

import (
	"errors"
	"testing"
)

func TestParseLivePath(t *testing.T) {
	meta := newTestAppV1(t).State().Meta

	accepted := []struct {
		name, raw, replay string
	}{
		{"entity", "/api/v1/tickets/TKT-1", "/api/v1/tickets/TKT-1"},
		{"entity with face", "/api/v1/tickets/TKT-1@draft", "/api/v1/tickets/TKT-1@draft"},
		{"entity with world", "/api/v1/tickets/TKT-1?world=main", "/api/v1/tickets/TKT-1?world=main"},
		{"list", "/api/v1/tickets", "/api/v1/tickets"},
		{"list with paging", "/api/v1/tickets?per_page=20&page=2", "/api/v1/tickets?page=2&per_page=20"},
		{"list with filter", "/api/v1/tickets?filter%5Bstatus%5D=open", "/api/v1/tickets?filter%5Bstatus%5D=open"},
		{"list with filter op", "/api/v1/tickets?filter[status][ne]=done", "/api/v1/tickets?filter%5Bstatus%5D%5Bne%5D=done"},
		{"view", "/api/v1/_views/ticket/TKT-1", "/api/v1/_views/ticket/TKT-1"},
		// Every ListParams key in frontend/src/types/entity.ts, plus the
		// search box's q: EntityList subscribes the params it fetches with,
		// and one refused path fails the whole PUT.
		{
			"list with every client parameter",
			"/api/v1/tickets?page=1&per_page=25&cursor=c&sort=title&fields=title&include=*&comment_counts=true" +
				"&world=main&list_id=l&query_scope=open&scope_page=p&scope_tab=t&anchor=A-1" +
				"&relation=r&direction=incoming&q=text&filter[status]=open",
			"/api/v1/tickets?anchor=A-1&comment_counts=true&cursor=c&direction=incoming&fields=title" +
				"&filter%5Bstatus%5D=open&include=%2A&list_id=l&page=1&per_page=25&q=text&query_scope=open" +
				"&relation=r&scope_page=p&scope_tab=t&sort=title&world=main",
		},
	}
	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			lp, err := parseLivePath(tc.raw, meta)
			if err != nil {
				t.Fatalf("parseLivePath(%q): %v", tc.raw, err)
			}
			if got := lp.String(); got != tc.replay {
				t.Fatalf("replay URL = %q, want %q", got, tc.replay)
			}
		})
	}

	rejected := []struct {
		name, raw string
	}{
		{"empty", ""},
		{"events stream", "/api/v1/_events"},
		{"collection export", "/api/v1/tickets/_export"},
		{"document", "/api/v1/_documents/report"},
		{"history", "/api/v1/_history/TKT-1"},
		{"comments", "/api/v1/_comments/TKT-1"},
		{"action", "/api/v1/tickets/TKT-1/_action/close"},
		{"sub-resource", "/api/v1/tickets/TKT-1/relations"},
		{"view id with slash", "/api/v1/_views/ticket/TKT-1/extra"},
		{"unknown view type", "/api/v1/_views/nope/TKT-1"},
		{"unknown plural", "/api/v1/nopes"},
		{"escaped slash", "/api/v1/tickets/TKT%2F1"},
		{"double slash", "/api/v1//tickets"},
		{"dot dot", "/api/v1/features/../tickets"},
		{"trailing slash", "/api/v1/tickets/"},
		{"absolute URL", "https://example.com/api/v1/tickets"},
		{"scheme-relative", "//example.com/api/v1/tickets"},
		{"fragment", "/api/v1/tickets#x"},
		{"legacy API", "/api/tickets"},
		{"invalid id", "/api/v1/tickets/TKT--1"},
		{"entity unknown query key", "/api/v1/tickets/TKT-1?refresh=true"},
		{"list unknown query key", "/api/v1/tickets?format=csv"},
		{"view unknown query key", "/api/v1/_views/ticket/TKT-1?include=x"},
		{"filter on entity", "/api/v1/tickets/TKT-1?filter[status]=open"},
		{"malformed filter", "/api/v1/tickets?filter[st atus]=open"},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := parseLivePath(tc.raw, meta); !errors.Is(err, errLivePath) {
				t.Fatalf("parseLivePath(%q) = %v, want errLivePath", tc.raw, err)
			}
		})
	}
}
