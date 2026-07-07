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
		t.Error("allDay should be true in Phase 1")
	}
	if len(e.Alarms) != 1 || e.Alarms[0].Trigger != "-PT9H" {
		t.Errorf("alarms wrong: %+v", e.Alarms)
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
