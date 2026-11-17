package configedit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const testFlows = `# Triggers fire in document order.
triggers:
  stamp:
    description: Stamp new tasks
    on: {entity: task, created: true}
    outputs:
      - set: {properties: {state: open}}
flows:
  review: # a flow the editor may not touch
    steps: [a, b]
`

// flowsDraft edits the flows.yaml tree with fn and wraps it in a Draft.
func (h *harness) flowsDraft(t *testing.T, fn func(flows *Map)) *Draft {
	t.Helper()
	snap, err := h.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	flows := snap.Flows.(*Map)
	fn(flows)
	raw, err := json.Marshal(flows)
	if err != nil {
		t.Fatal(err)
	}
	return &Draft{BaseVersion: snap.Version, Flows: raw}
}

func stampOn(flows *Map) *Map {
	tr, _ := flows.Get("triggers")
	r, _ := tr.(*Map).Get("stamp")
	on, _ := r.(*Map).Get("on")
	return on.(*Map)
}

func stampSet(flows *Map) *Map {
	tr, _ := flows.Get("triggers")
	r, _ := tr.(*Map).Get("stamp")
	outs, _ := r.(*Map).Get("outputs")
	set, _ := outs.([]any)[0].(*Map).Get("set")
	props, _ := set.(*Map).Get("properties")
	return props.(*Map)
}

// addStamp adds a trigger to a flows.yaml without one.
func addStamp(f *Map) {
	f.Set("triggers", &Map{Keys: []string{"stamp"}, Values: []any{&Map{
		Keys: []string{"on", "outputs"},
		Values: []any{
			&Map{Keys: []string{"entity", "created"}, Values: []any{"task", true}},
			[]any{&Map{Keys: []string{"set"}, Values: []any{
				&Map{Keys: []string{"properties"}, Values: []any{&Map{Keys: []string{"state"}, Values: []any{"open"}}}},
			}}},
		},
	}}})
}

func TestSave_FlowsOnly(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	h.files["flows.yaml"] = []byte(testFlows)
	before, err := h.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	d := h.flowsDraft(t, func(f *Map) { stampOn(f).Set("updated", true) })
	res, err := h.svc.Save(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved || len(res.Problems) > 0 || res.Version == before.Version {
		t.Fatalf("result = %+v", res)
	}
	got := string(h.files["flows.yaml"])
	for _, want := range []string{"updated: true", "# Triggers fire in document order.", "# a flow the editor may not touch"} {
		if !strings.Contains(got, want) {
			t.Errorf("flows.yaml misses %q:\n%s", want, got)
		}
	}
	if string(h.files["schema.yaml"]) != testSchema || len(h.act.activated) != 1 {
		t.Fatal("schema.yaml changed or the server was not switched")
	}
	recs := h.audit.Records()
	if len(recs) != 1 || !strings.Contains(recs[0].Summary, "flows.yaml "+short([]byte(testFlows))) ||
		!strings.Contains(recs[0].Summary, "flows.yaml "+short(h.files["flows.yaml"])) ||
		!strings.Contains(recs[0].Summary, "triggers.stamp.on.updated") {

		t.Fatalf("audit = %+v", recs)
	}
	if strings.Contains(recs[0].Summary, "Stamp new tasks") {
		t.Fatal("audit carries content")
	}
}

func TestSave_CreatesFlowsFile(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	snap, err := h.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := snap.Flows.(*Map); !ok || len(m.Keys) != 0 {
		t.Fatalf("absent flows.yaml reads as %#v", snap.Flows)
	}
	d := h.flowsDraft(t, addStamp)
	res, err := h.svc.Save(context.Background(), d)
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(string(h.files["flows.yaml"]), "stamp:") {
		t.Fatalf("flows.yaml = %q", h.files["flows.yaml"])
	}
	if recs := h.audit.Records(); len(recs) != 1 || !strings.Contains(recs[0].Summary, "flows.yaml absent") {
		t.Fatalf("audit = %+v", recs)
	}
}

// TestSave_EmptyDraftLeavesFlowsAbsent pins that a draft holding only empty
// mappings, such as one whose last automation was removed, does not create
// flows.yaml.
func TestSave_EmptyDraftLeavesFlowsAbsent(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	d := h.flowsDraft(t, func(f *Map) { f.Set("triggers", &Map{}) })
	if _, err := h.svc.Save(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.files["flows.yaml"]; ok {
		t.Fatalf("flows.yaml was created: %q", h.files["flows.yaml"])
	}
}

// TestSave_FlowsRefusals pins that a flows.yaml edit the allowlist, the
// trigger check or the ACL check refuses writes nothing.
func TestSave_FlowsRefusals(t *testing.T) {
	tests := []struct {
		name     string
		problems []TriggerProblem
		err      error
		edit     func(*Map)
		want     string
	}{
		{"broken trigger", nil, nil, func(f *Map) { stampOn(f).Set("property", "no_such_property") }, "invalid"},
		{"locked key", nil, nil, func(f *Map) { stampOn(f).Set("deleted", true) }, "locked"},
		{"refused by the ACL check", []TriggerProblem{{Trigger: "stamp", Message: "no"}}, nil,
			func(f *Map) { stampSet(f).Set("owner", "me") }, "acl_trigger"},
		{"the ACL check fails", nil, errors.New("acl.yaml is absent or empty"),
			func(f *Map) { stampSet(f).Set("owner", "me") }, "acl_trigger"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, counts{}, nil)
			h.files["flows.yaml"] = []byte(testFlows)
			h.files["acl.yaml"] = []byte("roles: {}\n")
			h.triggers.problems, h.triggers.err = tc.problems, tc.err
			d := h.flowsDraft(t, tc.edit)
			res, err := h.svc.Preview(context.Background(), d)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(codes(res.Problems), tc.want) {
				t.Fatalf("preview: want %s, got %+v", tc.want, res.Problems)
			}
			res, err = h.svc.Save(context.Background(), d)
			if err != nil {
				t.Fatal(err)
			}
			if res.Saved || string(h.files["flows.yaml"]) != testFlows || len(h.act.activated) != 0 {
				t.Fatalf("a refused save changed something: %+v", res)
			}
		})
	}
}

// TestPreview_TriggerCheckInput pins what the ACL check is asked about: a
// flows-only edit and a schema-only edit are judged, with the files as read
// and as saved; an edit of neither is not.
func TestPreview_TriggerCheckInput(t *testing.T) {
	relabel := func(s *Map) {
		e, _ := s.Get("entities")
		task, _ := e.(*Map).Get("task")
		task.(*Map).Set("label", "Job")
	}
	tests := []struct {
		name      string
		draft     func(*testing.T, *harness) *Draft
		wantCheck bool
		flowsSame bool
	}{
		{"flows only", func(t *testing.T, h *harness) *Draft {
			t.Helper()
			return h.flowsDraft(t, func(f *Map) { stampOn(f).Set("updated", true) })
		}, true, false},
		{"schema only", func(t *testing.T, h *harness) *Draft {
			t.Helper()
			return h.draft(t, relabel)
		}, true, true},
		{"data-entry only", func(t *testing.T, h *harness) *Draft {
			t.Helper()
			snap, err := h.svc.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			de := snap.DataEntry.(*Map)
			app, _ := de.Get("app")
			app.(*Map).Set("name", "Other")
			raw, err := json.Marshal(de)
			if err != nil {
				t.Fatal(err)
			}
			return &Draft{BaseVersion: snap.Version, DataEntry: raw}
		}, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, counts{}, nil)
			h.files["flows.yaml"] = []byte(testFlows)
			h.files["acl.yaml"] = []byte("roles: {}\n")
			res, err := h.svc.Preview(context.Background(), tc.draft(t, h))
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Problems) > 0 {
				t.Fatalf("problems: %+v", res.Problems)
			}
			if got := len(h.triggers.edits) > 0; got != tc.wantCheck {
				t.Fatalf("checked = %v, want %v", got, tc.wantCheck)
			}
			if !tc.wantCheck {
				return
			}
			e := h.triggers.edits[0]
			if string(e.ACL) != "roles: {}\n" || string(e.FlowsBefore) != testFlows ||
				string(e.SchemaBefore) != testSchema || len(e.SchemaAfter) == 0 {

				t.Fatalf("edit = %+v", e)
			}
			if same := string(e.FlowsAfter) == testFlows; same != tc.flowsSame {
				t.Fatalf("flows after = %q", e.FlowsAfter)
			}
		})
	}
}

func TestSave_PrepareFailureRestoresFlows(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	h.act.prepareErr = &ValidationError{Problems: []string{"boom"}}
	d := h.flowsDraft(t, addStamp)
	if res, err := h.svc.Save(context.Background(), d); err != nil || res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, ok := h.files["flows.yaml"]; ok {
		t.Fatal("a flows.yaml that was absent was left behind")
	}

	h = newHarness(t, counts{}, nil)
	h.files["flows.yaml"] = []byte(testFlows)
	h.act.prepareErr = &ValidationError{Problems: []string{"boom"}}
	d = h.flowsDraft(t, func(f *Map) { stampOn(f).Set("updated", true) })
	if res, err := h.svc.Save(context.Background(), d); err != nil || res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if string(h.files["flows.yaml"]) != testFlows {
		t.Fatalf("flows.yaml not restored:\n%s", h.files["flows.yaml"])
	}
}
