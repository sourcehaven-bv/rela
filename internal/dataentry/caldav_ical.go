package dataentry

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/emersion/go-ical"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
)

// This file is the ONLY place calfeed.Todo meets go-ical. Keeping the
// conversion here is what lets the rest of the codebase — and calfeed itself —
// stay free of the vendored parsing types (the "don't leak parsing types"
// rule): go-webdav's Backend traffics in *ical.Calendar, and that type stops
// at this boundary.

// icalFromTodo renders a to-do to the go-ical representation go-webdav returns
// to clients.
//
// It goes through calfeed's serializer rather than building the object
// property-by-property, so the CalDAV path inherits calfeed's normalization
// (the completion trio, range clamping, typed-value validation) instead of
// re-deriving it. Rendering to bytes and re-parsing is a small cost for one
// serializer of record.
func icalFromTodo(ic calfeed.ICal, td calfeed.Todo) (*ical.Calendar, error) {
	feed := calfeed.Feed{Component: calfeed.ComponentTodo, Todos: []calfeed.Todo{td}}
	dec := ical.NewDecoder(bytes.NewReader(ic.RenderCollection(feed)))
	cal, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("caldav: re-parse rendered todo: %w", err)
	}
	return cal, nil
}

// inboundTodo is a parsed client VTODO plus the set of properties the client
// actually SENT.
//
// Presence is tracked explicitly because calfeed.Todo cannot express it: its
// zero values are ambiguous, so "" could mean the client cleared a note or
// never mentioned one, and those must produce different writes. Guessing per
// field is how a mapper ends up erasing a property nobody touched — Apple omits
// DESCRIPTION whenever the note is empty, so an unconditional write blanks it
// on every sync.
type inboundTodo struct {
	Todo calfeed.Todo
	// sent holds the iCalendar property names present in the request.
	sent map[string]bool
}

// has reports whether the client sent a property.
func (i inboundTodo) has(prop string) bool { return i.sent[prop] }

// todoFromICal extracts the one VTODO a client PUT carries, recording which
// properties were present.
func todoFromICal(cal *ical.Calendar) (inboundTodo, error) {
	if cal == nil {
		return inboundTodo{}, errors.New("caldav: empty request body")
	}
	var comp *ical.Component
	for _, child := range cal.Children {
		if child.Name == ical.CompToDo {
			comp = child
			break
		}
	}
	if comp == nil {
		return inboundTodo{}, errors.New("caldav: request contains no VTODO")
	}

	td := calfeed.Todo{
		UID:         textProp(comp, ical.PropUID),
		Summary:     textProp(comp, ical.PropSummary),
		Description: textProp(comp, ical.PropDescription),
		URL:         textProp(comp, ical.PropURL),
		Location:    textProp(comp, ical.PropLocation),
		// RRULE is parsed but never written back: the collection's `rrule:` is
		// either a literal the operator set or a property rela owns, so a
		// client-edited recurrence has nowhere to land. Reading it keeps the
		// value visible to the mapper without implying a write path.
		RRule: rawProp(comp, ical.PropRecurrenceRule),
	}
	// CATEGORIES may REPEAT, and clients differ: rela emits one comma-separated
	// line, while Thunderbird sends a separate line per category (verified on
	// the wire). Reading only Props.Get would silently keep the first and drop
	// the rest — the user's other tags vanish on the next sync.
	for _, cats := range comp.Props.Values(ical.PropCategories) {
		values, err := cats.TextList()
		if err != nil {
			continue
		}
		td.Categories = append(td.Categories, values...)
	}
	if td.UID == "" {
		// The UID is the client's own identifier for the resource; without one
		// there is nothing to alias it by.
		return inboundTodo{}, errors.New("caldav: VTODO has no UID")
	}

	sent := map[string]bool{}
	for _, prop := range []string{
		ical.PropSummary, ical.PropDescription, ical.PropDue, ical.PropStatus,
		ical.PropCompleted, ical.PropPercentComplete, ical.PropPriority,
		ical.PropLocation, ical.PropCategories, ical.PropDateTimeStart,
	} {
		sent[prop] = comp.Props.Get(prop) != nil
	}

	if start := comp.Props.Get(ical.PropDateTimeStart); start != nil {
		when, timed, err := parseICalTime(start)
		if err != nil {
			return inboundTodo{}, fmt.Errorf("caldav: DTSTART: %w", err)
		}
		td.Start, td.Timed = when, timed
	}
	if due := comp.Props.Get(ical.PropDue); due != nil {
		when, timed, err := parseICalTime(due)
		if err != nil {
			return inboundTodo{}, fmt.Errorf("caldav: DUE: %w", err)
		}
		td.Due, td.Timed = when, timed
	}
	// Clamped ON INGEST, not just on render. calfeed normalizes these on the way
	// OUT, which protects the wire but not the store: an unclamped
	// `PRIORITY:2147483647` was written into the mapped entity property
	// verbatim, so every other consumer — the SPA, the CLI, exports, validation,
	// Lua — saw a value the metamodel would have rejected, while the render kept
	// clamping it and hiding the damage.
	//
	// Clamping rather than rejecting matches the outbound rule: a nonsense
	// priority degrades that one property, it does not fail the whole write (and
	// with it the completion tick the user actually meant).
	if n, err := strconv.Atoi(textProp(comp, ical.PropPriority)); err == nil {
		td.Priority = calfeed.ClampPriority(n)
	}
	if n, err := strconv.Atoi(textProp(comp, ical.PropPercentComplete)); err == nil {
		td.PercentComplete = calfeed.ClampPercentComplete(n)
	}
	// STATUS is an enumerated value: an unrecognized one is dropped rather than
	// written through, or a garbage string would become a status WRITE.
	if raw := textProp(comp, ical.PropStatus); raw != "" {
		switch calfeed.TodoStatus(raw) {
		case calfeed.TodoNeedsAction, calfeed.TodoCompleted,
			calfeed.TodoInProcess, calfeed.TodoCancelled:
			td.Status = calfeed.TodoStatus(raw)
		default:
			sent[ical.PropStatus] = false
		}
	}
	if done := comp.Props.Get(ical.PropCompleted); done != nil {
		when, _, err := parseICalTime(done)
		if err != nil {
			return inboundTodo{}, fmt.Errorf("caldav: COMPLETED: %w", err)
		}
		td.Completed = when
		// A COMPLETED timestamp is evidence the work finished, and RFC 5545
		// does not require STATUS alongside it. Promote so a client that sends
		// only the timestamp is not silently ignored: without this the write
		// succeeds, changes nothing, and the next sync renders
		// STATUS:NEEDS-ACTION back — the checkbox reverts under the user with
		// no error anywhere.
		//
		// This mirrors calfeed.Todo.normalized() ("a timestamp is the stronger
		// signal"), which the OUTBOUND path already applies. Only the promotion
		// arm belongs here: normalized() also DEMOTES a COMPLETED with no
		// timestamp, which would be wrong inbound — a client legitimately sends
		// STATUS:COMPLETED alone, and mapCompletion stamps the time itself.
		if td.Status == "" {
			td.Status = calfeed.TodoCompleted
			sent[ical.PropStatus] = true
		}
	}
	return inboundTodo{Todo: td, sent: sent}, nil
}

// parseICalTime reads a DATE or DATE-TIME property, reporting which it was.
//
// A client may send either form for DUE, and the distinction is what decides
// whether the stored value carries a time of day.
func parseICalTime(p *ical.Prop) (when time.Time, timed bool, err error) {
	if p.ValueType() == ical.ValueDate {
		d, dateErr := p.DateTime(time.UTC)
		return d, false, dateErr
	}
	t, dtErr := p.DateTime(time.UTC)
	return t, true, dtErr
}

// rawProp reads a property's UNESCAPED value, for structured-value properties
// (RRULE) whose semicolons and commas are separators rather than literal text.
func rawProp(c *ical.Component, name string) string {
	p := c.Props.Get(name)
	if p == nil {
		return ""
	}
	return p.Value
}

// textProp reads a property's raw text, or "" when absent.
func textProp(c *ical.Component, name string) string {
	p := c.Props.Get(name)
	if p == nil {
		return ""
	}
	v, err := p.Text()
	if err != nil {
		return p.Value
	}
	return v
}
