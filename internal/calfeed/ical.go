package calfeed

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// crlf is the RFC 5545 line separator; iCalendar mandates CRLF everywhere.
const crlf = "\r\n"

// icalFoldLimit is the RFC 5545 octet limit before a content line must be
// folded (75 octets, excluding the CRLF).
const icalFoldLimit = 75

// ICal renders [Feed]/[Event] values to iCalendar (RFC 5545) text.
//
// Now is injected rather than read from the clock so output is deterministic
// and testable; it becomes the DTSTAMP of every rendered event. ProdID is the
// PRODID identifier (defaults to a rela identifier when empty).
type ICal struct {
	ProdID string
	Now    time.Time
}

func (ic ICal) prodID() string {
	if ic.ProdID == "" {
		return "-//Sourcehaven//rela//EN"
	}
	return ic.ProdID
}

// RenderCollection renders the whole feed as one VCALENDAR object: the standard
// envelope wrapping one component per entry — VEVENT via [ICal.RenderEvent], or
// VTODO via [ICal.RenderTodo] when [Feed.Component] is [ComponentTodo]. The
// result is CRLF-terminated (including the final END:VCALENDAR line).
//
// A feed renders exactly one component kind; the slice that does not match
// Component is ignored rather than merged (see the [Feed] doc for why mixing
// breaks Apple clients).
func (ic ICal) RenderCollection(f Feed) []byte {
	var b strings.Builder
	writeLine(&b, "BEGIN:VCALENDAR")
	writeLine(&b, "VERSION:2.0")
	writeLine(&b, "PRODID:"+ic.prodID())
	writeLine(&b, "CALSCALE:GREGORIAN")
	if f.Name != "" {
		writeProp(&b, "X-WR-CALNAME", f.Name)
	}
	if f.Description != "" {
		writeProp(&b, "X-WR-CALDESC", f.Description)
	}
	if f.Color != "" {
		// COLOR is RFC 7986; harmless to clients that ignore it.
		writeProp(&b, "COLOR", f.Color)
	}
	if f.isTodo() {
		for _, t := range f.Todos {
			b.Write(ic.RenderTodo(t))
		}
	} else {
		for _, e := range f.Events {
			b.Write(ic.RenderEvent(e))
		}
	}
	writeLine(&b, "END:VCALENDAR")
	return []byte(b.String())
}

// RenderEvent renders one event as a VEVENT block (BEGIN:VEVENT…END:VEVENT),
// CRLF-terminated. It is the single source of per-event serialization; both the
// ICS feed and a future CalDAV per-resource GET use it. An all-day event emits
// DTSTART;VALUE=DATE; a timed event (Event.Timed) emits a UTC date-time DTSTART.
func (ic ICal) RenderEvent(e Event) []byte {
	var b strings.Builder
	writeLine(&b, "BEGIN:VEVENT")
	writeProp(&b, "UID", e.UID)
	writeLine(&b, "DTSTAMP:"+formatDateTimeUTC(ic.Now))
	// Timed events render UTC date-time instants; all-day events render a DATE
	// value. Only the FORMAT differs — Start/End are rendered verbatim.
	if e.Timed {
		writeLine(&b, "DTSTART:"+formatDateTimeUTC(e.Start))
		if !e.End.IsZero() {
			writeLine(&b, "DTEND:"+formatDateTimeUTC(e.End))
		}
	} else {
		writeLine(&b, "DTSTART;VALUE=DATE:"+formatDate(e.Start))
		if !e.End.IsZero() {
			writeLine(&b, "DTEND;VALUE=DATE:"+formatDate(e.End))
		}
	}
	if e.RRule != "" {
		writeLine(&b, "RRULE:"+e.RRule)
	}
	writeProp(&b, "SUMMARY", e.Summary)
	if e.Description != "" {
		writeProp(&b, "DESCRIPTION", e.Description)
	}
	if e.URL != "" {
		writeProp(&b, "URL", e.URL)
	}
	for _, a := range e.Alarms {
		desc := a.Description
		if desc == "" {
			desc = e.Summary
		}
		writeLine(&b, "BEGIN:VALARM")
		writeLine(&b, "ACTION:DISPLAY")
		writeProp(&b, "DESCRIPTION", desc)
		writeLine(&b, "TRIGGER:"+a.Trigger)
		writeLine(&b, "END:VALARM")
	}
	writeLine(&b, "END:VEVENT")
	return []byte(b.String())
}

// RenderTodo renders one to-do as a VTODO block (BEGIN:VTODO…END:VTODO),
// CRLF-terminated. It is the single source of per-to-do serialization, mirroring
// [ICal.RenderEvent]: both the whole-collection ICS render and a CalDAV
// per-resource GET go through it.
//
// A to-do with no Due emits no DUE property (a deadline-less to-do is legal).
// Property order follows RFC 5545's examples; Apple re-sorts on write-back
// anyway, so no consumer may diff on raw bytes.
func (ic ICal) RenderTodo(t Todo) []byte {
	// Normalize FIRST: the exported fields allow a half-completed or
	// out-of-range Todo, and this is the single chokepoint every render (and
	// therefore every ETag) passes through. See [Todo.normalized].
	t = t.normalized()

	var b strings.Builder
	writeLine(&b, "BEGIN:VTODO")
	writeProp(&b, "UID", t.UID)
	writeLine(&b, "DTSTAMP:"+formatDateTimeUTC(ic.Now))
	// DTSTART is when work begins, as against DUE's deadline. Shares Timed with
	// DUE: RFC 5545 3.8.2.4 requires the two to agree on value type, and a
	// client presents a to-do as all-day or timed as a whole.
	if !t.Start.IsZero() {
		if t.Timed {
			writeLine(&b, "DTSTART:"+formatDateTimeUTC(t.Start))
		} else {
			writeLine(&b, "DTSTART;VALUE=DATE:"+formatDate(t.Start))
		}
	}
	// DUE is optional; a to-do without one has no deadline. Timed selects the
	// format exactly as it does for an event's DTSTART.
	if !t.Due.IsZero() {
		if t.Timed {
			writeLine(&b, "DUE:"+formatDateTimeUTC(t.Due))
		} else {
			writeLine(&b, "DUE;VALUE=DATE:"+formatDate(t.Due))
		}
	}
	writeProp(&b, "SUMMARY", t.Summary)
	if t.Description != "" {
		writeProp(&b, "DESCRIPTION", t.Description)
	}
	if t.URL != "" {
		writeProp(&b, "URL", t.URL)
	}
	writeLine(&b, "STATUS:"+string(t.status()))
	// COMPLETED is RFC 5545 DATE-TIME in UTC, never a DATE — it is an instant,
	// not a day, regardless of whether DUE is all-day.
	if !t.Completed.IsZero() {
		writeLine(&b, "COMPLETED:"+formatDateTimeUTC(t.Completed))
	}
	// Emit PERCENT-COMPLETE only when it carries information: a pending to-do
	// at 0% is the default, so the property would be noise.
	if t.PercentComplete != 0 {
		writeLine(&b, fmt.Sprintf("PERCENT-COMPLETE:%d", t.PercentComplete))
	}
	if t.Priority != 0 {
		writeLine(&b, fmt.Sprintf("PRIORITY:%d", t.Priority))
	}
	if t.Location != "" {
		writeProp(&b, "LOCATION", t.Location)
	}
	// CATEGORIES is ONE property carrying a comma-separated list (RFC 5545
	// 3.8.1.2). Each value is escaped individually, so a comma inside a
	// category name cannot forge a list separator.
	if cats := nonEmpty(t.Categories); len(cats) > 0 {
		for i, c := range cats {
			cats[i] = escapeText(c)
		}
		writeLine(&b, "CATEGORIES:"+strings.Join(cats, ","))
	}
	// writeLine, NOT writeProp: an RRULE is structured value syntax whose
	// semicolons and commas are SEPARATORS. Escaping them (as writeProp does
	// for free text) produces "FREQ=WEEKLY\;BYDAY=MO" — a rule no client can
	// parse. Matches the VEVENT path, which makes the same choice.
	if t.RRule != "" {
		writeLine(&b, "RRULE:"+t.RRule)
	}
	// A relative TRIGGER on a VTODO is anchored to DTSTART or DUE (RFC 5545
	// §3.8.6.3). With neither, an alarm has no anchor at all — an
	// underspecified state clients resolve inconsistently. Emit alarms only
	// when there is something to anchor them to.
	if !t.Due.IsZero() || !t.Start.IsZero() {
		for _, a := range t.Alarms {
			// An unparseable trigger drops its whole VALARM: a silently
			// mis-timed reminder is worse than an absent one.
			if !validTrigger(a.Trigger) {
				continue
			}
			desc := a.Description
			if desc == "" {
				desc = t.Summary
			}
			writeLine(&b, "BEGIN:VALARM")
			writeLine(&b, "ACTION:DISPLAY")
			writeProp(&b, "DESCRIPTION", desc)
			writeLine(&b, "TRIGGER:"+a.Trigger)
			writeLine(&b, "END:VALARM")
		}
	}
	writeLine(&b, "END:VTODO")
	return []byte(b.String())
}

// status returns the STATUS to render, defaulting the zero value to
// NEEDS-ACTION so a caller need not set it for a plain pending to-do.
//
// STATUS is an enumerated value, not TEXT, so it is VALIDATED rather than
// escaped: anything outside the VTODO status set (RFC 5545 §3.8.1.11 —
// note VEVENT's set differs) falls back to NEEDS-ACTION. Escaping would be
// wrong here, since an escaped non-value is still not a legal STATUS, and it
// keeps caller-supplied content out of a line that is written unescaped.
func (t Todo) status() TodoStatus {
	switch t.Status {
	case TodoNeedsAction, TodoCompleted, TodoCancelled, TodoInProcess:
		return t.Status
	default:
		// Covers both the zero value and any unrecognized string.
		return TodoNeedsAction
	}
}

// icalDuration matches the RFC 5545 DURATION values usable as a VALARM TRIGGER
// (§3.3.6): an optional sign, "P", then a week form or a day/time form with at
// least one component. Deliberately a subset — enough for alarm offsets like
// "-PT9H", "-P1D", "PT15M" — and it excludes anything containing a separator
// that could alter the property's meaning.
var icalDuration = regexp.MustCompile(
	`^[+-]?P(?:\d+W|(?:\d+D)?(?:T(?:\d+H)?(?:\d+M)?(?:\d+S)?)?)$`)

// validTrigger reports whether an alarm trigger is a renderable RFC 5545
// duration.
//
// TRIGGER is a DURATION (or DATE-TIME), not TEXT, so it is VALIDATED rather
// than escaped — escaping it as text would corrupt a legal value, and passing
// it through raw would let ";" (the parameter separator) change what the
// property means. A trigger that fails this check drops its whole VALARM: a
// silently mis-timed alarm is worse than no alarm.
func validTrigger(s string) bool {
	// A bare "P"/"PT" carries no components and is not a duration.
	if !strings.ContainsFunc(s, func(r rune) bool { return r >= '0' && r <= '9' }) {
		return false
	}
	return icalDuration.MatchString(s)
}

// ETag returns a strong, stable content tag for one event: it changes if and
// only if the rendered event content changes. Used as the CalDAV per-resource
// ETag. Independent of [ICal.Now] (DTSTAMP is excluded) so re-rendering an
// unchanged event yields the same tag.
func (ic ICal) ETag(e Event) string {
	// Hash the event with a zero clock so DTSTAMP (which always moves) does not
	// perturb the tag; only real content changes do.
	sum := sha256.Sum256(ICal{ProdID: ic.ProdID}.RenderEvent(e))
	return `"` + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10]) + `"`
}

// TodoETag is [ICal.ETag] for a to-do: a strong content tag that changes if and
// only if the rendered VTODO content changes, and is independent of [ICal.Now].
func (ic ICal) TodoETag(t Todo) string {
	sum := sha256.Sum256(ICal{ProdID: ic.ProdID}.RenderTodo(t))
	return `"` + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10]) + `"`
}

// CollectionTag returns a tag over the whole feed that changes if and only if
// any entry's content changes (added, removed, or modified). Used as the CalDAV
// collection ctag, for both component kinds.
//
// It must be derived from CONTENT, not from a change counter: store events are
// dropped when a subscriber's buffer fills (see store.Watcher), and the
// filesystem backend has no monotonic sequence — a counter-derived ctag would
// silently skip a change and leave a client permanently stale.
func (ic ICal) CollectionTag(f Feed) string {
	h := sha256.New()
	// Length-prefix each entry tag to avoid boundary ambiguities between
	// adjacent entries. Only the slice matching Component contributes, mirroring
	// what RenderCollection actually emits.
	if f.isTodo() {
		for _, t := range f.Todos {
			et := ic.TodoETag(t)
			fmt.Fprintf(h, "%d:%s", len(et), et)
		}
	} else {
		for _, e := range f.Events {
			et := ic.ETag(e)
			fmt.Fprintf(h, "%d:%s", len(et), et)
		}
	}
	sum := h.Sum(nil)
	return `"` + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10]) + `"`
}

// nonEmpty returns the non-blank entries of xs, in order, as a fresh slice.
// A blank category would render as an empty list element, which strict parsers
// reject.
func nonEmpty(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			out = append(out, x)
		}
	}
	return out
}

// writeProp writes "NAME:VALUE" with the value escaped per RFC 5545 and the
// whole line folded to the octet limit.
func writeProp(b *strings.Builder, name, value string) {
	writeLine(b, name+":"+escapeText(value))
}

// writeLine folds a single content line to the RFC 5545 octet limit and
// terminates it with CRLF. The line must already be escaped.
//
// Any raw CR or LF in the input is stripped first: folding is the only
// legitimate source of CRLF in the output, so a stray line break in content is
// always a bug or an injection attempt (e.g. a CRLF-bearing property value
// smuggling extra iCalendar lines). Stripping here makes "no unfolded line
// breaks" a structural invariant rather than a per-field responsibility.
func writeLine(b *strings.Builder, line string) {
	b.WriteString(foldLine(stripLineBreaks(line)))
	b.WriteString(crlf)
}

// stripLineBreaks removes raw CR and LF bytes so they cannot terminate a line
// prematurely. TEXT values render newlines as the "\n" escape (see escapeText),
// so any surviving raw break is unwanted.
func stripLineBreaks(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// escapeText escapes a TEXT value per RFC 5545 §3.3.11: backslash, semicolon,
// comma, and newlines. (A literal CR is normalized into the \n escape.)
func escapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case ';':
			b.WriteString(`\;`)
		case ',':
			b.WriteString(`\,`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			// Collapse CRLF/CR into a single \n; skip a following LF.
			b.WriteString(`\n`)
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// foldLine folds a content line longer than icalFoldLimit octets by inserting
// CRLF followed by a single space, per RFC 5545 §3.1. Folding counts octets,
// never splitting a multi-byte UTF-8 sequence: a continuation is emitted only
// at a rune boundary.
func foldLine(line string) string {
	if len(line) <= icalFoldLimit {
		return line
	}
	var b strings.Builder
	// First segment: up to icalFoldLimit octets on a rune boundary.
	cut := runeBoundedCut(line, icalFoldLimit)
	b.WriteString(line[:cut])
	rest := line[cut:]
	// Continuation segments start with a space, which counts toward the limit,
	// so each carries up to icalFoldLimit-1 octets of content.
	for rest != "" {
		b.WriteString(crlf)
		b.WriteByte(' ')
		cut = runeBoundedCut(rest, icalFoldLimit-1)
		b.WriteString(rest[:cut])
		rest = rest[cut:]
	}
	return b.String()
}

// runeBoundedCut returns the largest byte length <= limit that ends on a UTF-8
// rune boundary within s. It never returns 0 for non-empty s (a single rune
// longer than the limit is emitted whole rather than split).
func runeBoundedCut(s string, limit int) int {
	if len(s) <= limit {
		return len(s)
	}
	cut := limit
	// Back up while the byte at cut is a UTF-8 continuation byte.
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut == 0 {
		// The first rune alone exceeds the limit; emit it whole to make progress.
		_, size := utf8.DecodeRuneInString(s)
		return size
	}
	return cut
}

// formatDate renders a date as YYYYMMDD for DTSTART;VALUE=DATE (all-day).
func formatDate(t time.Time) string {
	return t.Format("20060102")
}

// formatDateTimeUTC renders a UTC timestamp as YYYYMMDDTHHMMSSZ, used for
// DTSTAMP and for a timed event's DTSTART/DTEND.
func formatDateTimeUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
