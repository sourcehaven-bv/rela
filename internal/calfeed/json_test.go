package calfeed

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRenderJSON_RoundTrips(t *testing.T) {
	f := Feed{
		Name:  "PIM tasks",
		Color: "#C2185B",
		Events: []Event{
			{
				UID: "TSK-1@rela", Summary: "Renew passport", URL: "/entity/task/TSK-1",
				Start:  time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
				Alarms: []Alarm{{Trigger: "-PT9H", Description: "reminder"}},
			},
		},
	}
	out, err := RenderJSON(f)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonFeed
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Name != "PIM tasks" || got.Color != "#C2185B" {
		t.Errorf("feed meta wrong: %+v", got)
	}
	if len(got.Events) != 1 {
		t.Fatalf("event count = %d, want 1", len(got.Events))
	}
	e := got.Events[0]
	if e.UID != "TSK-1@rela" || e.Summary != "Renew passport" || e.URL != "/entity/task/TSK-1" {
		t.Errorf("event fields wrong: %+v", e)
	}
	if e.Date != "2026-07-10" {
		t.Errorf("date = %q, want 2026-07-10 (all-day, no time)", e.Date)
	}
	if !e.AllDay {
		t.Error("allDay should be true for an all-day event")
	}
	if e.Start != "" || e.End != "" {
		t.Errorf("all-day event must not populate start/end; got start=%q end=%q", e.Start, e.End)
	}
	if len(e.Alarms) != 1 || e.Alarms[0].Trigger != "-PT9H" {
		t.Errorf("alarms wrong: %+v", e.Alarms)
	}
}

func TestRenderJSON_TimedEvent(t *testing.T) {
	// A timed event populates start/end (RFC3339) with allDay=false, and
	// leaves the date-only date/endDate fields empty (backward-compatible:
	// existing date-only consumers just see no date + allDay=false).
	f := Feed{Events: []Event{{
		UID: "EVT-1@rela", Summary: "Meeting",
		Start: time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
		Timed: true,
	}}}
	out, err := RenderJSON(f)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonFeed
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	e := got.Events[0]
	if e.AllDay {
		t.Error("timed event must have allDay=false")
	}
	if e.Start != "2026-07-13T14:00:00Z" {
		t.Errorf("start = %q, want 2026-07-13T14:00:00Z", e.Start)
	}
	if e.End != "2026-07-13T15:00:00Z" {
		t.Errorf("end = %q, want 2026-07-13T15:00:00Z", e.End)
	}
	if e.Date != "" || e.EndDate != "" {
		t.Errorf("timed event must not populate date/endDate; got date=%q endDate=%q", e.Date, e.EndDate)
	}
}

func TestRenderJSON_EmptyFeedHasEventsArray(t *testing.T) {
	out, err := RenderJSON(Feed{Name: "empty"})
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	// events must serialize as [] (not null) so consumers can iterate.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if string(raw["events"]) != "[]" {
		t.Errorf("empty feed events = %s, want []", raw["events"])
	}
}
