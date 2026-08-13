package calfeed

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// completedAt is the injected completion instant used across VTODO tests.
var completedAt = time.Date(2026, 8, 9, 8, 14, 6, 0, time.UTC)

func TestRenderTodo_Minimal(t *testing.T) {
	body := testICal().RenderTodo(Todo{UID: "task--TKT-1@rela", Summary: "Buy milk"})
	lines := logicalLines(t, body)

	for _, want := range []string{
		"BEGIN:VTODO",
		"UID:task--TKT-1@rela",
		"DTSTAMP:20260704T123000Z",
		"SUMMARY:Buy milk",
		"STATUS:NEEDS-ACTION",
		"END:VTODO",
	} {
		if !containsLine(lines, want) {
			t.Errorf("missing line %q in:\n%s", want, body)
		}
	}
	// A to-do with no deadline emits no DUE at all — that is legal and common.
	for _, l := range lines {
		if strings.HasPrefix(l, "DUE") {
			t.Errorf("unexpected DUE for a to-do with zero Due: %q", l)
		}
		// A pending to-do at 0% would be noise.
		if strings.HasPrefix(l, "PERCENT-COMPLETE") {
			t.Errorf("unexpected PERCENT-COMPLETE on a plain pending to-do: %q", l)
		}
		if strings.HasPrefix(l, "COMPLETED") {
			t.Errorf("unexpected COMPLETED on a pending to-do: %q", l)
		}
	}
}

func TestRenderTodo_DueAllDayAndTimed(t *testing.T) {
	tests := []struct {
		name  string
		todo  Todo
		want  string
		unwnt string
	}{
		{
			name:  "all-day due renders VALUE=DATE",
			todo:  Todo{UID: "u@rela", Summary: "S", Due: day(8, 10)},
			want:  "DUE;VALUE=DATE:20260810",
			unwnt: "DUE:20260810T000000Z",
		},
		{
			name:  "timed due renders a UTC instant",
			todo:  Todo{UID: "u@rela", Summary: "S", Due: dayTime(8, 10, 9, 30), Timed: true},
			want:  "DUE:20260810T093000Z",
			unwnt: "DUE;VALUE=DATE:20260810",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := logicalLines(t, testICal().RenderTodo(tc.todo))
			if !containsLine(lines, tc.want) {
				t.Errorf("want line %q, got:\n%s", tc.want, strings.Join(lines, "\n"))
			}
			if containsLine(lines, tc.unwnt) {
				t.Errorf("unexpected line %q", tc.unwnt)
			}
		})
	}
}

// TestTodo_CompleteSetsTheWholeTrio pins the completion contract. Apple writes
// STATUS + COMPLETED + PERCENT-COMPLETE together, and RFC 4791 §7.8.9's
// "pending to-dos" filter keys on COMPLETED being ABSENT — so a to-do that sets
// only STATUS reads as done in one client and pending in another.
func TestTodo_CompleteSetsTheWholeTrio(t *testing.T) {
	todo := Todo{UID: "u@rela", Summary: "S", Due: day(8, 10)}
	todo.Complete(completedAt)

	if todo.Status != TodoCompleted {
		t.Errorf("Status = %q, want %q", todo.Status, TodoCompleted)
	}
	if !todo.Completed.Equal(completedAt) {
		t.Errorf("Completed = %v, want %v", todo.Completed, completedAt)
	}
	if todo.PercentComplete != 100 {
		t.Errorf("PercentComplete = %d, want 100", todo.PercentComplete)
	}

	lines := logicalLines(t, testICal().RenderTodo(todo))
	for _, want := range []string{
		"STATUS:COMPLETED",
		"COMPLETED:20260809T081406Z",
		"PERCENT-COMPLETE:100",
	} {
		if !containsLine(lines, want) {
			t.Errorf("missing completion line %q in:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

// TestRenderTodo_CompletedIsAlwaysUTCDateTime guards a subtle RFC 5545 rule:
// COMPLETED is an instant, never a DATE, even when DUE is all-day.
func TestRenderTodo_CompletedIsAlwaysUTCDateTime(t *testing.T) {
	todo := Todo{UID: "u@rela", Summary: "S", Due: day(8, 10)} // all-day
	todo.Complete(completedAt)

	lines := logicalLines(t, testICal().RenderTodo(todo))
	if !containsLine(lines, "COMPLETED:20260809T081406Z") {
		t.Errorf("COMPLETED must be a UTC date-time, got:\n%s", strings.Join(lines, "\n"))
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "COMPLETED;VALUE=DATE") {
			t.Errorf("COMPLETED must never be a DATE value: %q", l)
		}
	}
}

func TestRenderTodo_StatusDefaultsAndOptionalProps(t *testing.T) {
	tests := []struct {
		name string
		todo Todo
		want []string
		omit []string
	}{
		{
			name: "zero status defaults to NEEDS-ACTION",
			todo: Todo{UID: "u@rela", Summary: "S"},
			want: []string{"STATUS:NEEDS-ACTION"},
		},
		{
			name: "cancelled status is rendered",
			todo: Todo{UID: "u@rela", Summary: "S", Status: TodoCancelled},
			want: []string{"STATUS:CANCELLED"},
		},
		{
			name: "priority rendered when non-zero",
			todo: Todo{UID: "u@rela", Summary: "S", Priority: 5},
			want: []string{"PRIORITY:5"},
		},
		{
			name: "zero priority omitted",
			todo: Todo{UID: "u@rela", Summary: "S"},
			omit: []string{"PRIORITY:0"},
		},
		{
			name: "in-progress percent rendered without completion",
			todo: Todo{UID: "u@rela", Summary: "S", PercentComplete: 40},
			want: []string{"PERCENT-COMPLETE:40", "STATUS:NEEDS-ACTION"},
			omit: []string{"COMPLETED:"},
		},
		{
			name: "description and url rendered",
			todo: Todo{UID: "u@rela", Summary: "S", Description: "d", URL: "http://x/y"},
			want: []string{"DESCRIPTION:d", "URL:http://x/y"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := logicalLines(t, testICal().RenderTodo(tc.todo))
			for _, w := range tc.want {
				if !containsLine(lines, w) {
					t.Errorf("missing %q in:\n%s", w, strings.Join(lines, "\n"))
				}
			}
			for _, o := range tc.omit {
				for _, l := range lines {
					if strings.HasPrefix(l, o) {
						t.Errorf("unexpected line %q", l)
					}
				}
			}
		})
	}
}

func TestRenderTodo_VALARM(t *testing.T) {
	todo := Todo{
		UID: "u@rela", Summary: "Renew passport", Due: day(8, 10),
		Alarms: []Alarm{{Trigger: "-PT9H"}},
	}
	lines := logicalLines(t, testICal().RenderTodo(todo))
	for _, want := range []string{
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"DESCRIPTION:Renew passport", // defaults to the summary
		"TRIGGER:-PT9H",
		"END:VALARM",
	} {
		if !containsLine(lines, want) {
			t.Errorf("missing %q in:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

// TestRenderTodo_NoLineBreakInjection mirrors the VEVENT guard: a newline in
// user content must never break out into a forged property line.
func TestRenderTodo_NoLineBreakInjection(t *testing.T) {
	todo := Todo{
		UID:         "u@rela",
		Summary:     "evil\r\nSTATUS:COMPLETED",
		Description: "line1\nline2",
	}
	lines := logicalLines(t, testICal().RenderTodo(todo))
	if countLine(lines, "STATUS:COMPLETED") != 0 {
		t.Errorf("injected STATUS line survived:\n%s", strings.Join(lines, "\n"))
	}
	if countLine(lines, "STATUS:NEEDS-ACTION") != 1 {
		t.Errorf("want exactly one real STATUS line, got:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRenderTodo_StableAcrossRenders(t *testing.T) {
	todo := Todo{UID: "u@rela", Summary: "S", Due: day(8, 10)}
	a := testICal().RenderTodo(todo)
	b := testICal().RenderTodo(todo)
	if !bytes.Equal(a, b) {
		t.Error("RenderTodo is not deterministic for a fixed clock")
	}
}

func TestTodoETag_StableAndSensitive(t *testing.T) {
	base := Todo{UID: "u@rela", Summary: "S", Due: day(8, 10)}
	done := base
	done.Complete(completedAt)

	ic := testICal()
	first, second := ic.TodoETag(base), ic.TodoETag(base)
	if first != second {
		t.Error("TodoETag is not stable for identical content")
	}
	if ic.TodoETag(base) == ic.TodoETag(done) {
		t.Error("TodoETag must change when completion state changes")
	}

	// Independent of the injected clock, so DTSTAMP churn does not move it.
	later := ICal{ProdID: ic.ProdID, Now: fixedNow.Add(72 * time.Hour)}
	if ic.TodoETag(base) != later.TodoETag(base) {
		t.Error("TodoETag must not depend on ICal.Now")
	}
}

func TestRenderCollection_TodoComponent(t *testing.T) {
	f := Feed{
		Name:      "rela Tasks",
		Component: ComponentTodo,
		Todos: []Todo{
			{UID: "task--A@rela", Summary: "A"},
			{UID: "task--B@rela", Summary: "B"},
		},
	}
	body := testICal().RenderCollection(f)
	lines := logicalLines(t, body)

	if got := countLine(lines, "BEGIN:VTODO"); got != 2 {
		t.Errorf("BEGIN:VTODO count = %d, want 2", got)
	}
	if got := countLine(lines, "BEGIN:VEVENT"); got != 0 {
		t.Errorf("BEGIN:VEVENT count = %d, want 0 in a VTODO feed", got)
	}
	if !containsLine(lines, "X-WR-CALNAME:rela Tasks") {
		t.Error("missing calendar name")
	}
	// The envelope is shared with the event path.
	for _, want := range []string{"BEGIN:VCALENDAR", "VERSION:2.0", "END:VCALENDAR"} {
		if !containsLine(lines, want) {
			t.Errorf("missing envelope line %q", want)
		}
	}
}

// TestRenderCollection_ComponentSelectsOneSlice pins the no-mixing rule. Apple
// segregates by component set — Reminders binds only to a VTODO collection and
// Calendar.app makes its own VEVENT one — so a feed must emit exactly one kind
// even when both slices are populated.
func TestRenderCollection_ComponentSelectsOneSlice(t *testing.T) {
	both := Feed{
		Events: []Event{{UID: "e@rela", Summary: "E", Start: day(8, 1)}},
		Todos:  []Todo{{UID: "t@rela", Summary: "T"}},
	}

	eventLines := logicalLines(t, testICal().RenderCollection(both))
	if countLine(eventLines, "BEGIN:VEVENT") != 1 || countLine(eventLines, "BEGIN:VTODO") != 0 {
		t.Errorf("default component must emit only VEVENT, got:\n%s", strings.Join(eventLines, "\n"))
	}

	both.Component = ComponentTodo
	todoLines := logicalLines(t, testICal().RenderCollection(both))
	if countLine(todoLines, "BEGIN:VTODO") != 1 || countLine(todoLines, "BEGIN:VEVENT") != 0 {
		t.Errorf("VTODO component must emit only VTODO, got:\n%s", strings.Join(todoLines, "\n"))
	}
}

func TestRenderCollection_IsWrappedRenderTodo(t *testing.T) {
	todo := Todo{UID: "task--A@rela", Summary: "A", Due: day(8, 10)}
	f := Feed{Component: ComponentTodo, Todos: []Todo{todo}}

	ic := testICal()
	if !bytes.Contains(ic.RenderCollection(f), ic.RenderTodo(todo)) {
		t.Error("RenderCollection must embed RenderTodo output verbatim")
	}
}

func TestCollectionTag_TodoSensitive(t *testing.T) {
	base := Feed{Component: ComponentTodo, Todos: []Todo{
		{UID: "a@rela", Summary: "A"},
		{UID: "b@rela", Summary: "B"},
	}}

	ic := testICal()
	first, second := ic.CollectionTag(base), ic.CollectionTag(base)
	if first != second {
		t.Error("CollectionTag is not stable for identical content")
	}

	// Completing a member must move the collection tag, or a polling client
	// never learns the box was ticked.
	completed := Feed{Component: ComponentTodo, Todos: []Todo{
		base.Todos[0], base.Todos[1],
	}}
	completed.Todos[1].Complete(completedAt)
	if ic.CollectionTag(base) == ic.CollectionTag(completed) {
		t.Error("CollectionTag must change when a to-do is completed")
	}

	// Removing a member must move it too.
	fewer := Feed{Component: ComponentTodo, Todos: base.Todos[:1]}
	if ic.CollectionTag(base) == ic.CollectionTag(fewer) {
		t.Error("CollectionTag must change when a to-do is removed")
	}
}

// TestCollectionTag_ComponentScoped guards against the tag being computed over
// the wrong slice: a VTODO feed's tag must not be the empty-events tag.
func TestCollectionTag_ComponentScoped(t *testing.T) {
	ic := testICal()
	empty := Feed{Component: ComponentTodo}
	populated := Feed{Component: ComponentTodo, Todos: []Todo{{UID: "a@rela", Summary: "A"}}}

	if ic.CollectionTag(empty) == ic.CollectionTag(populated) {
		t.Error("CollectionTag must reflect the Todos slice for a VTODO feed")
	}
}

// atMostOnceVTODO are the VTODO properties RFC 5545 §3.6.2 permits at most once
// per component. Duplicates are a classic serializer defect and clients resolve
// them inconsistently (first-wins vs last-wins), so this is asserted directly
// rather than left to the presence checks the other tests use.
var atMostOnceVTODO = []string{
	"UID", "DTSTAMP", "SUMMARY", "STATUS", "DUE",
	"COMPLETED", "PRIORITY", "PERCENT-COMPLETE", "URL",
}

func TestRenderTodo_PropertyCardinality(t *testing.T) {
	completed := Todo{UID: "u@rela", Summary: "S", Due: day(8, 10), Priority: 5}
	completed.Complete(completedAt)

	tests := []struct {
		name string
		todo Todo
	}{
		{"minimal", Todo{UID: "u@rela", Summary: "S"}},
		{"all fields set", func() Todo {
			td := completed
			td.Description = "d"
			td.URL = "http://x/y"
			td.Alarms = []Alarm{{Trigger: "-PT1H"}, {Trigger: "-PT2H"}}
			return td
		}()},
		{"timed due", Todo{UID: "u@rela", Summary: "S", Due: dayTime(8, 10, 9, 0), Timed: true}},
		{"in progress", Todo{UID: "u@rela", Summary: "S", PercentComplete: 40}},
		{"cancelled", Todo{UID: "u@rela", Summary: "S", Status: TodoCancelled}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Count only VTODO-level lines: a VALARM legitimately carries its
			// own DESCRIPTION, so nested blocks are excluded from the tally.
			var depth int
			counts := map[string]int{}
			for _, l := range logicalLines(t, testICal().RenderTodo(tc.todo)) {
				switch {
				case strings.HasPrefix(l, "BEGIN:VALARM"):
					depth++
					continue
				case strings.HasPrefix(l, "END:VALARM"):
					depth--
					continue
				}
				if depth > 0 {
					continue
				}
				name, _, _ := strings.Cut(l, ":")
				name, _, _ = strings.Cut(name, ";")
				counts[name]++
			}
			for _, prop := range atMostOnceVTODO {
				if counts[prop] > 1 {
					t.Errorf("%s appears %d times, RFC 5545 permits at most once", prop, counts[prop])
				}
			}
		})
	}
}

// TestTodoETag_SensitiveToEveryField guards the CalDAV conditional-request
// contract: a field the ETag ignores is a field whose edit a client never
// learns about, leaving a permanently stale entry that polling cannot fix.
func TestTodoETag_SensitiveToEveryField(t *testing.T) {
	base := Todo{
		UID: "u@rela", Summary: "S", Description: "d", URL: "http://x/y",
		Due: day(8, 10), Priority: 5, PercentComplete: 40,
		Alarms: []Alarm{{Trigger: "-PT9H"}},
	}

	tests := []struct {
		field  string
		mutate func(*Todo)
	}{
		{"UID", func(td *Todo) { td.UID = "other@rela" }},
		{"Summary", func(td *Todo) { td.Summary = "changed" }},
		{"Description", func(td *Todo) { td.Description = "changed" }},
		{"URL", func(td *Todo) { td.URL = "http://x/z" }},
		{"Due", func(td *Todo) { td.Due = day(8, 11) }},
		{"Timed", func(td *Todo) { td.Timed = true }},
		{"Status", func(td *Todo) { td.Status = TodoCancelled }},
		{"Completed", func(td *Todo) { td.Complete(completedAt) }},
		{"PercentComplete", func(td *Todo) { td.PercentComplete = 60 }},
		{"Priority", func(td *Todo) { td.Priority = 1 }},
		{"Alarms", func(td *Todo) { td.Alarms = []Alarm{{Trigger: "-PT1H"}} }},
	}

	ic := testICal()
	want := ic.TodoETag(base)
	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			mutated := base
			tc.mutate(&mutated)
			if ic.TodoETag(mutated) == want {
				t.Errorf("TodoETag unchanged after mutating %s — a client would never see this edit", tc.field)
			}
		})
	}
}

// TestRenderTodo_NormalizesCompletionTrio pins the invariant the exported
// fields cannot enforce: neither half of the completion state can reach a
// client alone. RFC 4791 §7.8.9's pending filter keys on COMPLETED while UIs
// read STATUS, so a half-set to-do would read done in one client and pending
// in another.
func TestRenderTodo_NormalizesCompletionTrio(t *testing.T) {
	tests := []struct {
		name    string
		todo    Todo
		want    []string
		notWant []string
	}{
		{
			name:    "STATUS:COMPLETED without a timestamp is demoted, not invented",
			todo:    Todo{UID: "u@rela", Summary: "S", Status: TodoCompleted},
			want:    []string{"STATUS:NEEDS-ACTION"},
			notWant: []string{"STATUS:COMPLETED", "COMPLETED:"},
		},
		{
			name: "a COMPLETED timestamp promotes STATUS and percent",
			todo: Todo{UID: "u@rela", Summary: "S", Completed: completedAt},
			want: []string{"STATUS:COMPLETED", "COMPLETED:20260809T081406Z", "PERCENT-COMPLETE:100"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := logicalLines(t, testICal().RenderTodo(tc.todo))
			for _, w := range tc.want {
				if !containsLine(lines, w) {
					t.Errorf("missing %q in:\n%s", w, strings.Join(lines, "\n"))
				}
			}
			for _, nw := range tc.notWant {
				for _, l := range lines {
					if strings.HasPrefix(l, nw) {
						t.Errorf("unexpected %q", l)
					}
				}
			}
		})
	}
}

func TestRenderTodo_ClampsOutOfRangeValues(t *testing.T) {
	tests := []struct {
		name       string
		todo       Todo
		wantLine   string
		unwantable string
	}{
		{"priority above range", Todo{UID: "u", Summary: "S", Priority: 999}, "PRIORITY:9", "PRIORITY:999"},
		{"percent above range", Todo{UID: "u", Summary: "S", PercentComplete: 150}, "PERCENT-COMPLETE:100", "PERCENT-COMPLETE:150"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := logicalLines(t, testICal().RenderTodo(tc.todo))
			if !containsLine(lines, tc.wantLine) {
				t.Errorf("want %q in:\n%s", tc.wantLine, strings.Join(lines, "\n"))
			}
			if containsLine(lines, tc.unwantable) {
				t.Errorf("out-of-range value %q reached the output", tc.unwantable)
			}
		})
	}

	// Below-range values clamp to 0, which is the "no information" value for
	// both properties — so the property is omitted entirely rather than
	// emitting a meaningless PRIORITY:0 / PERCENT-COMPLETE:0.
	for _, tc := range []struct {
		name string
		todo Todo
		prop string
	}{
		{"negative priority", Todo{UID: "u", Summary: "S", Priority: -3}, "PRIORITY"},
		{"negative percent", Todo{UID: "u", Summary: "S", PercentComplete: -5}, "PERCENT-COMPLETE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, l := range logicalLines(t, testICal().RenderTodo(tc.todo)) {
				if strings.HasPrefix(l, tc.prop) {
					t.Errorf("below-range value should clamp to 0 and be omitted, got %q", l)
				}
			}
		})
	}
}

// TestRenderTodo_RejectsInvalidTypedValues covers the fields written WITHOUT
// escaping because they are typed values rather than TEXT: STATUS (enumerated)
// and TRIGGER (a duration). They are validated instead, so caller content
// cannot reach an unescaped line.
func TestRenderTodo_RejectsInvalidTypedValues(t *testing.T) {
	t.Run("unknown status falls back to NEEDS-ACTION", func(t *testing.T) {
		lines := logicalLines(t, testICal().RenderTodo(
			Todo{UID: "u", Summary: "S", Status: TodoStatus("BOGUS;PARAM=x")}))
		if !containsLine(lines, "STATUS:NEEDS-ACTION") {
			t.Errorf("want the NEEDS-ACTION fallback, got:\n%s", strings.Join(lines, "\n"))
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "STATUS:") && l != "STATUS:NEEDS-ACTION" {
				t.Errorf("unrecognized status reached the output: %q", l)
			}
		}
	})

	t.Run("invalid trigger drops its alarm but keeps valid ones", func(t *testing.T) {
		lines := logicalLines(t, testICal().RenderTodo(Todo{
			UID: "u", Summary: "S", Due: day(8, 10),
			Alarms: []Alarm{{Trigger: "-PT9H;EVIL=1"}, {Trigger: "not-a-duration"}, {Trigger: "-PT1H"}},
		}))
		if got := countLine(lines, "BEGIN:VALARM"); got != 1 {
			t.Errorf("VALARM count = %d, want 1 (only the valid trigger)", got)
		}
		if !containsLine(lines, "TRIGGER:-PT1H") {
			t.Error("the valid trigger should survive")
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "TRIGGER:") && l != "TRIGGER:-PT1H" {
				t.Errorf("invalid trigger reached the output: %q", l)
			}
		}
	})

	t.Run("all VTODO status values are accepted", func(t *testing.T) {
		for _, s := range []TodoStatus{TodoNeedsAction, TodoCompleted, TodoInProcess, TodoCancelled} {
			todo := Todo{UID: "u", Summary: "S", Status: s}
			if s == TodoCompleted {
				todo.Complete(completedAt) // avoid the demotion path
			}
			lines := logicalLines(t, testICal().RenderTodo(todo))
			if !containsLine(lines, "STATUS:"+string(s)) {
				t.Errorf("status %q was not rendered", s)
			}
		}
	})
}

// TestRenderTodo_AlarmRequiresAnchor documents an RFC 5545 subtlety: a relative
// TRIGGER anchors to DTSTART or DUE, and this renderer never emits DTSTART, so
// an alarm on a to-do with no DUE has nothing to anchor to.
func TestRenderTodo_AlarmRequiresAnchor(t *testing.T) {
	lines := logicalLines(t, testICal().RenderTodo(
		Todo{UID: "u", Summary: "S", Alarms: []Alarm{{Trigger: "-PT9H"}}}))
	if containsLine(lines, "BEGIN:VALARM") {
		t.Errorf("an alarm with no DUE to anchor to must not be emitted:\n%s", strings.Join(lines, "\n"))
	}
}
