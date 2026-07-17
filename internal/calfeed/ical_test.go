package calfeed

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// fixedNow is the injected clock used across serializer tests so DTSTAMP is
// deterministic.
var fixedNow = time.Date(2026, 7, 4, 12, 30, 0, 0, time.UTC)

func testICal() ICal { return ICal{ProdID: "-//test//test//EN", Now: fixedNow} }

func day(m time.Month, d int) time.Time {
	return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
}

// dayTime builds a UTC instant with a time-of-day, for timed-event tests.
func dayTime(mo time.Month, d, h, mn int) time.Time {
	return time.Date(2026, mo, d, h, mn, 0, 0, time.UTC)
}

// unfold reverses RFC 5545 line folding so a test can assert on logical lines.
func unfold(s string) string { return strings.ReplaceAll(s, "\r\n ", "") }

// logicalLines splits an unfolded iCalendar body into its CRLF-separated lines.
func logicalLines(t *testing.T, body []byte) []string {
	t.Helper()
	s := string(body)
	if !strings.HasSuffix(s, crlf) {
		t.Fatalf("body does not end with CRLF: %q", tail(s))
	}
	s = unfold(s)
	// Every line must be CRLF-terminated; splitting on CRLF yields a trailing "".
	parts := strings.Split(s, crlf)
	if parts[len(parts)-1] != "" {
		t.Fatalf("content after final CRLF: %q", parts[len(parts)-1])
	}
	return parts[:len(parts)-1]
}

func tail(s string) string {
	if len(s) > 40 {
		return s[len(s)-40:]
	}
	return s
}

func TestRenderCollection_Envelope(t *testing.T) {
	f := Feed{Name: "PIM tasks", Events: []Event{
		{UID: "TSK-1@rela", Summary: "A", Start: day(7, 10)},
		{UID: "TSK-2@rela", Summary: "B", Start: day(7, 11)},
	}}
	lines := logicalLines(t, testICal().RenderCollection(f))

	if lines[0] != "BEGIN:VCALENDAR" {
		t.Errorf("first line = %q, want BEGIN:VCALENDAR", lines[0])
	}
	if lines[len(lines)-1] != "END:VCALENDAR" {
		t.Errorf("last line = %q, want END:VCALENDAR", lines[len(lines)-1])
	}
	if !containsLine(lines, "VERSION:2.0") {
		t.Error("missing VERSION:2.0")
	}
	if !containsLine(lines, "PRODID:-//test//test//EN") {
		t.Error("missing PRODID")
	}
	if !containsLine(lines, "X-WR-CALNAME:PIM tasks") {
		t.Error("missing X-WR-CALNAME")
	}
	if n := countLine(lines, "BEGIN:VEVENT"); n != 2 {
		t.Errorf("BEGIN:VEVENT count = %d, want 2", n)
	}
}

// TestRenderCollection_IsWrappedRenderEvent pins AC7: the collection body equals
// the VCALENDAR envelope wrapping exactly RenderEvent per event, so the two code
// paths can never diverge.
func TestRenderCollection_IsWrappedRenderEvent(t *testing.T) {
	ic := testICal()
	events := []Event{
		{UID: "TSK-1@rela", Summary: "A", Start: day(7, 10)},
		{UID: "TSK-2@rela", Summary: "B", Start: day(7, 11), Alarms: []Alarm{{Trigger: "-PT9H"}}},
	}
	f := Feed{Name: "cal", Events: events}
	got := string(ic.RenderCollection(f))

	var want strings.Builder
	for _, e := range events {
		want.Write(ic.RenderEvent(e))
	}
	// The per-event blocks must appear verbatim, contiguously, inside the body.
	if !strings.Contains(got, want.String()) {
		t.Error("RenderCollection does not contain the concatenation of RenderEvent outputs verbatim")
	}
}

func TestRenderEvent_AllDayValueDate(t *testing.T) {
	// A single-digit month/day exercises zero-padding in the YYYYMMDD format.
	lines := logicalLines(t, testICal().RenderEvent(Event{
		UID: "TSK-1@rela", Summary: "Due", Start: day(1, 3),
	}))
	if !containsLine(lines, "DTSTART;VALUE=DATE:20260103") {
		t.Errorf("missing all-day DTSTART; got lines: %v", lines)
	}
	// No timed DTSTART / trailing Z on the date.
	for _, l := range lines {
		if strings.HasPrefix(l, "DTSTART:") {
			t.Errorf("unexpected timed DTSTART: %q", l)
		}
	}
}

func TestRenderEvent_Timed(t *testing.T) {
	// A datetime-typed source yields a timed event: DTSTART is a UTC
	// date-time instant, NOT a VALUE=DATE line.
	lines := logicalLines(t, testICal().RenderEvent(Event{
		UID: "EVT-1@rela", Summary: "Standup", Start: dayTime(7, 13, 14, 30), Timed: true,
	}))
	if !containsLine(lines, "DTSTART:20260713T143000Z") {
		t.Errorf("missing timed DTSTART; got lines: %v", lines)
	}
	// The all-day form must be absent for a timed event.
	for _, l := range lines {
		if strings.HasPrefix(l, "DTSTART;VALUE=DATE:") {
			t.Errorf("unexpected all-day DTSTART on a timed event: %q", l)
		}
	}
}

func TestRenderEvent_TimedRange(t *testing.T) {
	// A datetime start+end yields timed DTSTART and DTEND (the exact end
	// instant, verbatim — no coercion).
	lines := logicalLines(t, testICal().RenderEvent(Event{
		UID: "EVT-2@rela", Summary: "Meeting",
		Start: dayTime(7, 13, 14, 0), End: dayTime(7, 13, 15, 0), Timed: true,
	}))
	if !containsLine(lines, "DTSTART:20260713T140000Z") {
		t.Errorf("missing timed DTSTART; got %v", lines)
	}
	if !containsLine(lines, "DTEND:20260713T150000Z") {
		t.Errorf("missing timed DTEND; got %v", lines)
	}
}

func TestRenderEvent_UIDAndDTSTAMP(t *testing.T) {
	lines := logicalLines(t, testICal().RenderEvent(Event{
		UID: "TSK-1@rela", Summary: "X", Start: day(7, 10),
	}))
	if !containsLine(lines, "UID:TSK-1@rela") {
		t.Error("missing/incorrect UID")
	}
	if !containsLine(lines, "DTSTAMP:20260704T123000Z") {
		t.Errorf("missing/incorrect DTSTAMP; got %v", lines)
	}
}

func TestRenderEvent_StableAcrossRenders(t *testing.T) {
	e := Event{UID: "TSK-1@rela", Summary: "X", Start: day(7, 10)}
	a := testICal().RenderEvent(e)
	b := testICal().RenderEvent(e)
	if !bytes.Equal(a, b) {
		t.Error("RenderEvent is not deterministic for a fixed clock")
	}
}

func TestRenderEvent_RRuleAndDTEND(t *testing.T) {
	lines := logicalLines(t, testICal().RenderEvent(Event{
		UID: "u", Summary: "Ranged recurring", Start: day(7, 10), End: day(7, 13),
		RRule: "FREQ=DAILY",
	}))
	if !containsLine(lines, "DTSTART;VALUE=DATE:20260710") {
		t.Error("missing DTSTART")
	}
	if !containsLine(lines, "DTEND;VALUE=DATE:20260713") {
		t.Errorf("missing/incorrect DTEND; got %v", lines)
	}
	if !containsLine(lines, "RRULE:FREQ=DAILY") {
		t.Errorf("missing RRULE; got %v", lines)
	}

	// Neither present when unset.
	plain := logicalLines(t, testICal().RenderEvent(Event{UID: "u", Summary: "Plain", Start: day(7, 10)}))
	for _, l := range plain {
		if strings.HasPrefix(l, "DTEND") || strings.HasPrefix(l, "RRULE") {
			t.Errorf("unexpected %q on a plain event", l)
		}
	}
}

func TestETag_SensitiveToRRuleAndEnd(t *testing.T) {
	ic := testICal()
	base := Event{UID: "u", Summary: "X", Start: day(7, 10)}
	if ic.ETag(base) == ic.ETag(Event{UID: "u", Summary: "X", Start: day(7, 10), RRule: "FREQ=DAILY"}) {
		t.Error("ETag unchanged after adding RRULE")
	}
	if ic.ETag(base) == ic.ETag(Event{UID: "u", Summary: "X", Start: day(7, 10), End: day(7, 12)}) {
		t.Error("ETag unchanged after adding DTEND")
	}
}

// TestRenderEvent_NoLineBreakInjection pins that a CR/LF smuggled into any field
// (here RRULE, which is not TEXT-escaped) cannot inject extra iCalendar lines:
// stripLineBreaks removes them at the writeLine choke point. Without the guard, a
// property-referenced RRULE value could inject a whole VALARM/VEVENT into the feed.
func TestRenderEvent_NoLineBreakInjection(t *testing.T) {
	evil := "FREQ=DAILY\r\nBEGIN:VALARM\r\nACTION:AUDIO\r\nTRIGGER:PT0S\r\nEND:VALARM"
	body := testICal().RenderEvent(Event{UID: "u", Summary: "X", Start: day(7, 10), RRule: evil})
	lines := logicalLines(t, body)
	// The whole rule must collapse onto one RRULE line; no injected components.
	if !containsLine(lines, "RRULE:FREQ=DAILYBEGIN:VALARMACTION:AUDIOTRIGGER:PT0SEND:VALARM") {
		t.Errorf("RRULE not collapsed to a single line; got: %v", lines)
	}
	// Exactly one VEVENT boundary, no injected VALARM.
	if countLine(lines, "BEGIN:VALARM") != 0 {
		t.Error("CRLF injection produced a VALARM")
	}
	// Also verify a summary with a raw newline can't break the line.
	sumLines := logicalLines(t, testICal().RenderEvent(Event{UID: "u", Summary: "a\r\nSUMMARY:injected", Start: day(7, 10)}))
	if countLine(sumLines, "SUMMARY:injected") != 0 {
		t.Error("newline in summary injected a second SUMMARY line")
	}
}

func TestRenderEvent_VALARM(t *testing.T) {
	withAlarm := logicalLines(t, testICal().RenderEvent(Event{
		UID: "TSK-1@rela", Summary: "X", Start: day(7, 10),
		Alarms: []Alarm{{Trigger: "-PT9H"}},
	}))
	for _, want := range []string{"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT9H", "DESCRIPTION:X", "END:VALARM"} {
		if !containsLine(withAlarm, want) {
			t.Errorf("VALARM missing %q; got %v", want, withAlarm)
		}
	}
	// No alarm → no VALARM.
	noAlarm := logicalLines(t, testICal().RenderEvent(Event{UID: "u", Summary: "X", Start: day(7, 10)}))
	if containsLine(noAlarm, "BEGIN:VALARM") {
		t.Error("VALARM emitted for an event with no alarm")
	}
}

func TestEscapeText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"comma", "a,b", `a\,b`},
		{"semicolon", "a;b", `a\;b`},
		{"backslash", `a\b`, `a\\b`},
		{"newline", "a\nb", `a\nb`},
		{"crlf collapses", "a\r\nb", `a\nb`},
		{"cr alone", "a\rb", `a\nb`},
		{"combined", `a,b;c\d`, `a\,b\;c\\d`},
		{"plain", "hello world", "hello world"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeText(tc.in); got != tc.want {
				t.Errorf("escapeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFoldLine_Boundary(t *testing.T) {
	// A 75-octet line is NOT folded (limit is inclusive).
	l75 := "X:" + strings.Repeat("a", 73) // 75 octets total
	if len(l75) != 75 {
		t.Fatalf("setup: len=%d", len(l75))
	}
	if got := foldLine(l75); strings.Contains(got, crlf) {
		t.Errorf("75-octet line was folded: %q", got)
	}
	// A 76-octet line IS folded exactly once.
	l76 := "X:" + strings.Repeat("a", 74)
	if len(l76) != 76 {
		t.Fatalf("setup: len=%d", len(l76))
	}
	folded := foldLine(l76)
	if !strings.Contains(folded, crlf+" ") {
		t.Errorf("76-octet line was not folded: %q", folded)
	}
	if unfold(folded) != l76 {
		t.Errorf("unfold(fold(x)) != x: got %q want %q", unfold(folded), l76)
	}
}

func TestFoldLine_MultibyteNotSplit(t *testing.T) {
	// A run of 3-byte runes (e.g. CJK) that crosses the 75-octet boundary must
	// fold on a rune boundary; unfolding must reproduce valid UTF-8.
	body := strings.Repeat("あ", 40) // 120 octets
	line := "SUMMARY:" + body
	folded := foldLine(line)
	restored := unfold(folded)
	if restored != line {
		t.Errorf("multibyte fold not reversible")
	}
	// Every folded segment must be valid UTF-8 (no split rune).
	for seg := range strings.SplitSeq(folded, crlf) {
		seg = strings.TrimPrefix(seg, " ")
		if !utf8.ValidString(seg) {
			t.Errorf("fold split a rune: segment %q is not valid UTF-8", seg)
		}
	}
}

func TestFoldLine_EscapedSummaryRoundTrips(t *testing.T) {
	// A long summary with special chars: escape then fold, then verify the
	// logical (unfolded) line is the escaped form.
	sum := strings.Repeat("a,b; ", 30)
	body := testICal().RenderEvent(Event{UID: "u", Summary: sum, Start: day(7, 10)})
	lines := logicalLines(t, body)
	want := "SUMMARY:" + escapeText(sum)
	if !containsLine(lines, want) {
		t.Errorf("folded+escaped SUMMARY did not round-trip.\n got: %v\nwant line: %q", lines, want)
	}
}

func TestETag_StableAndSensitive(t *testing.T) {
	ic := testICal()
	e := Event{UID: "TSK-1@rela", Summary: "X", Start: day(7, 10)}

	// Stable across renders and independent of the clock (DTSTAMP excluded).
	if ic.ETag(e) != (ICal{Now: fixedNow.Add(time.Hour)}).ETag(e) {
		t.Error("ETag changed when only the clock moved")
	}
	// Sensitive to content.
	e2 := e
	e2.Summary = "Y"
	if ic.ETag(e) == ic.ETag(e2) {
		t.Error("ETag did not change when summary changed")
	}
}

func TestCollectionTag_Sensitive(t *testing.T) {
	ic := testICal()
	base := Feed{Events: []Event{
		{UID: "a", Summary: "A", Start: day(7, 10)},
		{UID: "b", Summary: "B", Start: day(7, 11)},
	}}
	tag := ic.CollectionTag(base)

	// Adding an event changes the tag.
	more := base
	more.Events = append([]Event{}, base.Events...)
	more.Events = append(more.Events, Event{UID: "c", Summary: "C", Start: day(7, 12)})
	if ic.CollectionTag(more) == tag {
		t.Error("CollectionTag unchanged after adding an event")
	}
	// Modifying an event changes the tag.
	mod := Feed{Events: append([]Event{}, base.Events...)}
	mod.Events[0].Summary = "A2"
	if ic.CollectionTag(mod) == tag {
		t.Error("CollectionTag unchanged after modifying an event")
	}
	// Same content → same tag.
	if ic.CollectionTag(Feed{Events: append([]Event{}, base.Events...)}) != tag {
		t.Error("CollectionTag changed for identical content")
	}
}

func TestRenderCollection_EmptyFeed(t *testing.T) {
	lines := logicalLines(t, testICal().RenderCollection(Feed{Name: "empty"}))
	if lines[0] != "BEGIN:VCALENDAR" || lines[len(lines)-1] != "END:VCALENDAR" {
		t.Errorf("empty feed is not a well-formed VCALENDAR: %v", lines)
	}
	if countLine(lines, "BEGIN:VEVENT") != 0 {
		t.Error("empty feed contains a VEVENT")
	}
}

// --- helpers ---

func containsLine(lines []string, want string) bool { return countLine(lines, want) > 0 }

func countLine(lines []string, want string) int {
	n := 0
	for _, l := range lines {
		if l == want {
			n++
		}
	}
	return n
}
