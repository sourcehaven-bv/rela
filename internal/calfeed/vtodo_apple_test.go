package calfeed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures in testdata/ are REAL bytes captured from Apple Reminders on
// macOS 26.5.1 (2026-08-09) syncing against a local CalDAV server. They are the
// ground truth for what this package must interoperate with, so the properties
// asserted below are observations, not guesses.
//
// calfeed is a render-only package (model → bytes); parsing VTODO belongs to the
// CalDAV adapter, which uses go-ical. So these tests do NOT round-trip through a
// parser. Instead they pin the two things that are this package's job:
//
//  1. the properties Apple actually emits are ones we can produce, and
//  2. our rendering of the equivalent model matches Apple's semantically.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// fixtureProp returns the value of the first occurrence of an unfolded property
// line, e.g. prop "STATUS" of "STATUS:COMPLETED" → "COMPLETED".
func fixtureProp(t *testing.T, body, name string) string {
	t.Helper()
	for line := range strings.SplitSeq(unfold(body), crlf) {
		if after, ok := strings.CutPrefix(line, name+":"); ok {
			return after
		}
		// Parameterised form, e.g. DUE;VALUE=DATE:20260810
		if strings.HasPrefix(line, name+";") {
			if _, v, found := strings.Cut(line, ":"); found {
				return v
			}
		}
	}
	return ""
}

// TestApple_CompletedTodoShape pins the completion trio as Apple writes it. If
// this ever changes, the mapping design (which treats completion as ONE logical
// event) needs revisiting.
func TestApple_CompletedTodoShape(t *testing.T) {
	body := readFixture(t, "apple-completed-todo.ics")

	checks := map[string]string{
		"STATUS":           "COMPLETED",
		"PERCENT-COMPLETE": "100",
		"COMPLETED":        "20260809T081406Z",
	}
	for prop, want := range checks {
		if got := fixtureProp(t, body, prop); got != want {
			t.Errorf("Apple fixture %s = %q, want %q", prop, got, want)
		}
	}

	// Apple keeps properties it does not model — the deep link survived, which
	// is what lets a to-do carry a route back into the app.
	if got := fixtureProp(t, body, "URL"); !strings.Contains(got, "/entity/task/") {
		t.Errorf("URL did not survive the round-trip: %q", got)
	}
	// It normalises URL to the VALUE=URI form; we must not assume the bare form.
	if !strings.Contains(unfold(body), "URL;VALUE=URI:") {
		t.Error("expected Apple's URL;VALUE=URI normalisation in the fixture")
	}
}

// TestApple_AddsDTSTARTMirroringDUE records an Apple behavior that would
// otherwise look like user intent: it invents a DTSTART equal to DUE. A diffing
// consumer must not treat that as an edit.
func TestApple_AddsDTSTARTMirroringDUE(t *testing.T) {
	body := readFixture(t, "apple-completed-todo.ics")

	due := fixtureProp(t, body, "DUE")
	dtstart := fixtureProp(t, body, "DTSTART")
	if due == "" || dtstart == "" {
		t.Fatalf("fixture missing DUE (%q) or DTSTART (%q)", due, dtstart)
	}
	if due != dtstart {
		t.Errorf("expected Apple to mirror DUE into DTSTART, got DUE=%q DTSTART=%q", due, dtstart)
	}
}

// TestApple_ClientCreatedTodoIsBareUUID is the fact that makes the alias table
// mandatory: a to-do created in Reminders carries a bare UUID as its UID, which
// can never be a rela entity ID (those must start with a letter or digit).
func TestApple_ClientCreatedTodoIsBareUUID(t *testing.T) {
	body := readFixture(t, "apple-client-created-todo.ics")

	uid := fixtureProp(t, body, "UID")
	if uid == "" {
		t.Fatal("fixture has no UID")
	}
	if strings.Contains(uid, "@") {
		t.Errorf("expected a bare UUID with no domain part, got %q", uid)
	}
	if _, _, found := strings.Cut(uid, feedUIDSepForTest); found {
		t.Errorf("expected no rela type prefix in a client-minted UID, got %q", uid)
	}

	// It arrives with essentially nothing but a title — which is why a
	// create-target entity type must be constructible from SUMMARY alone.
	if got := fixtureProp(t, body, "SUMMARY"); got == "" {
		t.Error("client-created to-do has no SUMMARY")
	}
	for _, absent := range []string{"DUE", "PRIORITY", "COMPLETED"} {
		if got := fixtureProp(t, body, absent); got != "" {
			t.Errorf("expected no %s on a freshly created to-do, got %q", absent, got)
		}
	}
	if got := fixtureProp(t, body, "STATUS"); got != "NEEDS-ACTION" {
		t.Errorf("STATUS = %q, want NEEDS-ACTION", got)
	}
}

// feedUIDSepForTest mirrors the "--" separator the dataentry feed UID scheme
// uses. Duplicated as a literal rather than imported: calfeed is a leaf package
// and must not depend on dataentry.
const feedUIDSepForTest = "--"

// appleOnlyProps are properties present in a captured fixture that this
// renderer deliberately does not reproduce. Everything NOT listed here must
// match, so the comparison below is a denylist rather than a whitelist: a
// property Apple starts emitting fails loudly instead of going unnoticed.
//
// Each exclusion is a decision, not an oversight:
var appleOnlyProps = map[string]string{
	"PRODID":             "identifies the writing client; ours is rela's",
	"DTSTAMP":            "the render clock, injected per render",
	"VERSION":            "envelope, not a VTODO property",
	"CALSCALE":           "envelope, not a VTODO property",
	"CREATED":            "Apple bookkeeping; rela's own timestamps live on the entity",
	"LAST-MODIFIED":      "Apple bookkeeping, as above",
	"X-APPLE-SORT-ORDER": "vendor extension for Reminders' manual ordering",
	"DTSTART":            "Apple mirrors DUE into DTSTART; we deliberately omit it (RFC 5545 §3.6.2 permits DUE alone)",
	"BEGIN":              "block delimiter",
	"END":                "block delimiter",
}

// TestRenderTodo_MatchesAppleSemantics renders the model equivalent of the
// captured completed to-do and asserts that EVERY property Apple emitted is
// either reproduced with the same value or explicitly excluded above.
//
// Byte equality is deliberately NOT asserted — Apple re-sorts properties and
// stamps its own PRODID/DTSTAMP, which is precisely why consumers must diff on
// parsed semantics rather than raw bytes.
func TestRenderTodo_MatchesAppleSemantics(t *testing.T) {
	fixture := readFixture(t, "apple-completed-todo.ics")

	todo := Todo{
		UID:         fixtureProp(t, fixture, "UID"),
		Summary:     "rela probe: check this box",
		Due:         time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		URL:         "http://127.0.0.1:8080/entity/task/TKT-PROBE1",
		Priority:    5,
		Description: "Seeded by rela CalDAV feasibility test. Check it off in Reminders; then we verify the server saw STATUS:COMPLETED.",
	}
	todo.Complete(time.Date(2026, 8, 9, 8, 14, 6, 0, time.UTC))

	ours := string(testICal().RenderTodo(todo))

	var compared int
	for line := range strings.SplitSeq(unfold(fixture), crlf) {
		if line == "" {
			continue
		}
		name, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name, _, _ = strings.Cut(name, ";") // drop parameters, e.g. DUE;VALUE=DATE
		if _, skip := appleOnlyProps[name]; skip {
			continue
		}
		want := fixtureProp(t, fixture, name)
		got := fixtureProp(t, ours, name)
		if got != want {
			t.Errorf("%s: rendered %q, Apple had %q", name, got, want)
		}
		compared++
	}

	// Guard the guard: if the fixture or the skip-list drifted such that
	// nothing is compared, the test would pass vacuously.
	if compared < 5 {
		t.Fatalf("only %d properties compared — the fixture or appleOnlyProps has drifted", compared)
	}

	// DUE must additionally agree on its VALUE type, which fixtureProp discards.
	if !strings.Contains(unfold(ours), "DUE;VALUE=DATE:") {
		t.Error("an all-day due must render as DUE;VALUE=DATE")
	}
}
