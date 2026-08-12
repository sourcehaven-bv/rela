package calfeed

import (
	"encoding/json"
	"time"
)

// jsonFeed / jsonEvent / jsonAlarm are the stable wire shapes for the JSON
// rendering. They are defined separately from the domain types so the JSON
// contract (consumed by menubar/notification glue) does not drift with internal
// field changes.
//
// All-day events populate date/endDate (plain YYYY-MM-DD) with allDay=true;
// timed events populate start/end (RFC3339 UTC instants) with allDay=false.
// The two field pairs are mutually exclusive so an existing date-only consumer
// keeps working (it simply sees empty date fields, and allDay=false, for a
// timed event).
type jsonFeed struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	// Component names the entry kind this feed carries: "vevent" or "vtodo".
	// ALWAYS emitted, so a consumer never has to infer the kind from which key
	// is present — an empty to-do feed and an empty event feed are otherwise
	// byte-identical, and a consumer reading "events" would show an empty
	// calendar for a to-do list that simply has nothing due.
	Component string      `json:"component"`
	Events    []jsonEvent `json:"events"`
	// Todos is populated instead of Events for a VTODO feed. It is omitted
	// entirely for an event feed so the existing consumer contract is unchanged.
	Todos []jsonTodo `json:"todos,omitempty"`
}

// jsonTodo is the wire shape for a VTODO entry. Kept separate from jsonEvent
// for the same reason [Todo] is separate from [Event]: the fields genuinely
// differ (due vs start/end, plus the completion trio).
type jsonTodo struct {
	UID             string      `json:"uid"`
	Summary         string      `json:"summary"`
	Description     string      `json:"description,omitempty"`
	URL             string      `json:"url,omitempty"`
	Due             string      `json:"due,omitempty"`   // YYYY-MM-DD (all-day)
	DueAt           string      `json:"dueAt,omitempty"` // RFC3339 UTC (timed)
	AllDay          bool        `json:"allDay"`
	Status          string      `json:"status"`
	Completed       string      `json:"completed,omitempty"` // RFC3339 UTC
	PercentComplete int         `json:"percentComplete,omitempty"`
	Priority        int         `json:"priority,omitempty"`
	Alarms          []jsonAlarm `json:"alarms,omitempty"`
}

type jsonEvent struct {
	UID         string      `json:"uid"`
	Summary     string      `json:"summary"`
	Description string      `json:"description,omitempty"`
	URL         string      `json:"url,omitempty"`
	Date        string      `json:"date,omitempty"`    // YYYY-MM-DD (all-day only)
	EndDate     string      `json:"endDate,omitempty"` // YYYY-MM-DD, all-day range
	Start       string      `json:"start,omitempty"`   // RFC3339 UTC (timed only)
	End         string      `json:"end,omitempty"`     // RFC3339 UTC, timed range
	AllDay      bool        `json:"allDay"`
	RRule       string      `json:"rrule,omitempty"` // bare RFC 5545 recurrence rule
	Alarms      []jsonAlarm `json:"alarms,omitempty"`
}

type jsonAlarm struct {
	Trigger     string `json:"trigger"`
	Description string `json:"description,omitempty"`
}

// RenderJSON renders the feed as JSON: a stable, self-describing shape for
// non-calendar consumers (e.g. a menubar plugin or notification script). It is
// collection-only — there is no per-entry JSON rendering.
//
// A VTODO feed populates "todos" and leaves "events" an empty array, mirroring
// [ICal.RenderCollection]: only the slice matching [Feed.Component] is emitted.
func RenderJSON(f Feed) ([]byte, error) {
	out := jsonFeed{
		Name:        f.Name,
		Description: f.Description,
		Color:       f.Color,
		Component:   "vevent",
		Events:      make([]jsonEvent, 0, len(f.Events)),
	}
	if f.isTodo() {
		out.Component = "vtodo"
		out.Todos = make([]jsonTodo, 0, len(f.Todos))
		for _, raw := range f.Todos {
			// Normalize exactly as the iCalendar path does, so the two
			// renderings can never disagree about the same Todo.
			t := raw.normalized()
			jt := jsonTodo{
				UID:             t.UID,
				Summary:         t.Summary,
				Description:     t.Description,
				URL:             t.URL,
				AllDay:          !t.Timed,
				Status:          string(t.status()),
				PercentComplete: t.PercentComplete,
				Priority:        t.Priority,
			}
			if !t.Due.IsZero() {
				if t.Timed {
					jt.DueAt = t.Due.UTC().Format(time.RFC3339)
				} else {
					jt.Due = t.Due.Format(time.DateOnly)
				}
			}
			if !t.Completed.IsZero() {
				jt.Completed = t.Completed.UTC().Format(time.RFC3339)
			}
			for _, a := range t.Alarms {
				jt.Alarms = append(jt.Alarms, jsonAlarm(a))
			}
			out.Todos = append(out.Todos, jt)
		}
		return json.MarshalIndent(out, "", "  ")
	}
	for _, e := range f.Events {
		je := jsonEvent{
			UID:         e.UID,
			Summary:     e.Summary,
			Description: e.Description,
			URL:         e.URL,
			AllDay:      !e.Timed,
			RRule:       e.RRule,
		}
		if e.Timed {
			je.Start = e.Start.UTC().Format(time.RFC3339)
			if !e.End.IsZero() {
				je.End = e.End.UTC().Format(time.RFC3339)
			}
		} else {
			je.Date = e.Start.Format(time.DateOnly)
			if !e.End.IsZero() {
				je.EndDate = e.End.Format(time.DateOnly)
			}
		}
		for _, a := range e.Alarms {
			je.Alarms = append(je.Alarms, jsonAlarm(a))
		}
		out.Events = append(out.Events, je)
	}
	return json.MarshalIndent(out, "", "  ")
}
