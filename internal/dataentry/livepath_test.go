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
		{"list with busy", "/api/v1/tickets?busy=run", "/api/v1/tickets?busy=run"},
		{"list with filter op", "/api/v1/tickets?filter[status][ne]=done", "/api/v1/tickets?filter%5Bstatus%5D%5Bne%5D=done"},
		{"view", "/api/v1/_views/ticket/TKT-1", "/api/v1/_views/ticket/TKT-1"},
		{"presence", "/api/v1/tickets/TKT-1/_presence", "/api/v1/tickets/TKT-1/_presence"},
		{"presence with face and world", "/api/v1/tickets/TKT-1@draft/_presence?world=main",
			"/api/v1/tickets/TKT-1@draft/_presence?world=main"},
		{"inbox", "/api/v1/_inbox", "/api/v1/_inbox"},
		{"inbox with limit and kinds", "/api/v1/_inbox?limit=10&kinds=waiting,mention",
			"/api/v1/_inbox?kinds=mention%2Cwaiting&limit=10"},
		{"inbox limit without leading zero", "/api/v1/_inbox?limit=05", "/api/v1/_inbox?limit=5"},
		{"inbox kinds deduplicated", "/api/v1/_inbox?kinds=waiting,mention,waiting",
			"/api/v1/_inbox?kinds=mention%2Cwaiting"},
		{"inbox summary", "/api/v1/_inbox/summary", "/api/v1/_inbox/summary"},
		{"channel list", "/api/v1/tickets/TKT-1/_channels", "/api/v1/tickets/TKT-1/_channels"},
		{"channel list by state", "/api/v1/tickets/TKT-1/_channels?state=open&world=main",
			"/api/v1/tickets/TKT-1/_channels?state=open&world=main"},
		{"channel head", "/api/v1/tickets/TKT-1/_channels/room", "/api/v1/tickets/TKT-1/_channels/room"},
		// `messages` is a subresource only after a type; a channel type may
		// carry the name.
		{"head of a type named messages", "/api/v1/tickets/TKT-1/_channels/messages",
			"/api/v1/tickets/TKT-1/_channels/messages"},
		{"named channel head", "/api/v1/tickets/TKT-1@draft/_channels/threads/0a1b2c3d4e5f6",
			"/api/v1/tickets/TKT-1@draft/_channels/threads/0a1b2c3d4e5f6"},
		// Every ListParams key in frontend/src/types/entity.ts except the
		// search box's q: EntityList subscribes the params it fetches with,
		// and one refused path fails the whole PUT.
		{
			"list with every client parameter",
			"/api/v1/tickets?page=1&per_page=25&cursor=c&sort=title&fields=title&include=*&comment_counts=true" +
				"&busy=run&world=main&list_id=l&query_scope=open&scope_page=p&scope_tab=t&anchor=A-1" +
				"&relation=r&direction=incoming&filter[status]=open",
			"/api/v1/tickets?anchor=A-1&busy=run&comment_counts=true&cursor=c&direction=incoming&fields=title" +
				"&filter%5Bstatus%5D=open&include=%2A&list_id=l&page=1&per_page=25&query_scope=open" +
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
		// q runs the unscoped searcher and gates its capped hits afterwards,
		// so a hidden write can evict a visible row.
		{"list free-text search", "/api/v1/tickets?q=text"},
		{"list free-text search with filter", "/api/v1/tickets?filter[status]=open&q=text"},
		{"presence unknown query key", "/api/v1/tickets/TKT-1/_presence?include=x"},
		{"presence on unknown plural", "/api/v1/nopes/TKT-1/_presence"},
		{"presence on reserved collection", "/api/v1/_views/TKT-1/_presence"},
		{"presence on sub-resource id", "/api/v1/tickets/_export/_presence"},
		// A frame on the head is the signal to fetch messages?after_rev=;
		// the messages are not a live shape.
		{"channel messages", "/api/v1/tickets/TKT-1/_channels/room/messages"},
		{"messages of a type named messages", "/api/v1/tickets/TKT-1/_channels/messages/messages"},
		{"named channel messages", "/api/v1/tickets/TKT-1/_channels/threads/0a1b2c3d4e5f6/messages"},
		{"channel message", "/api/v1/tickets/TKT-1/_channels/room/messages/1"},
		{"channel too deep", "/api/v1/tickets/TKT-1/_channels/threads/0a1b2c3d4e5f6/x"},
		{"channel invalid type", "/api/v1/tickets/TKT-1/_channels/Room"},
		{"channel invalid name", "/api/v1/tickets/TKT-1/_channels/threads/NAME"},
		{"channels on unknown plural", "/api/v1/nopes/TKT-1/_channels"},
		{"channels on sub-resource id", "/api/v1/tickets/_export/_channels"},
		{"channel list unknown query key", "/api/v1/tickets/TKT-1/_channels?include=x"},
		{"channel head state", "/api/v1/tickets/TKT-1/_channels/room?state=open"},
		{"inbox cursor", "/api/v1/_inbox?cursor=x"},
		{"inbox q", "/api/v1/_inbox?q=x"},
		{"inbox unknown key", "/api/v1/_inbox?foo=1"},
		{"inbox limit zero", "/api/v1/_inbox?limit=0"},
		{"inbox limit over max", "/api/v1/_inbox?limit=201"},
		{"inbox limit not a number", "/api/v1/_inbox?limit=abc"},
		{"inbox bad kind", "/api/v1/_inbox?kinds=bogus"},
		{"inbox empty kinds", "/api/v1/_inbox?kinds="},
		{"inbox repeated kinds", "/api/v1/_inbox?kinds=waiting&kinds=mention"},
		{"inbox repeated limit", "/api/v1/_inbox?limit=5&limit=6"},
		{"inbox summary with query", "/api/v1/_inbox/summary?limit=1"},
		{"inbox other", "/api/v1/_inbox/other"},
		{"inbox too deep", "/api/v1/_inbox/summary/x"},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := parseLivePath(tc.raw, meta); !errors.Is(err, errLivePath) {
				t.Fatalf("parseLivePath(%q) = %v, want errLivePath", tc.raw, err)
			}
		})
	}
}

// A presence path on a faced type names a face: the route would answer a
// bare id with face_required, so the subscription could never join.
func TestParseLivePath_PresenceOnFacedTypeNamesAFace(t *testing.T) {
	meta := facedTicketApp(t).State().Meta
	if _, err := parseLivePath("/api/v1/tickets/TKT-1/_presence", meta); !errors.Is(err, errLivePath) {
		t.Fatalf("bare id on a faced type: err = %v, want errLivePath", err)
	}
	for _, ok := range []string{"/api/v1/tickets/TKT-1@draft/_presence", "/api/v1/tickets/TKT-1"} {
		if _, err := parseLivePath(ok, meta); err != nil {
			t.Fatalf("%s: %v", ok, err)
		}
	}
}

// presenceID is the bare entity id of a presence path and empty for every
// other shape.
func TestLivePath_PresenceID(t *testing.T) {
	meta := newTestAppV1(t).State().Meta
	for raw, want := range map[string]string{
		"/api/v1/tickets/TKT-1/_presence":       "TKT-1",
		"/api/v1/tickets/TKT-1@draft/_presence": "TKT-1",
		"/api/v1/tickets/TKT-1":                 "",
		"/api/v1/_views/ticket/TKT-1":           "",
	} {
		lp, err := parseLivePath(raw, meta)
		if err != nil {
			t.Fatal(err)
		}
		if got := lp.presenceID(); got != want {
			t.Errorf("%s: presenceID = %q, want %q", raw, got, want)
		}
	}
}

// busyType is the channel type of a busy list path and empty for every
// other shape, a detail or a view included.
func TestLivePath_BusyType(t *testing.T) {
	meta := newTestAppV1(t).State().Meta
	for raw, want := range map[string]string{
		"/api/v1/tickets?busy=run":            "run",
		"/api/v1/tickets?per_page=5&busy=job": "job",
		"/api/v1/tickets":                     "",
		"/api/v1/tickets/TKT-1/_presence":     "",
		"/api/v1/tickets/TKT-1/_channels/run": "",
		"/api/v1/_inbox":                      "",
	} {
		lp, err := parseLivePath(raw, meta)
		if err != nil {
			t.Fatalf("parseLivePath(%q) = %v", raw, err)
		}
		if got := lp.busyType(); got != want {
			t.Errorf("%s: busyType = %q, want %q", raw, got, want)
		}
	}
}

// TestLivePath_InboxCanonical pins AC 7: equal inbox requests rebuild one
// replay URL, so they share one replay pair.
func TestLivePath_InboxCanonical(t *testing.T) {
	meta := newTestAppV1(t).State().Meta
	for _, group := range [][]string{
		{"/api/v1/_inbox?kinds=mention,waiting", "/api/v1/_inbox?kinds=waiting,mention,waiting"},
		{"/api/v1/_inbox?limit=05", "/api/v1/_inbox?limit=5"},
		{"/api/v1/_inbox?limit=5&kinds=reply", "/api/v1/_inbox?kinds=reply&limit=005"},
	} {
		var want string
		for _, raw := range group {
			lp, err := parseLivePath(raw, meta)
			if err != nil {
				t.Fatalf("parseLivePath(%q) = %v", raw, err)
			}
			if !lp.isInbox() {
				t.Fatalf("%q is not an inbox path", raw)
			}
			if want == "" {
				want = lp.String()
			} else if got := lp.String(); got != want {
				t.Fatalf("%q replays %q, want %q like %q", raw, got, want, group[0])
			}
		}
	}
}
