package dataentry

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// Feed bodies embed genuinely user-authored entity content: feed_provider.go
// reads Summary/Description straight off entity properties, and anyone who can
// write an entity controls those strings. The feed is therefore a real XSS
// surface unless two things hold, which these tests pin (gosec G705 on
// feed_handler.go):
//
//  1. The body is served with a non-HTML Content-Type AND nosniff, so a browser
//     cannot MIME-sniff it into text/html and run the payload.
//  2. The serializer escapes for its own grammar, so a payload cannot break out
//     of the field it sits in and forge feed structure.

// feedXSSPayload breaks out of an iCalendar TEXT value (via the ; , and CRLF
// metacharacters) and out of HTML element context, in one string.
const feedXSSPayload = "Evil<script>alert(1)</script>;DTSTART:20200101\r\nX-INJECTED:yes,tail"

// xssFeedApp builds a feed app whose single task carries the payload in its
// title (the SUMMARY source).
func xssFeedApp(t *testing.T) *App {
	t.Helper()
	return feedHandlerApp(t,
		map[string]dataentryconfig.Feed{
			"tasks": {
				Meta: dataentryconfig.FeedMeta{Name: "PIM tasks"},
				Sources: []dataentryconfig.FeedSource{
					{EntityType: "task", Date: "due", Summary: "title"},
				},
			},
		},
		task("TSK-1", feedXSSPayload, "todo", "2026-07-10"),
	)
}

// TestFeedHandler_NosniffOnBothFormats pins the header that binds the declared
// Content-Type. Without nosniff a browser may sniff a text/calendar or
// application/json body containing "<script>" as HTML and execute it.
func TestFeedHandler_NosniffOnBothFormats(t *testing.T) {
	for _, ext := range []string{"ics", "json"} {
		t.Run(ext, func(t *testing.T) {
			rec := doFeed(t, xssFeedApp(t), "/api/v1/_feeds/tasks."+ext)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
			}
		})
	}
}

// TestFeedHandler_ICSEscapesHostileEntityContent pins that a hostile SUMMARY
// cannot forge iCalendar structure: the RFC 5545 metacharacters are escaped and
// the embedded CRLF is stripped, so no attacker-authored property line appears.
func TestFeedHandler_ICSEscapesHostileEntityContent(t *testing.T) {
	rec := doFeed(t, xssFeedApp(t), "/api/v1/_feeds/tasks.ics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// The CRLF must not have produced a new content line.
	for _, forged := range []string{"\r\nX-INJECTED:yes", "\r\nDTSTART:20200101"} {
		if strings.Contains(body, forged) {
			t.Errorf("CRLF injection forged a property line %q; body:\n%q", forged, body)
		}
	}
	// The structural metacharacters must be backslash-escaped per RFC 5545.
	// Compare against the unfolded body: a 75-octet content line is folded with
	// CRLF + a leading space, which can split an escape sequence's neighbors.
	unfolded := strings.ReplaceAll(body, "\r\n ", "")
	if !strings.Contains(unfolded, `\;DTSTART`) {
		t.Errorf("semicolon not escaped in TEXT value; unfolded:\n%q", unfolded)
	}
	if !strings.Contains(unfolded, `\,tail`) {
		t.Errorf("comma not escaped in TEXT value; unfolded:\n%q", unfolded)
	}
	// The CRLF became the literal two-character \n escape, not a line break.
	if !strings.Contains(unfolded, `\nX-INJECTED`) {
		t.Errorf("CRLF not collapsed into the \\n escape; unfolded:\n%q", unfolded)
	}
	// Exactly one DTSTART (the real one) — the payload's must not have landed.
	if n := strings.Count(body, "DTSTART:"); n != 1 {
		t.Errorf("DTSTART count = %d, want 1; body:\n%q", n, body)
	}
}

// TestFeedHandler_JSONEscapesHostileEntityContent pins that the JSON encoder
// escapes the HTML-significant characters, so the payload is inert even if the
// body is somehow interpreted as HTML.
func TestFeedHandler_JSONEscapesHostileEntityContent(t *testing.T) {
	rec := doFeed(t, xssFeedApp(t), "/api/v1/_feeds/tasks.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if strings.Contains(body, "<script>") {
		t.Errorf("raw <script> in JSON body; encoding/json should emit \\u003c; body:\n%s", body)
	}
	// encoding/json escapes <, > and & by default (SetEscapeHTML), so the
	// payload appears only in \u-escaped form. The want string is built with an
	// escaped backslash so it matches the six literal bytes < on the wire.
	wantEscaped := "\\u003cscript\\u003e"
	if !strings.Contains(body, wantEscaped) {
		t.Errorf("expected escaped %s; body:\n%s", wantEscaped, body)
	}

	// The payload must survive intact as DATA (round-trips to the original
	// string) — escaping must not silently corrupt the value.
	var decoded struct {
		Events []struct {
			Summary string `json:"summary"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v; body:\n%s", err, body)
	}
	if len(decoded.Events) != 1 {
		t.Fatalf("events = %d, want 1; body:\n%s", len(decoded.Events), body)
	}
	if decoded.Events[0].Summary != feedXSSPayload {
		t.Errorf("summary round-trip = %q, want %q", decoded.Events[0].Summary, feedXSSPayload)
	}
}
