package calfeed

import (
	"bytes"
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

func TestRenderJSON_TodoFeed(t *testing.T) {
	todo := Todo{
		UID: "task--TKT-1@rela", Summary: "Buy milk", URL: "/entity/task/TKT-1",
		Due:      time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		Priority: 5,
	}
	todo.Complete(time.Date(2026, 8, 9, 8, 14, 6, 0, time.UTC))

	out, err := RenderJSON(Feed{Name: "rela Tasks", Component: ComponentTodo, Todos: []Todo{todo}})
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonFeed
	if unmarshalErr := json.Unmarshal(out, &got); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v", unmarshalErr)
	}

	if len(got.Todos) != 1 {
		t.Fatalf("todos length = %d, want 1", len(got.Todos))
	}
	jt := got.Todos[0]
	if jt.UID != "task--TKT-1@rela" || jt.Summary != "Buy milk" {
		t.Errorf("todo identity wrong: %+v", jt)
	}
	if jt.Due != "2026-08-10" || !jt.AllDay {
		t.Errorf("all-day due wrong: due=%q allDay=%v", jt.Due, jt.AllDay)
	}
	if jt.Status != "COMPLETED" || jt.PercentComplete != 100 {
		t.Errorf("completion wrong: status=%q pct=%d", jt.Status, jt.PercentComplete)
	}
	if jt.Completed != "2026-08-09T08:14:06Z" {
		t.Errorf("completed = %q, want RFC3339 UTC", jt.Completed)
	}
	if jt.Priority != 5 {
		t.Errorf("priority = %d, want 5", jt.Priority)
	}

	// An event feed must not grow a "todos" key — the existing consumer
	// contract is unchanged.
	evOut, err := RenderJSON(Feed{Events: []Event{{UID: "e@rela", Summary: "E"}}})
	if err != nil {
		t.Fatalf("RenderJSON(events): %v", err)
	}
	if bytes.Contains(evOut, []byte(`"todos"`)) {
		t.Errorf("event feed must omit the todos key:\n%s", evOut)
	}
}

func TestRenderJSON_TodoTimedDue(t *testing.T) {
	todo := Todo{
		UID: "t@rela", Summary: "Standup",
		Due:   time.Date(2026, 8, 10, 9, 30, 0, 0, time.UTC),
		Timed: true,
	}
	out, err := RenderJSON(Feed{Component: ComponentTodo, Todos: []Todo{todo}})
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonFeed
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	jt := got.Todos[0]
	if jt.DueAt != "2026-08-10T09:30:00Z" || jt.Due != "" || jt.AllDay {
		t.Errorf("timed due wrong: dueAt=%q due=%q allDay=%v", jt.DueAt, jt.Due, jt.AllDay)
	}
	if jt.Status != "NEEDS-ACTION" {
		t.Errorf("status = %q, want the NEEDS-ACTION default", jt.Status)
	}
}

// TestRenderJSON_ComponentIsAlwaysExplicit guards against a consumer having to
// infer the feed kind from which key is present: an empty to-do feed and an
// empty event feed are otherwise byte-identical, and a consumer reading
// "events" would render an empty calendar for a to-do list that simply has
// nothing due.
func TestRenderJSON_ComponentIsAlwaysExplicit(t *testing.T) {
	tests := []struct {
		name string
		feed Feed
		want string
	}{
		{"empty event feed", Feed{}, "vevent"},
		{"empty todo feed", Feed{Component: ComponentTodo}, "vtodo"},
		{"populated todo feed", Feed{Component: ComponentTodo, Todos: []Todo{{UID: "u", Summary: "S"}}}, "vtodo"},
	}
	var rendered []string
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RenderJSON(tc.feed)
			if err != nil {
				t.Fatalf("RenderJSON: %v", err)
			}
			var got jsonFeed
			if unmarshalErr := json.Unmarshal(out, &got); unmarshalErr != nil {
				t.Fatalf("output is not valid JSON: %v", unmarshalErr)
			}
			if got.Component != tc.want {
				t.Errorf("component = %q, want %q", got.Component, tc.want)
			}
			rendered = append(rendered, string(out))
		})
	}
	// The two empty feeds must now be distinguishable.
	if rendered[0] == rendered[1] {
		t.Errorf("an empty todo feed is indistinguishable from an empty event feed:\n%s", rendered[0])
	}
}
