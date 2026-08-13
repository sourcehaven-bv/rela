package dataentry

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

const ctagPropfind = `<?xml version="1.0"?>` +
	`<propfind xmlns="DAV:" xmlns:cs="http://calendarserver.org/ns/">` +
	`<prop><displayname/><cs:getctag/></prop></propfind>`

var ctagRe = regexp.MustCompile(`<getctag[^>]*>([^<]*)</getctag>`)

// fetchCTag drives a real collection PROPFIND and returns the ctag, if served.
func fetchCTag(t *testing.T, app *App) string {
	t.Helper()
	rec := doCalDAV(t, app, "PROPFIND",
		"/api/v1/_caldav/principal/calendars/tasks/", ctagPropfind)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND = %d, want 207\n%s", rec.Code, rec.Body.String())
	}
	m := ctagRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		return ""
	}
	return m[1]
}

// TestCalDAVCTag_ServedInCalendarserverNamespace pins the wire contract. The
// property is a CalendarServer extension, not a DAV: one, and a client looking
// for it in the wrong namespace simply will not find it.
func TestCalDAVCTag_ServedInCalendarserverNamespace(t *testing.T) {
	app := caldavTestApp(t, task("TSK-a", "A", "todo", ""))
	rec := doCalDAV(t, app, "PROPFIND",
		"/api/v1/_caldav/principal/calendars/tasks/", ctagPropfind)
	body := rec.Body.String()

	if !strings.Contains(body, `xmlns="http://calendarserver.org/ns/"`) {
		t.Errorf("getctag not in the calendarserver namespace:\n%s", body)
	}
	if fetchCTag(t, app) == "" {
		t.Error("no ctag served for a collection PROPFIND that asked for one")
	}
	// The wrapper must not disturb what go-webdav already answers.
	if !strings.Contains(body, "rela Tasks") {
		t.Error("splicing the ctag dropped displayname from the response")
	}
}

// TestCalDAVCTag_ChangesIffContentChanges is the whole contract: a client that
// sees an unchanged ctag skips the collection entirely, so a missed change
// leaves it permanently stale, and a spurious change costs a full resync.
func TestCalDAVCTag_ChangesIffContentChanges(t *testing.T) {
	t.Run("stable across repeated polls", func(t *testing.T) {
		app := caldavTestApp(t, task("TSK-a", "A", "todo", ""))
		if first, second := fetchCTag(t, app), fetchCTag(t, app); first != second {
			t.Errorf("ctag changed with no write: %s then %s — every poll would resync", first, second)
		}
	})

	t.Run("differs when an entry differs", func(t *testing.T) {
		before := fetchCTag(t, caldavTestApp(t, task("TSK-a", "A", "todo", "")))
		after := fetchCTag(t, caldavTestApp(t, task("TSK-a", "A EDITED", "todo", "")))
		if before == after {
			t.Error("ctag unchanged after an edit; the client would never re-read")
		}
	})

	t.Run("differs when an entry is removed", func(t *testing.T) {
		both := fetchCTag(t, caldavTestApp(t,
			task("TSK-a", "A", "todo", ""), task("TSK-b", "B", "todo", "")))
		one := fetchCTag(t, caldavTestApp(t, task("TSK-a", "A", "todo", "")))
		if both == one {
			t.Error("ctag unchanged after a removal — the deletion case a " +
				"counter-derived tag misses when an event is dropped")
		}
	})
}

// TestCalDAVCTag_NotComputedWhenNotAsked guards the cost. Computing the ctag
// renders the whole collection, so doing it for requests that never asked
// would make every PROPFIND pay for an optimisation nobody requested.
func TestCalDAVCTag_NotComputedWhenNotAsked(t *testing.T) {
	app := caldavTestApp(t, task("TSK-a", "A", "todo", ""))

	t.Run("a propfind for other props gets no ctag", func(t *testing.T) {
		rec := doCalDAV(t, app, "PROPFIND", "/api/v1/_caldav/principal/calendars/tasks/",
			`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><displayname/></prop></propfind>`)
		if ctagRe.MatchString(rec.Body.String()) {
			t.Error("served a ctag nobody asked for")
		}
	})

	t.Run("allprop gets no ctag", func(t *testing.T) {
		// RFC 4918 8.1 excludes extension properties from allprop, and an empty
		// body means allprop — computing one here would render the collection
		// on every bare PROPFIND.
		rec := doCalDAV(t, app, "PROPFIND", "/api/v1/_caldav/principal/calendars/tasks/", "")
		if ctagRe.MatchString(rec.Body.String()) {
			t.Error("allprop returned a ctag; every bare PROPFIND now renders the collection")
		}
	})
}

// TestCalDAVCTag_OnTheHomeSetPropfind covers the request Apple Reminders
// actually sends — and the one an earlier implementation silently failed.
//
// Reminders does not PROPFIND each collection individually: it sends ONE
// Depth:1 PROPFIND at the calendar home set, whose multistatus carries the home
// set followed by every member collection. An implementation that splices into
// the first <prop> it finds writes into the HOME SET's response, so every
// member collection is left with go-webdav's empty, 404-status placeholder —
// and the client gets no ctag at all despite asking for one.
//
// Verified against a live accountsd before this test was written.
func TestCalDAVCTag_OnTheHomeSetPropfind(t *testing.T) {
	app := caldavTestApp(t, task("TSK-a", "A", "todo", ""))

	rec := doCalDAV(t, app, "PROPFIND",
		"/api/v1/_caldav/principal/calendars/", ctagPropfind)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND = %d, want 207", rec.Code)
	}
	body := rec.Body.String()

	collection := responseFor(t, body, "/api/v1/_caldav/principal/calendars/tasks/")
	m := ctagRe.FindStringSubmatch(collection)
	if len(m) < 2 || m[1] == "" {
		t.Fatalf("member collection got no ctag from a home-set PROPFIND:\n%s", collection)
	}
	// An empty element under 404 is what go-webdav emits for a property it does
	// not know. Serving that IS the bug: the client sees the property answered
	// and answered as missing.
	if strings.Contains(collection, "404 Not Found") {
		t.Errorf("ctag served under a 404 propstat; the client reads it as absent:\n%s", collection)
	}
}

// responseFor extracts the <response> element whose href matches.
func responseFor(t *testing.T, multistatus, href string) string {
	t.Helper()
	for part := range strings.SplitSeq(multistatus, "<response") {
		if strings.Contains(part, "<href>"+href+"</href>") {
			return part
		}
	}
	t.Fatalf("no <response> for %s in:\n%s", href, multistatus)
	return ""
}

// TestCalDAV_PropPatchIsRefusedNotUnimplemented pins the difference between
// "I refuse this property" and "I do not implement this method".
//
// go-webdav answers PROPPATCH with a flat 501. Apple Reminders reads that as a
// failed sync step and NEVER ENUMERATES THE COLLECTION — observed against
// remindd/3976: 27 PROPFINDs, 12 PROPPATCHes, zero REPORTs, zero resources
// fetched, repeating every ~3 minutes. The account looks connected and stays
// permanently empty.
//
// RFC 4918 §9.2 wants a 207 with a per-property status, which lets the client
// carry on.
func TestCalDAV_PropPatchIsRefusedNotUnimplemented(t *testing.T) {
	app := caldavTestApp(t, task("TSK-a", "A", "todo", ""))
	body := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<A:propertyupdate xmlns:A="DAV:"><A:set><A:prop>` +
		`<D:calendar-color xmlns:D="http://apple.com/ns/ical/" symbolic-color="custom">#007AFF</D:calendar-color>` +
		`</A:prop></A:set></A:propertyupdate>`

	rec := doCalDAV(t, app, "PROPPATCH", "/api/v1/_caldav/principal/calendars/tasks/", body)

	if rec.Code == http.StatusNotImplemented {
		t.Fatal("501 for PROPPATCH: Apple Reminders treats this as a fatal sync " +
			"error and never enumerates the collection")
	}
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPPATCH = %d, want 207 (RFC 4918 9.2)\n%s", rec.Code, rec.Body.String())
	}

	out := rec.Body.String()
	// The refusal must name the property the client asked about...
	if !strings.Contains(out, "calendar-color") {
		t.Errorf("the response does not name the refused property:\n%s", out)
	}
	// ...and refuse it explicitly. A 200 here would be a lie: the client would
	// display a color that reverts on the next poll.
	if !strings.Contains(out, "403 Forbidden") {
		t.Errorf("want a per-property 403; claiming success for a discarded value "+
			"makes the client show something that reverts:\n%s", out)
	}
}
