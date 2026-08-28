package dataentry

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func caldavTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Types: map[string]metamodel.CustomType{
			"task_status":   {Values: []string{"todo", "doing", "done"}, Default: "todo"},
			"task_priority": {Values: []string{"high", "normal", "low"}, Default: "normal"},
		},
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label: "Task", IDPrefix: "TSK-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":        {Type: metamodel.PropertyTypeString, Required: true},
					"due":          {Type: metamodel.PropertyTypeDate},
					"at":           {Type: metamodel.PropertyTypeDatetime},
					"status":       {Type: "task_status", Required: true},
					"completed_at": {Type: metamodel.PropertyTypeDatetime},
					"notes":        {Type: metamodel.PropertyTypeString},
					"rank":         {Type: metamodel.PropertyTypeInteger},
					"secret":       {Type: metamodel.PropertyTypeString},
					"urgency":      {Type: "task_priority"},
					"place":        {Type: metamodel.PropertyTypeString},
					"tags":         {Type: metamodel.PropertyTypeString, List: true},
					"starts":       {Type: metamodel.PropertyTypeDate},
				},
			},
		},
	}
}

func caldavTestCollection() dataentryconfig.CalDAVCollection {
	return dataentryconfig.CalDAVCollection{
		Component:   dataentryconfig.CalDAVComponentTodo,
		EntityType:  "task",
		Due:         "due",
		Summary:     "title",
		Description: "notes",
		Priority:    "rank",
		Completion: &dataentryconfig.CalDAVCompletion{
			StatusProperty: "status",
			CompletedValue: "done",
			PendingValue:   "todo",
			CompletedAt:    "completed_at",
		},
		Defaults: map[string]string{"status": "todo"},
		OnDelete: &dataentryconfig.CalDAVOnDelete{Set: map[string]string{"status": "done"}},
	}
}

func testMapper(t *testing.T, mutate ...func(*dataentryconfig.CalDAVCollection)) *caldavMapper {
	t.Helper()
	cfg := caldavTestCollection()
	for _, f := range mutate {
		f(&cfg)
	}
	return newCalDAVMapper("tasks", cfg, caldavTestMeta(),
		func(entityType, id string) string { return "/entity/" + entityType + "/" + id })
}

// inbound builds an inboundTodo whose "sent" set names exactly the properties a
// client transmitted. Tests must state this explicitly, because absent and
// empty produce DIFFERENT writes: an omitted DESCRIPTION must leave the mapped
// property alone, while an empty one clears it.
func inbound(td calfeed.Todo, sent ...string) inboundTodo {
	set := map[string]bool{}
	for _, p := range sent {
		set[p] = true
	}
	return inboundTodo{Todo: td, sent: set}
}

func taskEntity(props map[string]any) *entitypkg.Entity {
	e := entitypkg.New("TSK-1", "task")
	e.Properties = props
	return e
}

func TestCalDAVMapper_ToTodo(t *testing.T) {
	m := testMapper(t)
	td := m.toTodo(taskEntity(map[string]any{
		"title": "Buy milk", "notes": "semi-skimmed", "due": "2026-08-10",
		"status": "todo", "rank": "5",
	}), "task--TSK-1@rela", "/entity/task/TSK-1")

	if td.Summary != "Buy milk" || td.Description != "semi-skimmed" {
		t.Errorf("text fields wrong: %+v", td)
	}
	if td.Due.Format(time.DateOnly) != "2026-08-10" || td.Timed {
		t.Errorf("a date-typed property must map to an all-day due: %v timed=%v", td.Due, td.Timed)
	}
	if td.Priority != 5 {
		t.Errorf("Priority = %d, want 5", td.Priority)
	}
	if td.Status == calfeed.TodoCompleted {
		t.Error("a todo-status entity must not be completed")
	}
}

// TestCalDAVMapper_DueTypeDrivesAllDay pins that the declared property TYPE
// decides all-day vs timed, matching the ICS feed's rule.
func TestCalDAVMapper_DueTypeDrivesAllDay(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) { c.Due = "at" })
	td := m.toTodo(taskEntity(map[string]any{
		"title": "Standup", "at": "2026-08-10T09:30:00Z", "status": "todo",
	}), "u", "")
	if !td.Timed {
		t.Error("a datetime-typed property must map to a timed due")
	}
	if got := td.Due.UTC().Format(time.RFC3339); got != "2026-08-10T09:30:00Z" {
		t.Errorf("due = %q", got)
	}
}

func TestCalDAVMapper_CompletionOut(t *testing.T) {
	m := testMapper(t)
	td := m.toTodo(taskEntity(map[string]any{
		"title": "Done thing", "status": "done", "completed_at": "2026-08-09T08:14:06Z",
	}), "u", "")

	if td.Status != calfeed.TodoCompleted {
		t.Errorf("Status = %q, want COMPLETED", td.Status)
	}
	if got := td.Completed.UTC().Format(time.RFC3339); got != "2026-08-09T08:14:06Z" {
		t.Errorf("Completed = %q", got)
	}
	if td.PercentComplete != 100 {
		t.Errorf("PercentComplete = %d, want 100", td.PercentComplete)
	}
}

// TestCalDAVMapper_CompletedWithoutTimestamp guards the filter interaction: RFC
// 4791 §7.8.9's pending query keys on COMPLETED being ABSENT, so a done entity
// with no recorded timestamp must still get one or it reappears as pending.
func TestCalDAVMapper_CompletedWithoutTimestamp(t *testing.T) {
	m := testMapper(t)
	td := m.toTodo(taskEntity(map[string]any{"title": "x", "status": "done"}), "u", "")
	if td.Status != calfeed.TodoCompleted {
		t.Fatalf("Status = %q, want COMPLETED", td.Status)
	}
	if td.Completed.IsZero() {
		t.Error("a completed to-do must carry a COMPLETED timestamp, or filter-driven clients see it as pending")
	}
}

// TestCalDAVMapper_PatchOnlyNamesMappedProperties is the data-safety property.
// An inbound patch must never mention a property the collection does not map,
// so PatchEntity leaves everything else — including values the caller cannot
// read — untouched.
func TestCalDAVMapper_PatchOnlyNamesMappedProperties(t *testing.T) {
	m := testMapper(t)
	td := calfeed.Todo{UID: "u", Summary: "Renamed", Due: mustDay(t, "2026-08-11")}

	patch, err := m.patchFor(inbound(td, ical.PropSummary, ical.PropDue))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}

	mapped := map[string]bool{"title": true, "notes": true, "rank": true, "due": true,
		"status": true, "completed_at": true}
	for k := range patch.Properties {
		if !mapped[k] {
			t.Errorf("patch names unmapped property %q — PatchEntity would overwrite it", k)
		}
	}
	for _, k := range patch.MetaUnset {
		if !mapped[k] {
			t.Errorf("patch unsets unmapped property %q", k)
		}
	}
	if _, ok := patch.Properties["secret"]; ok {
		t.Error("an unmapped property leaked into the patch")
	}
}

func TestCalDAVMapper_PatchCompletion(t *testing.T) {
	m := testMapper(t)

	t.Run("completing sets the status and stamp", func(t *testing.T) {
		td := calfeed.Todo{UID: "u", Summary: "x"}
		td.Complete(time.Date(2026, 8, 9, 8, 14, 6, 0, time.UTC))
		patch, err := m.patchFor(inbound(td, ical.PropSummary, ical.PropStatus, ical.PropCompleted))
		if err != nil {
			t.Fatalf("patchFor: %v", err)
		}
		if patch.Properties["status"] != "done" {
			t.Errorf("status = %v, want done", patch.Properties["status"])
		}
		if patch.Properties["completed_at"] != "2026-08-09T08:14:06Z" {
			t.Errorf("completed_at = %v", patch.Properties["completed_at"])
		}
	})

	t.Run("re-opening restores pending and clears the stamp", func(t *testing.T) {
		patch, err := m.patchFor(inbound(
			calfeed.Todo{UID: "u", Summary: "x", Status: calfeed.TodoNeedsAction},
			ical.PropSummary, ical.PropStatus))
		if err != nil {
			t.Fatalf("patchFor: %v", err)
		}
		if patch.Properties["status"] != "todo" {
			t.Errorf("status = %v, want todo", patch.Properties["status"])
		}
		// The stamp must be UNSET, not left behind: an entity claiming a
		// completion time for unfinished work is a lie.
		var cleared bool
		for _, k := range patch.MetaUnset {
			if k == "completed_at" {
				cleared = true
			}
		}
		if !cleared {
			t.Error("re-opening must clear completed_at")
		}
	})
}

// TestCalDAVMapper_ClearedDueIsUnsetNotOmitted: omitting the property would
// silently keep the old deadline, so clearing must be explicit.
func TestCalDAVMapper_ClearedDueIsUnsetNotOmitted(t *testing.T) {
	m := testMapper(t)
	// The client SENT an empty DUE — an explicit clear, distinct from omitting it.
	patch, err := m.patchFor(inbound(calfeed.Todo{UID: "u", Summary: "x"},
		ical.PropSummary, ical.PropDue))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if _, present := patch.Properties["due"]; present {
		t.Error("a cleared due must not be written as a property value")
	}
	var unset bool
	for _, k := range patch.MetaUnset {
		if k == "due" {
			unset = true
		}
	}
	if !unset {
		t.Error("a cleared due must be explicitly unset, or the old value survives")
	}
}

// TestCalDAVMapper_RoundTrip is the symmetry claim: the same declaration drives
// both directions, so a to-do projected out and patched back must preserve the
// mapped values.
func TestCalDAVMapper_RoundTrip(t *testing.T) {
	m := testMapper(t)
	original := taskEntity(map[string]any{
		"title": "Renew passport", "notes": "at the town hall", "due": "2026-08-10",
		"status": "todo", "rank": "5", "secret": "keep me",
	})

	td := m.toTodo(original, "u", "")
	patch, err := m.patchFor(inbound(td,
		ical.PropSummary, ical.PropDescription, ical.PropDue, ical.PropStatus))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	patch.Apply(original)

	for prop, want := range map[string]string{
		"title": "Renew passport", "notes": "at the town hall", "due": "2026-08-10",
		"status": "todo", "secret": "keep me",
	} {
		if got := original.GetString(prop); got != want {
			t.Errorf("%s = %q after a round trip, want %q", prop, got, want)
		}
	}
}

func TestCalDAVMapper_CreateProperties(t *testing.T) {
	m := testMapper(t)

	// The real inbound shape: Apple sends a summary and nothing else.
	patch, err := m.createPatch(inbound(
		calfeed.Todo{UID: "u", Summary: "this is a test"}, ical.PropSummary))
	if err != nil {
		t.Fatalf("createPatch: %v", err)
	}
	props := patch.Properties
	if props["title"] != "this is a test" {
		t.Errorf("title = %v", props["title"])
	}
	if props["status"] != "todo" {
		t.Errorf("status = %v, want the configured default", props["status"])
	}
}

// TestCalDAVMapper_DefaultsDoNotOverrideClient: a default is what to use when
// the client said nothing, not a value that wins over what it sent.
func TestCalDAVMapper_DefaultsDoNotOverrideClient(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
		c.Defaults = map[string]string{"status": "todo", "title": "PLACEHOLDER"}
	})
	patch, err := m.createPatch(inbound(
		calfeed.Todo{UID: "u", Summary: "real title"}, ical.PropSummary))
	if err != nil {
		t.Fatalf("createPatch: %v", err)
	}
	props := patch.Properties
	if props["title"] != "real title" {
		t.Errorf("a default overrode the client's value: %v", props["title"])
	}
}

func TestCalDAVMapper_DeletePatch(t *testing.T) {
	t.Run("configured status transition", func(t *testing.T) {
		patch, hard, ok := testMapper(t).deletePatch()
		if !ok || hard {
			t.Fatalf("ok=%v hard=%v, want a configured soft delete", ok, hard)
		}
		if patch.Properties["status"] != "done" {
			t.Errorf("status = %v, want done", patch.Properties["status"])
		}
	})

	t.Run("hard delete opt-in", func(t *testing.T) {
		m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
			c.OnDelete = &dataentryconfig.CalDAVOnDelete{Hard: true}
		})
		if _, hard, ok := m.deletePatch(); !ok || !hard {
			t.Errorf("ok=%v hard=%v, want an opted-in hard delete", ok, hard)
		}
	})

	t.Run("unconfigured means refuse", func(t *testing.T) {
		m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) { c.OnDelete = nil })
		if _, _, ok := m.deletePatch(); ok {
			t.Error("an unconfigured on_delete must report not-configured so the handler can refuse")
		}
	})
}

// TestCalDAVMapper_SummaryFallsBackToDisplayProperty: config validation allows
// summary to be omitted when the type has a display property.
func TestCalDAVMapper_SummaryFallsBackToDisplayProperty(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) { c.Summary = "" })
	td := m.toTodo(taskEntity(map[string]any{"title": "via display prop", "status": "todo"}), "u", "")
	if td.Summary != "via display prop" {
		t.Errorf("Summary = %q, want the display property's value", td.Summary)
	}
}

func mustDay(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

// TestCalDAVMapper_OmittedPropertyIsNotErased is the data-loss guard. Apple
// omits DESCRIPTION whenever the note is empty, and PatchEntity treats a named
// property as a WRITE — so writing an empty value is exactly as destructive as
// an unset. An omitted property must not appear in the patch at all.
func TestCalDAVMapper_OmittedPropertyIsNotErased(t *testing.T) {
	m := testMapper(t)
	// The real shape of an Apple write-back: summary and status, nothing else.
	patch, err := m.patchFor(inbound(
		calfeed.Todo{UID: "u", Summary: "kept", Status: calfeed.TodoNeedsAction},
		ical.PropSummary, ical.PropStatus))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}

	for _, prop := range []string{"notes", "due", "rank"} {
		if v, present := patch.Properties[prop]; present {
			t.Errorf("omitted property %q was written as %#v — this erases it", prop, v)
		}
		for _, k := range patch.MetaUnset {
			if k == prop {
				t.Errorf("omitted property %q was unset — this erases it", prop)
			}
		}
	}
}

// TestCalDAVMapper_EmptyIsDistinctFromAbsent: a client that SENDS an empty
// description means "clear the note", which must still work.
func TestCalDAVMapper_EmptyIsDistinctFromAbsent(t *testing.T) {
	m := testMapper(t)
	patch, err := m.patchFor(inbound(
		calfeed.Todo{UID: "u", Summary: "x", Description: ""},
		ical.PropSummary, ical.PropDescription))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	v, present := patch.Properties["notes"]
	if !present || v != "" {
		t.Errorf("an explicitly-sent empty description must clear the note, got present=%v %#v", present, v)
	}
}

// TestCalDAVMapper_UnmappableStatusLeavesTheProperty: RFC 5545 defines
// IN-PROCESS and CANCELLED for a VTODO, and neither corresponds to the binary
// done/pending a collection declares. Mapping them onto the pending value would
// silently reset a task the user moved to "doing", and resurrect a cancelled one.
func TestCalDAVMapper_UnmappableStatusLeavesTheProperty(t *testing.T) {
	m := testMapper(t)
	for _, status := range []calfeed.TodoStatus{calfeed.TodoInProcess, calfeed.TodoCancelled} {
		t.Run(string(status), func(t *testing.T) {
			patch, err := m.patchFor(inbound(
				calfeed.Todo{UID: "u", Summary: "x", Status: status},
				ical.PropSummary, ical.PropStatus))
			if err != nil {
				t.Fatalf("patchFor: %v", err)
			}
			if v, present := patch.Properties["status"]; present {
				t.Errorf("STATUS:%s wrote status=%v; it has no rela meaning and must leave the property alone",
					status, v)
			}
		})
	}
}

// TestCalDAVMapper_UnsentStatusLeavesTheProperty: a client editing only the
// title must not restate — and therefore must not change — the status.
func TestCalDAVMapper_UnsentStatusLeavesTheProperty(t *testing.T) {
	m := testMapper(t)
	patch, err := m.patchFor(inbound(calfeed.Todo{UID: "u", Summary: "retitled"}, ical.PropSummary))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if v, present := patch.Properties["status"]; present {
		t.Errorf("status=%v was written though the client never sent STATUS", v)
	}
}

// TestCalDAVMapper_DueAcceptsBothYAMLShapes: yaml.v3 decodes an unquoted
// `due: 2026-08-12` straight to time.Time, while a quoted or machine-written
// value arrives as a string. Entity.GetString returns "" for the former, so
// reading a date through it silently drops every hand-authored due date — the
// mapper must accept both shapes.
func TestCalDAVMapper_DueAcceptsBothYAMLShapes(t *testing.T) {
	m := testMapper(t)
	want := "2026-08-12"

	for _, tc := range []struct {
		name  string
		value any
	}{
		{"string (quoted or machine-written)", want},
		{"time.Time (unquoted YAML scalar)", time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			td := m.toTodo(taskEntity(map[string]any{
				"title": "x", "status": "todo", "due": tc.value,
			}), "u", "")
			if td.Due.IsZero() {
				t.Fatal("due date was dropped entirely")
			}
			if got := td.Due.Format(time.DateOnly); got != want {
				t.Errorf("due = %q, want %q", got, want)
			}
		})
	}
}

// TestTodoFromICal_CompletedWithoutStatusIsCompletion pins the promotion rule
// on the INBOUND path.
//
// RFC 5545 does not require STATUS alongside COMPLETED, and a client may send
// the timestamp alone. Before this, such a PUT passed the "did the client speak
// about completion?" guard and then fell through applyCompletionToPatch's
// switch to `default:` — writing nothing. The result was the silently reverting
// checkbox: 201, no warning, no log, and the next sync renders
// STATUS:NEEDS-ACTION back over the user's tick.
//
// This is the same rule calfeed.Todo.normalized() applies outbound ("a
// timestamp is the stronger signal"). Only the promotion arm applies here —
// normalized() also demotes a COMPLETED carrying no timestamp, which would be
// wrong inbound, since a client legitimately sends STATUS:COMPLETED alone.
func TestTodoFromICal_CompletedWithoutStatusIsCompletion(t *testing.T) {
	const body = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:u1\r\n" +
		"DTSTAMP:20260811T080000Z\r\nSUMMARY:Thing\r\nCOMPLETED:20260811T120000Z\r\n" +
		"PERCENT-COMPLETE:100\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"

	cal, err := ical.NewDecoder(strings.NewReader(body)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	in, err := todoFromICal(cal)
	if err != nil {
		t.Fatalf("todoFromICal: %v", err)
	}

	if in.Todo.Status != calfeed.TodoCompleted {
		t.Errorf("Status = %q, want COMPLETED: a timestamp with no STATUS is "+
			"still a completion, and ignoring it reverts the user's checkbox",
			in.Todo.Status)
	}
	// The mapper keys the write on has(STATUS); without this the promoted
	// status is computed and then never applied.
	if !in.has(ical.PropStatus) {
		t.Error("STATUS not marked as sent, so applyCompletionToPatch will skip it")
	}
}

// TestDescriptionMapsToBody covers `description: body`, the sentinel that maps
// DESCRIPTION to the entity's markdown body instead of to a property.
//
// The body is the natural target: DESCRIPTION is the one free-text, multi-line
// field a to-do has, and a `string` property renders as a single-line input
// everywhere else in the app — so routing a client's multi-line notes into one
// puts a paragraph in a text box.
func TestDescriptionMapsToBody(t *testing.T) {
	bodyMapper := func(t *testing.T) *caldavMapper {
		t.Helper()
		return testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
			c.Description = dataentryconfig.CalDAVDescriptionBody
		})
	}

	t.Run("read: the body becomes DESCRIPTION", func(t *testing.T) {
		m := bodyMapper(t)
		td := m.toTodo(&entitypkg.Entity{
			ID: "TSK-1", Type: "task",
			Properties: map[string]any{"title": "Thing"},
			Content:    "line one\n\nline two",
		}, "u1", "/entity/task/TSK-1")

		if td.Description != "line one\n\nline two" {
			t.Errorf("Description = %q, want the entity body", td.Description)
		}
	})

	t.Run("write: DESCRIPTION replaces the body, not a property", func(t *testing.T) {
		m := bodyMapper(t)
		patch, err := m.patchFor(inbound(
			calfeed.Todo{UID: "u1", Description: "client note"}, ical.PropDescription))
		if err != nil {
			t.Fatalf("patchFor: %v", err)
		}

		if patch.Content == nil {
			t.Fatal("Content is nil; the body would be left unchanged")
		}
		if *patch.Content != "client note" {
			t.Errorf("Content = %q, want the client's note", *patch.Content)
		}
		// It must NOT also land in a property called "body" — that would create
		// a phantom property the metamodel never declared.
		if _, stray := patch.Properties[dataentryconfig.CalDAVDescriptionBody]; stray {
			t.Error("the sentinel leaked into Properties as a real property name")
		}
	})

	t.Run("an omitted DESCRIPTION preserves the existing body", func(t *testing.T) {
		m := bodyMapper(t)
		// Apple omits DESCRIPTION whenever the note is empty, so an
		// unconditional write would blank the body on every completion sync.
		patch, err := m.patchFor(inbound(
			calfeed.Todo{UID: "u1", Summary: "Renamed"}, ical.PropSummary))
		if err != nil {
			t.Fatalf("patchFor: %v", err)
		}
		if patch.Content != nil {
			t.Errorf("Content = %q for a write that never mentioned DESCRIPTION; "+
				"nil is what preserves the body", *patch.Content)
		}
	})

	t.Run("create carries the body through", func(t *testing.T) {
		m := bodyMapper(t)
		patch, err := m.createPatch(inbound(
			calfeed.Todo{UID: "u1", Summary: "New", Description: "notes here"},
			ical.PropSummary, ical.PropDescription))
		if err != nil {
			t.Fatalf("createPatch: %v", err)
		}
		if patch.Content == nil || *patch.Content != "notes here" {
			t.Error("a client-created to-do lost its body; createPatch returns the " +
				"whole patch precisely so a caller cannot drop it")
		}
	})

	t.Run("a property mapping still targets the property", func(t *testing.T) {
		m := testMapper(t) // the default fixture maps description -> notes
		patch, err := m.patchFor(inbound(
			calfeed.Todo{UID: "u1", Description: "client note"}, ical.PropDescription))
		if err != nil {
			t.Fatalf("patchFor: %v", err)
		}
		if patch.Content != nil {
			t.Error("a property-mapped description must not touch the body")
		}
		if patch.Properties["notes"] != "client note" {
			t.Errorf("notes = %v, want the client's note", patch.Properties["notes"])
		}
	})
}

// TestPriorityMapRoundTrip pins the bucketed enum mapping in both directions.
//
// Outbound, a bucket emits ONE number for its band; inbound, the whole band
// resolves back to the same value. That asymmetry is the point: clients pick
// their own number within a band, so the inbound side must accept the range
// while the outbound side has to choose a representative.
func TestPriorityMapRoundTrip(t *testing.T) {
	mapper := func(t *testing.T) *caldavMapper {
		t.Helper()
		return testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
			c.Priority = ""
			c.PriorityMap = &dataentryconfig.CalDAVPriorityMap{
				Property: "urgency",
				Buckets: []dataentryconfig.CalDAVPriorityBucket{
					{Value: "high", From: 1, To: 4, Emit: 1},
					{Value: "normal", From: 5, To: 5},
					{Value: "low", From: 6, To: 9, Emit: 9},
				},
			}
		})
	}

	t.Run("outbound: the property value emits its bucket's number", func(t *testing.T) {
		for value, want := range map[string]int{"high": 1, "normal": 5, "low": 9} {
			td := mapper(t).toTodo(&entitypkg.Entity{
				ID: "TSK-1", Type: "task",
				Properties: map[string]any{"title": "T", "urgency": value},
			}, "u1", "")
			if td.Priority != want {
				t.Errorf("%s -> PRIORITY %d, want %d", value, td.Priority, want)
			}
		}
	})

	t.Run("inbound: every number in a band maps to that band's value", func(t *testing.T) {
		// The numbers real clients actually send, plus the band edges.
		for prio, want := range map[int]string{1: "high", 4: "high", 5: "normal", 6: "low", 9: "low"} {
			patch, err := mapper(t).patchFor(inbound(
				calfeed.Todo{UID: "u1", Priority: prio}, ical.PropPriority))
			if err != nil {
				t.Fatalf("patchFor: %v", err)
			}
			if got := patch.Properties["urgency"]; got != want {
				t.Errorf("PRIORITY %d -> %v, want %q", prio, got, want)
			}
		}
	})

	t.Run("an uncovered value leaves the property alone", func(t *testing.T) {
		// 0 is RFC 5545's "undefined" and is deliberately not bucketed. Writing
		// a guess would silently reclassify a to-do the client said nothing about.
		patch, err := mapper(t).patchFor(inbound(
			calfeed.Todo{UID: "u1", Priority: 0}, ical.PropPriority))
		if err != nil {
			t.Fatalf("patchFor: %v", err)
		}
		if _, set := patch.Properties["urgency"]; set {
			t.Error("PRIORITY:0 (undefined) wrote a priority the client never expressed")
		}
	})
}

// TestExtraPropertiesRoundTrip covers LOCATION, CATEGORIES and DTSTART.
func TestExtraPropertiesRoundTrip(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
		c.Location, c.Categories, c.Start = "place", "tags", "starts"
	})

	td := m.toTodo(&entitypkg.Entity{
		ID: "TSK-1", Type: "task",
		Properties: map[string]any{
			"title": "T", "place": "Albert Heijn",
			"tags": []string{"errands", "food"}, "starts": "2026-08-11",
		},
	}, "u1", "")

	if td.Location != "Albert Heijn" {
		t.Errorf("Location = %q", td.Location)
	}
	if len(td.Categories) != 2 || td.Categories[0] != "errands" {
		t.Errorf("Categories = %v", td.Categories)
	}
	if td.Start.IsZero() {
		t.Error("Start not mapped from the date property")
	}

	patch, err := m.patchFor(inbound(
		calfeed.Todo{UID: "u1", Location: "Jumbo", Categories: []string{"shopping"}},
		ical.PropLocation, ical.PropCategories))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if patch.Properties["place"] != "Jumbo" {
		t.Errorf("place = %v", patch.Properties["place"])
	}
	if got, ok := patch.Properties["tags"].([]string); !ok || len(got) != 1 || got[0] != "shopping" {
		t.Errorf("tags = %v", patch.Properties["tags"])
	}
}

// TestCategoriesAcceptsRepeatedProperties: CATEGORIES may repeat, and clients
// differ on which form they use.
//
// rela emits ONE comma-separated line; Thunderbird sends a SEPARATE line per
// category (verified on the wire). Reading only the first property kept one tag
// and silently dropped the rest — so a user's other tags vanished on the next
// sync, which is how "Cliënten" replaced "errands, food" in the live demo.
func TestCategoriesAcceptsRepeatedProperties(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "one comma-separated line (rela's own form)",
			body: "CATEGORIES:errands,food\r\n",
			want: []string{"errands", "food"},
		},
		{
			name: "repeated properties (Thunderbird's form)",
			body: "CATEGORIES:errands\r\nCATEGORIES:food\r\n",
			want: []string{"errands", "food"},
		},
		{
			name: "both forms mixed",
			body: "CATEGORIES:errands,food\r\nCATEGORIES:weekly\r\n",
			want: []string{"errands", "food", "weekly"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:u1\r\n" +
				"DTSTAMP:20260811T080000Z\r\nSUMMARY:T\r\n" + tc.body +
				"END:VTODO\r\nEND:VCALENDAR\r\n"
			cal, err := ical.NewDecoder(strings.NewReader(raw)).Decode()
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			in, err := todoFromICal(cal)
			if err != nil {
				t.Fatalf("todoFromICal: %v", err)
			}
			if len(in.Todo.Categories) != len(tc.want) {
				t.Fatalf("Categories = %v, want %v", in.Todo.Categories, tc.want)
			}
			for i, w := range tc.want {
				if in.Todo.Categories[i] != w {
					t.Errorf("Categories[%d] = %q, want %q", i, in.Todo.Categories[i], w)
				}
			}
		})
	}
}

// fullCollection maps every optional field, so a read_only test can name any of
// them without the "not mapped" case masking the "not written" one.
func fullCollection(c *dataentryconfig.CalDAVCollection) {
	c.Location = "place"
	c.Categories = "tags"
	c.Start = "starts"
}

// richTodo is a client edit that touches every mapped field at once — the shape
// a real client actually PUTs, since clients resend the whole VTODO.
func richTodo() inboundTodo {
	return inbound(calfeed.Todo{
		Summary:     "client title",
		Description: "client notes",
		Due:         time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Priority:    1,
		Location:    "client place",
		Categories:  []string{"client-tag"},
		Start:       time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
		Status:      calfeed.TodoCompleted,
		Completed:   time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
	},
		ical.PropSummary, ical.PropDescription, ical.PropDue, ical.PropPriority,
		ical.PropLocation, ical.PropCategories, ical.PropDateTimeStart,
		ical.PropStatus, ical.PropCompleted)
}

// TestCalDAVMapper_ReadOnlyDropsInboundField pins that a read-only field is
// absent from the patch entirely.
//
// Absent, not empty: PatchEntity preserves properties the patch does not name,
// so an omitted key is what makes the stored value survive. Naming it with a
// zero value would be exactly as destructive as the edit we are refusing.
func TestCalDAVMapper_ReadOnlyDropsInboundField(t *testing.T) {
	tests := []struct {
		field string
		prop  string
	}{
		{dataentryconfig.CalDAVFieldSummary, "title"},
		{dataentryconfig.CalDAVFieldDescription, "notes"},
		{dataentryconfig.CalDAVFieldDue, "due"},
		{dataentryconfig.CalDAVFieldPriority, "rank"},
		{dataentryconfig.CalDAVFieldLocation, "place"},
		{dataentryconfig.CalDAVFieldCategories, "tags"},
		{dataentryconfig.CalDAVFieldStart, "starts"},
		{dataentryconfig.CalDAVFieldCompletion, "status"},
	}
	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			// Writable first: proves the field WOULD be written, so the
			// read-only assertion below is not passing for some other reason.
			open := testMapper(t, fullCollection)
			openPatch, err := open.patchFor(richTodo())
			if err != nil {
				t.Fatalf("patchFor: %v", err)
			}
			if _, ok := openPatch.Properties[tc.prop]; !ok {
				t.Fatalf("precondition: %q is not written even when writable", tc.prop)
			}

			locked := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
				c.ReadOnly = []string{tc.field}
			})
			patch, err := locked.patchFor(richTodo())
			if err != nil {
				t.Fatalf("patchFor: %v", err)
			}
			if v, ok := patch.Properties[tc.prop]; ok {
				t.Errorf("read_only %q still wrote %s = %v", tc.field, tc.prop, v)
			}
			for _, u := range patch.MetaUnset {
				if u == tc.prop {
					t.Errorf("read_only %q unset %s, which erases it", tc.field, tc.prop)
				}
			}
		})
	}
}

// TestCalDAVMapper_ReadOnlyLeavesOtherFieldsWritable is the property that makes
// this feature usable: locking the rich fields must still let a user tick a
// to-do off, because a client sends every field in one PUT.
func TestCalDAVMapper_ReadOnlyLeavesOtherFieldsWritable(t *testing.T) {
	m := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
		c.ReadOnly = []string{
			dataentryconfig.CalDAVFieldSummary, dataentryconfig.CalDAVFieldDescription,
			dataentryconfig.CalDAVFieldDue, dataentryconfig.CalDAVFieldPriority,
			dataentryconfig.CalDAVFieldLocation, dataentryconfig.CalDAVFieldCategories,
			dataentryconfig.CalDAVFieldStart,
		}
	})
	patch, err := m.patchFor(richTodo())
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if patch.Properties["status"] != "done" {
		t.Errorf("completion must stay writable: status = %v", patch.Properties["status"])
	}
	for _, prop := range []string{"title", "notes", "due", "rank", "place", "tags", "starts"} {
		if v, ok := patch.Properties[prop]; ok {
			t.Errorf("%s should be read-only, got %v", prop, v)
		}
	}
}

// TestCalDAVMapper_ReadOnlyDescriptionProtectsBody pins that the body sentinel
// is covered too. The body travels in Patch.Content, not Properties, so a guard
// that only filtered the property map would leave it wide open.
func TestCalDAVMapper_ReadOnlyDescriptionProtectsBody(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
		c.Description = dataentryconfig.CalDAVDescriptionBody
		c.ReadOnly = []string{dataentryconfig.CalDAVFieldDescription}
	})
	patch, err := m.patchFor(richTodo())
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if patch.Content != nil {
		t.Errorf("read_only description must leave the body alone, got %q", *patch.Content)
	}
}

// TestCalDAVMapper_ReadOnlyPriorityMapCovered pins that `priority` covers the
// bucketed spelling as well: one client-visible field, one name.
func TestCalDAVMapper_ReadOnlyPriorityMapCovered(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
		c.Priority = ""
		c.PriorityMap = &dataentryconfig.CalDAVPriorityMap{
			Property: "urgency",
			Buckets: []dataentryconfig.CalDAVPriorityBucket{
				{Value: "high", From: 1, To: 4},
				{Value: "normal", From: 5, To: 5},
				{Value: "low", From: 6, To: 9},
			},
		}
		c.ReadOnly = []string{dataentryconfig.CalDAVFieldPriority}
	})
	patch, err := m.patchFor(richTodo())
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if v, ok := patch.Properties["urgency"]; ok {
		t.Errorf("read_only priority must cover priority_map, got urgency = %v", v)
	}
}

// TestCalDAVMapper_ReadOnlyClearIsAlsoRefused pins the explicit-clear path.
// An empty DUE is an unset rather than a write, so it takes a different branch
// and would bypass a guard placed only on the value-writing side.
func TestCalDAVMapper_ReadOnlyClearIsAlsoRefused(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
		c.ReadOnly = []string{dataentryconfig.CalDAVFieldDue}
	})
	patch, err := m.patchFor(inbound(calfeed.Todo{Summary: "x"}, ical.PropSummary, ical.PropDue))
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	for _, u := range patch.MetaUnset {
		if u == "due" {
			t.Error("read_only due must refuse an explicit clear, not just a value")
		}
	}
}

// TestCalDAVMapper_CreateIgnoresReadOnly pins the create exemption: read-only
// protects a STORED value, and a create has none. Dropping SUMMARY here would
// create an entity with no title, since that is the one field every client
// sends on a new to-do.
func TestCalDAVMapper_CreateIgnoresReadOnly(t *testing.T) {
	m := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
		c.ReadOnly = dataentryconfig.CalDAVReadOnlyFields
	})
	patch, err := m.createPatch(richTodo())
	if err != nil {
		t.Fatalf("createPatch: %v", err)
	}
	if patch.Properties["title"] != "client title" {
		t.Errorf("a create must keep the client's summary, got %v", patch.Properties["title"])
	}
	if patch.Properties["place"] != "client place" {
		t.Errorf("a create must keep the client's location, got %v", patch.Properties["place"])
	}
}

// TestCalDAVMapper_CreateDoesNotLeakRelaxationToUpdates pins that the create
// exemption is scoped to one call. The mapper is shared across concurrent
// requests, so relaxing it by mutating the receiver would let one create
// silently disable read-only for a parallel update.
func TestCalDAVMapper_CreateDoesNotLeakRelaxationToUpdates(t *testing.T) {
	m := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
		c.ReadOnly = []string{dataentryconfig.CalDAVFieldSummary}
	})
	if _, err := m.createPatch(richTodo()); err != nil {
		t.Fatalf("createPatch: %v", err)
	}
	patch, err := m.patchFor(richTodo())
	if err != nil {
		t.Fatalf("patchFor: %v", err)
	}
	if v, ok := patch.Properties["title"]; ok {
		t.Errorf("read_only leaked away after a create: title = %v", v)
	}
}

// TestCalDAVMapper_DroppedReadOnlySuppressesETag pins the ETag-suppression
// signal, field by field.
//
// RFC 4791 §5.3.4 permits a strong ETag on PUT only when the stored
// representation is octet-equal to the submitted one. A discarded read-only
// field breaks that equality, and withholding the tag is what forces the client
// to re-read and display the value rela actually kept — instead of caching its
// own rejected edit as though it were current.
func TestCalDAVMapper_DroppedReadOnlySuppressesETag(t *testing.T) {
	tests := []struct {
		field string
		sent  []string
	}{
		{dataentryconfig.CalDAVFieldSummary, []string{ical.PropSummary}},
		{dataentryconfig.CalDAVFieldDescription, []string{ical.PropDescription}},
		{dataentryconfig.CalDAVFieldDue, []string{ical.PropDue}},
		{dataentryconfig.CalDAVFieldPriority, []string{ical.PropPriority}},
		{dataentryconfig.CalDAVFieldLocation, []string{ical.PropLocation}},
		{dataentryconfig.CalDAVFieldCategories, []string{ical.PropCategories}},
		{dataentryconfig.CalDAVFieldStart, []string{ical.PropDateTimeStart}},
		{dataentryconfig.CalDAVFieldCompletion, []string{ical.PropStatus}},
		{dataentryconfig.CalDAVFieldCompletion, []string{ical.PropCompleted}},
	}
	for _, tc := range tests {
		t.Run(tc.field+"/"+strings.Join(tc.sent, ","), func(t *testing.T) {
			in := inbound(richTodo().Todo, tc.sent...)

			open := testMapper(t, fullCollection)
			if open.droppedReadOnly(in) {
				t.Fatalf("precondition: %q reported as dropped while writable", tc.field)
			}

			locked := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
				c.ReadOnly = []string{tc.field}
			})
			if !locked.droppedReadOnly(in) {
				t.Errorf("read_only %q was discarded but the ETag would still be served, "+
					"so the client caches a tag for content rela never stored", tc.field)
			}
		})
	}
}

// TestCalDAVMapper_UnsentReadOnlyKeepsETag pins the other half: a read-only
// field the client never mentioned changes nothing about the stored result, so
// the representations still match and the ETag is legitimate.
//
// Suppressing it there would be a silent performance and correctness cost —
// every write would force a re-read, and If-Match would have nothing to compare.
func TestCalDAVMapper_UnsentReadOnlyKeepsETag(t *testing.T) {
	m := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
		c.ReadOnly = dataentryconfig.CalDAVReadOnlyFields
	})
	// The client sent ONLY a summary; every other locked field is unmentioned.
	in := inbound(calfeed.Todo{Summary: "only this"}, ical.PropSummary)
	if !m.droppedReadOnly(in) {
		t.Fatal("precondition: summary is read-only and was sent, so it must count as dropped")
	}

	writableSummary := testMapper(t, fullCollection, func(c *dataentryconfig.CalDAVCollection) {
		c.ReadOnly = []string{dataentryconfig.CalDAVFieldDue, dataentryconfig.CalDAVFieldLocation}
	})
	if writableSummary.droppedReadOnly(in) {
		t.Error("no read-only field was sent, so the stored bytes match the submitted ones " +
			"and the ETag must stand")
	}
}

// TestCalDAVMapper_UnmappedReadOnlyKeepsETag pins that naming a field the
// collection does not map cannot suppress the ETag: there is no mapping, so
// nothing was discarded and the representations still agree.
func TestCalDAVMapper_UnmappedReadOnlyKeepsETag(t *testing.T) {
	m := testMapper(t, func(c *dataentryconfig.CalDAVCollection) {
		c.Location = "" // not mapped
		c.ReadOnly = []string{dataentryconfig.CalDAVFieldLocation}
	})
	if m.droppedReadOnly(inbound(richTodo().Todo, ical.PropLocation)) {
		t.Error("an unmapped field cannot be dropped, so the ETag must stand")
	}
}
