package calfeed

import (
	"encoding/json"
	"time"
)

// jsonFeed / jsonEvent / jsonAlarm are the stable wire shapes for the JSON
// rendering. They are defined separately from the domain types so the JSON
// contract (consumed by menubar/notification glue) does not drift with internal
// field changes, and so dates render as plain YYYY-MM-DD rather than RFC3339
// timestamps (Phase 1 events are all-day).
type jsonFeed struct {
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Color       string      `json:"color,omitempty"`
	Events      []jsonEvent `json:"events"`
}

type jsonEvent struct {
	UID         string      `json:"uid"`
	Summary     string      `json:"summary"`
	Description string      `json:"description,omitempty"`
	URL         string      `json:"url,omitempty"`
	Date        string      `json:"date"`              // YYYY-MM-DD (all-day)
	EndDate     string      `json:"endDate,omitempty"` // YYYY-MM-DD, for a range
	AllDay      bool        `json:"allDay"`            // always true in Phase 1
	RRule       string      `json:"rrule,omitempty"`   // bare RFC 5545 recurrence rule
	Alarms      []jsonAlarm `json:"alarms,omitempty"`
}

type jsonAlarm struct {
	Trigger     string `json:"trigger"`
	Description string `json:"description,omitempty"`
}

// RenderJSON renders the feed as JSON: a stable, self-describing shape for
// non-calendar consumers (e.g. a menubar plugin or notification script). It is
// collection-only — there is no per-event JSON rendering.
func RenderJSON(f Feed) ([]byte, error) {
	out := jsonFeed{
		Name:        f.Name,
		Description: f.Description,
		Color:       f.Color,
		Events:      make([]jsonEvent, 0, len(f.Events)),
	}
	for _, e := range f.Events {
		je := jsonEvent{
			UID:         e.UID,
			Summary:     e.Summary,
			Description: e.Description,
			URL:         e.URL,
			Date:        e.Start.Format(time.DateOnly),
			AllDay:      true,
			RRule:       e.RRule,
		}
		if !e.End.IsZero() {
			je.EndDate = e.End.Format(time.DateOnly)
		}
		for _, a := range e.Alarms {
			je.Alarms = append(je.Alarms, jsonAlarm(a))
		}
		out.Events = append(out.Events, je)
	}
	return json.MarshalIndent(out, "", "  ")
}
