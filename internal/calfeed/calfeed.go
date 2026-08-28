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
// Events are all-day by default (DTSTART;VALUE=DATE); an [Event] with Timed set
// renders as a UTC date-time instant (a datetime-typed feed source drives this).
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

// Event is a single calendar entry. By default an event is all-day: Start is
// interpreted as a date and rendered DTSTART;VALUE=DATE with no time component.
// Set Timed to render a time-bearing instant instead (DTSTART:...Z).
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
	// Start is the event's start. When Timed is false it is interpreted as a
	// day (time-of-day ignored); when Timed is true it is the exact UTC
	// instant of DTSTART.
	Start time.Time
	// End, when non-zero, is the event's end, rendered verbatim as the
	// RFC 5545 DTEND value (exclusive): a DATE value (next day) for an all-day
	// range, or the exact end instant for a timed event. Zero means a
	// single-day / no-DTEND event. Its all-day-vs-timed format follows Timed,
	// exactly like Start — the two must be the same kind (enforced upstream by
	// feed-source validation).
	End time.Time
	// Timed selects a time-bearing event (DTSTART/DTEND as UTC date-time
	// instants, YYYYMMDDTHHMMSSZ) over the default all-day form
	// (DTSTART;VALUE=DATE). The zero value is all-day, so existing all-day
	// callers need no change.
	Timed bool
	// RRule, when non-empty, is a bare RFC 5545 recurrence rule (without the
	// "RRULE:" prefix), e.g. "FREQ=DAILY". Rendered as an RRULE line so the
	// event recurs; an unbounded rule keeps the event visible until it leaves
	// the feed.
	RRule string
	// Alarms are optional reminders.
	Alarms []Alarm
}

// TodoStatus is a [Todo]'s completion state, rendered as the RFC 5545 STATUS
// property of a VTODO. The zero value is [TodoNeedsAction].
type TodoStatus string

// VTODO status values (RFC 5545 §3.8.1.11). This is the complete set for a
// VTODO and differs from VEVENT's — [ICal.RenderTodo] validates against it and
// falls back to NEEDS-ACTION for anything else, so the set is load-bearing
// rather than advisory.
const (
	TodoNeedsAction TodoStatus = "NEEDS-ACTION"
	TodoCompleted   TodoStatus = "COMPLETED"
	TodoInProcess   TodoStatus = "IN-PROCESS"
	TodoCancelled   TodoStatus = "CANCELLED"
)

// Todo is a single to-do entry, rendered as a VTODO.
//
// It is a SEPARATE type from [Event] rather than a set of extra fields on it:
// a VTODO's time anchor is DUE (not DTSTART) and its completion trio has no
// VEVENT meaning, so sharing one struct would dangle todo-only fields on every
// calendar event. Keeping them apart also makes "a collection is VEVENT-only or
// VTODO-only" — which Apple requires, see [Feed.Component] — structural rather
// than a runtime check.
type Todo struct {
	// UID is the globally-unique, stable identifier across refreshes
	// (RFC 5545). A changing UID makes clients duplicate the entry.
	UID string
	// Summary is the to-do title (SUMMARY). Required.
	Summary string
	// Description is optional longer text (DESCRIPTION).
	Description string
	// URL is an optional link back to the source (URL); typically a deep link
	// into the data-entry app.
	URL string
	// Due is the to-do's due date. Zero means no DUE property — a to-do
	// without a deadline is legal and common. When Timed is false it is
	// interpreted as a day (DUE;VALUE=DATE); when true it is an exact UTC
	// instant.
	Due time.Time
	// Timed selects a time-bearing DUE (YYYYMMDDTHHMMSSZ) over the default
	// all-day form. The zero value is all-day, matching [Event.Timed].
	Timed bool
	// Status is the completion state (STATUS). The zero value renders as
	// NEEDS-ACTION.
	Status TodoStatus
	// Completed is the completion timestamp (COMPLETED), always rendered as a
	// UTC date-time per RFC 5545. Zero means the property is omitted.
	//
	// This is load-bearing beyond display: RFC 4791 §7.8.9's canonical
	// "pending to-dos" query filters on COMPLETED being ABSENT, so a completed
	// to-do that omits it stays visible in clients that use that filter.
	// [Todo.Complete] keeps the trio consistent.
	Completed time.Time
	// PercentComplete is the PERCENT-COMPLETE value (0-100). It is only
	// rendered when Status is COMPLETED or the value is non-zero, so a plain
	// pending to-do emits no property.
	PercentComplete int
	// Priority is the RFC 5545 PRIORITY (1 = highest, 9 = lowest; 0 =
	// undefined and omitted).
	Priority int
	// Location is an optional free-text place (LOCATION).
	Location string
	// Categories are optional tags (CATEGORIES), emitted as one
	// comma-separated line. Empty entries are dropped.
	Categories []string
	// Start is when work begins (DTSTART), as against Due's deadline. Zero
	// omits the property. Shares Timed with Due: a to-do is all-day or timed as
	// a whole, which is what clients present.
	Start time.Time
	// RRule is an optional recurrence rule body (no "RRULE:" prefix), e.g.
	// "FREQ=WEEKLY". Mirrors [Event.RRule].
	RRule string
	// Alarms are optional reminders.
	Alarms []Alarm
}

// Complete marks the to-do done, setting the whole completion trio
// (STATUS:COMPLETED, COMPLETED, PERCENT-COMPLETE:100) in one step.
//
// It is a convenience, NOT the guarantee: the three fields stay exported and
// individually settable, so a caller can still construct a half-completed
// Todo. The actual guarantee is enforced downstream — [ICal.RenderTodo]
// normalizes the trio (see Todo.normalized), so a half-set value can never
// reach a client. Prefer this method anyway; it states the intent at the call
// site.
func (t *Todo) Complete(at time.Time) {
	t.Status = TodoCompleted
	t.Completed = at
	t.PercentComplete = 100
}

// normalized returns a copy of the to-do with self-consistent, in-range values,
// so a caller cannot emit iCalendar that is internally contradictory or that a
// strict client refuses to parse. Applied by [ICal.RenderTodo] and therefore by
// [ICal.TodoETag] too, so the tag reflects what a client actually receives.
//
// Three normalizations, each fixing a state that is reachable through the
// exported fields:
//
//  1. COMPLETION CONSISTENCY. RFC 4791 §7.8.9's canonical "pending to-dos"
//     query filters on COMPLETED being ABSENT, while a UI reads STATUS — so a
//     to-do carrying only one of them reads as done in one client and pending
//     in another. Either half implies the whole: a COMPLETED timestamp forces
//     STATUS:COMPLETED, and STATUS:COMPLETED without a timestamp is demoted
//     rather than invented, since fabricating a completion time would be a lie
//     about when the work finished.
//  2. PRIORITY is clamped to the RFC 5545 range 0-9. An out-of-range value is
//     a parse error in strict clients, and because entries share one VCALENDAR
//     body, one bad value can make the WHOLE collection unreadable.
//  3. PERCENT-COMPLETE is clamped to 0-100, for the same reason.
//
// Clamping rather than rejecting is deliberate: a nonsense priority should
// degrade that one property, never fail the render for every other entry.
func (t Todo) normalized() Todo {
	switch {
	case !t.Completed.IsZero():
		// A timestamp is the stronger signal — it is evidence the work
		// finished. Promote status to match.
		t.Status = TodoCompleted
		if t.PercentComplete == 0 {
			t.PercentComplete = 100
		}
	case t.Status == TodoCompleted:
		// Claimed done with no timestamp. Demote to in-progress rather than
		// invent a completion time; the caller should use [Todo.Complete].
		t.Status = TodoNeedsAction
	}
	t.Priority = clampInt(t.Priority, minPriority, maxPriority)
	t.PercentComplete = clampInt(t.PercentComplete, minPercentComplete, maxPercentComplete)
	return t
}

// RFC 5545 value ranges. PRIORITY (§3.8.1.9) is 0-9 with 0 meaning undefined;
// PERCENT-COMPLETE (§3.8.1.8) is 0-100. Out-of-range values are a parse error
// in strict clients, and because entries share one VCALENDAR body a single bad
// value can make an entire collection unreadable.
const (
	minPriority        = 0
	maxPriority        = 9
	minPercentComplete = 0
	maxPercentComplete = 100
)

// ClampPriority bounds an RFC 5545 PRIORITY to 0-9.
//
// Exported for the INBOUND direction. Todo.normalized clamps on render, which
// protects the wire but not the store: a client PUT carrying
// `PRIORITY:2147483647` used to be written into an entity property verbatim,
// and every other consumer of that property — the SPA, the CLI, exports,
// validation, Lua — then saw a value the metamodel would have rejected. The
// render stayed safe and hid the damage.
//
// Ingest and render must share one range definition, or they drift and the
// asymmetry comes back.
func ClampPriority(v int) int { return clampInt(v, minPriority, maxPriority) }

// ClampPercentComplete bounds an RFC 5545 PERCENT-COMPLETE to 0-100, for the
// same reason as [ClampPriority].
func ClampPercentComplete(v int) int { return clampInt(v, minPercentComplete, maxPercentComplete) }

// clampInt bounds v to [lo, hi].
//
// the bound comes from the named RFC constants rather than a magic zero, and a
// future range that does not start at 0 needs no signature change.
//
//nolint:unparam // lo is 0 for both current callers; keeping it explicit means
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Component is the calendar component a [Feed] carries. A feed holds exactly
// one kind.
type Component string

// Feed component kinds.
const (
	// ComponentEvent is the default (zero value): a feed of VEVENTs.
	ComponentEvent Component = ""
	// ComponentTodo is a feed of VTODOs.
	ComponentTodo Component = "VTODO"
)

// Feed is a named collection of entries — one calendar.
//
// A feed is EITHER a VEVENT feed (Component empty, Events populated) or a VTODO
// feed (Component VTODO, Todos populated), never both. This is not a stylistic
// choice: Apple's clients segregate by component set — Reminders binds only to a
// collection advertising supported-calendar-component-set: VTODO, and
// Calendar.app creates its own separate VEVENT collection — so a mixed
// collection is invisible to one of them.
type Feed struct {
	// Name is the human-readable calendar name (X-WR-CALNAME / displayname).
	Name string
	// Description is optional calendar-level text.
	Description string
	// Color is an optional calendar color (e.g. "#C2185B").
	Color string
	// Component selects which slice below is rendered. The zero value renders
	// Events, so existing callers need no change.
	Component Component
	// Events are the calendar's entries when Component is [ComponentEvent].
	Events []Event
	// Todos are the calendar's entries when Component is [ComponentTodo].
	Todos []Todo
}

// isTodo reports whether the feed renders its Todos rather than its Events.
//
// The single place the component decision is made. Every consumer
// ([ICal.RenderCollection], [ICal.CollectionTag], [RenderJSON]) branches on
// this rather than comparing Component itself, so the three cannot drift and a
// fourth consumer inherits the answer.
func (f Feed) isTodo() bool { return f.Component == ComponentTodo }
