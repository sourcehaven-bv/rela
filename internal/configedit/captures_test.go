package configedit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const captureDataEntry = `app:
  name: Tasks
lists:
  tasks:
    entity_type: task
  others:
    entity_type: task
dashboard:
  cards:
    - title: Open tasks
      list: tasks
      display: count
    - title: Other tasks
      list: others
      display: count
`

const captureSchedules = `tasks:
  - name: open-tasks
    every: day
    run_as: system:metrics
    capture:
      card: Open tasks
      record: measurement
      owner: MET-open
      field: value
`

func newCaptureHarness(t *testing.T, schedules string) *harness {
	t.Helper()
	h := newHarness(t, counts{}, nil)
	h.files["data-entry.yaml"] = []byte(captureDataEntry)
	if schedules != "" {
		h.files[schedulesFile] = []byte(schedules)
	}
	return h
}

// cardAt returns the dashboard card at index i of a data-entry tree.
func cardAt(de *Map, i int) *Map {
	dash, _ := de.Get("dashboard")
	cards, _ := dash.(*Map).Get("cards")
	return cards.([]any)[i].(*Map)
}

// dataEntryDraft edits the data-entry tree with fn and wraps it in a Draft.
func (h *harness) dataEntryDraft(t *testing.T, fn func(de *Map)) *Draft {
	t.Helper()
	snap, err := h.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	de := snap.DataEntry.(*Map)
	fn(de)
	raw, err := json.Marshal(de)
	if err != nil {
		t.Fatal(err)
	}
	return &Draft{BaseVersion: snap.Version, DataEntry: raw}
}

func TestSave_CapturedCard(t *testing.T) {
	tests := []struct {
		name      string
		schedules string
		edit      func(*Map)
		refused   bool
	}{
		{"a captured card's display", captureSchedules,
			func(de *Map) { cardAt(de, 0).Set("display", "sum"); cardAt(de, 0).Set("property", "title") }, true},
		{"its title moved to another card", captureSchedules,
			func(de *Map) { cardAt(de, 0).Set("title", "Was open"); cardAt(de, 1).Set("title", "Open tasks") }, true},
		{"a second card with its title", captureSchedules,
			func(de *Map) { cardAt(de, 1).Set("title", "Open tasks") }, true},
		{"another card", captureSchedules,
			func(de *Map) { cardAt(de, 1).Set("display", "ratio") }, false},
		{"no schedules.yaml", "",
			func(de *Map) { cardAt(de, 0).Set("display", "ratio") }, false},
		{"schedules.yaml that does not parse", "tasks: [",
			func(de *Map) { cardAt(de, 1).Set("display", "ratio") }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newCaptureHarness(t, tc.schedules)
			res, err := h.svc.Save(context.Background(), h.dataEntryDraft(t, tc.edit))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(codes(res.Problems), "captured_card"); got != tc.refused {
				t.Fatalf("refused = %v, want %v; problems = %+v", got, tc.refused, res.Problems)
			}
			if res.Saved == tc.refused {
				t.Fatalf("saved = %v; problems = %+v", res.Saved, res.Problems)
			}
		})
	}
}

// The schema the card reads is part of what it computes: a captured sum's
// property may not change type in Configure.
func TestSave_CapturedCardSchema(t *testing.T) {
	sched := strings.Replace(captureSchedules, "card: Open tasks", "card: Notes", 1)
	h := newCaptureHarness(t, sched)
	h.files["data-entry.yaml"] = []byte(captureDataEntry + `    - title: Notes
      list: tasks
      display: max
      property: notes
`)
	res, err := h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		notes, _ := taskProps(s).Get("notes")
		notes.(*Map).Set("description", "free text")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codes(res.Problems), "captured_card") || res.Saved {
		t.Fatalf("result = %+v", res)
	}

	res, err = h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		state, _ := taskProps(s).Get("state")
		state.(*Map).Set("description", "where it is")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved {
		t.Fatalf("a property the card does not read was refused: %+v", res.Problems)
	}
}

// A property the card's list reads in a condition is part of what the card
// computes, so its definition is pinned too.
func TestSave_CapturedCardListCondition(t *testing.T) {
	h := newCaptureHarness(t, captureSchedules)
	h.files["data-entry.yaml"] = []byte(strings.Replace(captureDataEntry,
		"  tasks:\n    entity_type: task\n",
		"  tasks:\n    entity_type: task\n    condition: state == 'open'\n", 1))
	res, err := h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		state, _ := taskProps(s).Get("state")
		state.(*Map).Set("description", "where it is")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codes(res.Problems), "captured_card") || res.Saved {
		t.Fatalf("result = %+v", res)
	}

	res, err = h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		notes, _ := taskProps(s).Get("notes")
		notes.(*Map).Set("description", "free text")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved {
		t.Fatalf("a property the card does not read was refused: %+v", res.Problems)
	}
}

// A list that names no scope reads the type's default scope, so adding one
// changes the captured figure.
func TestSave_CapturedCardDefaultScope(t *testing.T) {
	h := newCaptureHarness(t, captureSchedules)
	res, err := h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		e, _ := s.Get("entities")
		task, _ := e.(*Map).Get("task")
		scopes := &Map{}
		scopes.Set("default", "entity.state == 'open'")
		task.(*Map).Set("query_scopes", scopes)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codes(res.Problems), "captured_card") || res.Saved {
		t.Fatalf("result = %+v", res)
	}
}

// Definitions are followed transitively: the folded property names notes,
// whose definition names state, so state is pinned too.
func TestSave_CapturedCardTransitive(t *testing.T) {
	sched := strings.Replace(captureSchedules, "card: Open tasks", "card: Score", 1)
	h := newCaptureHarness(t, sched)
	h.files["schema.yaml"] = []byte(strings.Replace(testSchema,
		"      notes:\n        type: string\n",
		"      notes:\n        type: string\n        description: copied from state\n"+
			"      score:\n        type: integer\n        description: length of notes\n", 1))
	h.files["data-entry.yaml"] = []byte(captureDataEntry + `    - title: Score
      list: others
      display: max
      property: score
`)
	res, err := h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		state, _ := taskProps(s).Get("state")
		state.(*Map).Set("description", "where it is")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codes(res.Problems), "captured_card") || res.Saved {
		t.Fatalf("result = %+v", res)
	}

	res, err = h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
		taskProps(s).Set("estimate", &Map{Keys: []string{"type"}, Values: []any{"integer"}})
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved {
		t.Fatalf("a property the card does not reach was refused: %+v", res.Problems)
	}
}

// A name is found in the raw text, not in an encoding of it: a condition
// that starts a line with the name, or glues it to < or &, still pins it.
func TestSave_CapturedCardRawConditionText(t *testing.T) {
	for _, cond := range []string{"|\n      true &&\n      state == 'open'", `"1<state"`, `"true&&state"`} {
		t.Run(cond, func(t *testing.T) {
			h := newCaptureHarness(t, captureSchedules)
			h.files["data-entry.yaml"] = []byte(strings.Replace(captureDataEntry,
				"  tasks:\n    entity_type: task\n",
				"  tasks:\n    entity_type: task\n    condition: "+cond+"\n", 1))
			res, err := h.svc.Save(context.Background(), h.draft(t, func(s *Map) {
				state, _ := taskProps(s).Get("state")
				state.(*Map).Set("description", "where it is")
			}))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(codes(res.Problems), "captured_card") || res.Saved {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

func TestLineage_FollowsExtends(t *testing.T) {
	sc := map[string]any{"entities": map[string]any{
		"task": map[string]any{"extends": "work"},
		"work": map[string]any{"query_scopes": map[string]any{"default": "x"}, "extends": "task"},
	}}
	got := lineage(sc, "task")
	if len(got) != 2 {
		t.Fatalf("lineage = %v", got)
	}
	work, _ := got["work"].(map[string]any)
	if work["query_scopes"] == nil {
		t.Fatalf("parent scopes missing: %v", got)
	}
}
