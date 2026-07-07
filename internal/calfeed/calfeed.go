// Package calfeed models calendar feeds and serializes them to calendar
// interchange formats (iCalendar and JSON) as a service separate from the
// store.
//
// The package is a pure model→bytes layer: it holds no state, reads nothing
// from the store, and imports no storage or metamodel types. Callers build a
// [Feed] (typically by projecting entities elsewhere) and render it via [ICal]
// or [RenderJSON].
//
// The serializer is deliberately event-granular. [ICal.RenderEvent] renders one
// event and [ICal.RenderCollection] wraps it into a full VCALENDAR, so the same
// correctness (line folding, escaping, CRLF, UID, DTSTAMP, VALUE=DATE, VALARM)
// lives in one place and is reused by both the whole-feed ICS export and a
// future CalDAV server that fetches individual resources. [ICal.ETag] and
// [ICal.CollectionTag] provide the content tags CalDAV needs for conditional
// requests.
//
// Phase 1 emits all-day events only (DTSTART;VALUE=DATE). Timed events await a
// datetime property type in the metamodel.
package calfeed

import "time"

// Alarm is a display reminder attached to an [Event], rendered as a VALARM.
type Alarm struct {
	// Trigger is an RFC 5545 duration relative to the event start, e.g.
	// "-PT9H" for nine hours before. Required.
	Trigger string
	// Description is the alarm text; defaults to the event summary when empty.
	Description string
}

// Event is a single calendar entry. In Phase 1 every event is all-day: Start is
// interpreted as a date and rendered DTSTART;VALUE=DATE with no time component.
type Event struct {
	// UID is the globally-unique, stable identifier for this event across
	// refreshes (RFC 5545). Callers supply a stable value (e.g.
	// "<type>-<id>@rela"); a changing UID makes clients duplicate the event.
	UID string
	// Summary is the event title (SUMMARY). Required.
	Summary string
	// Description is optional longer text (DESCRIPTION).
	Description string
	// URL is an optional link back to the source (URL); typically a deep link
	// into the data-entry app.
	URL string
	// Start is the event's day. Its time-of-day is ignored in Phase 1 (all-day).
	Start time.Time
	// End, when non-zero, is the (exclusive) end day of an all-day range,
	// rendered DTEND;VALUE=DATE. Zero means a single-day event.
	End time.Time
	// RRule, when non-empty, is a bare RFC 5545 recurrence rule (without the
	// "RRULE:" prefix), e.g. "FREQ=DAILY". Rendered as an RRULE line so the
	// event recurs; an unbounded rule keeps the event visible until it leaves
	// the feed.
	RRule string
	// Alarms are optional reminders.
	Alarms []Alarm
}

// Feed is a named collection of events — one calendar.
type Feed struct {
	// Name is the human-readable calendar name (X-WR-CALNAME / displayname).
	Name string
	// Description is optional calendar-level text.
	Description string
	// Color is an optional calendar color (e.g. "#C2185B").
	Color string
	// Events are the calendar's entries, in caller-defined order.
	Events []Event
}
