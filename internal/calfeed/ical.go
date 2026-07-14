package calfeed

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
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
// envelope wrapping one VEVENT per event via [ICal.RenderEvent]. The result is
// CRLF-terminated (including the final END:VCALENDAR line).
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
	for _, e := range f.Events {
		b.Write(ic.RenderEvent(e))
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

// CollectionTag returns a tag over the whole feed that changes if and only if
// any event's content changes (added, removed, or modified). Used as the CalDAV
// collection ctag.
func (ic ICal) CollectionTag(f Feed) string {
	h := sha256.New()
	for _, e := range f.Events {
		// Include the per-event tag so any content change propagates; length-
		// prefix to avoid boundary ambiguities between adjacent events.
		et := ic.ETag(e)
		fmt.Fprintf(h, "%d:%s", len(et), et)
	}
	sum := h.Sum(nil)
	return `"` + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10]) + `"`
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

// formatDateTimeUTC renders a UTC timestamp as YYYYMMDDTHHMMSSZ for DTSTAMP.
func formatDateTimeUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
