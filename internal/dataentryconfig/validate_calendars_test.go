package dataentryconfig

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// calendarMetamodel extends the feed fixture with the shapes a calendar has to
// reject: a multi-valued date, a custom-format date, and a second entity type
// so multi-source behavior can be exercised.
func calendarMetamodel() *metamodel.Metamodel {
	meta := feedMetamodel()
	task := meta.Entities["task"]
	task.Properties["due_list"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeDate, List: true}
	task.Properties["due_custom"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeDate, Format: "02/01/2006"}
	task.Properties["priority"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	meta.Entities["task"] = task

	meta.Entities["meeting"] = metamodel.EntityDef{
		Label:           "Meeting",
		IDPrefix:        "MTG-",
		DisplayProperty: "name",
		Properties: map[string]metamodel.PropertyDef{
			"name":      {Type: metamodel.PropertyTypeString},
			"starts_at": {Type: metamodel.PropertyTypeDatetime},
			"room":      {Type: metamodel.PropertyTypeString},
		},
	}
	return meta
}

// calendarCfg builds a Config carrying one calendar under the key "cal".
func calendarCfg(cal Calendar) *Config {
	return &Config{Calendars: map[string]Calendar{"cal": cal}}
}

// validCalendar is a minimal calendar that must pass validation, so each test
// case can introduce exactly one defect.
func validCalendar() Calendar {
	return Calendar{
		Title:   "Schedule",
		Sources: []CalendarSource{{EntityType: "task", Date: "due", Summary: "title"}},
	}
}

func TestValidateCalendars_Valid(t *testing.T) {
	meta := calendarMetamodel()
	tests := []struct {
		name string
		cal  Calendar
	}{
		{"minimal", validCalendar()},
		{"datetime source with end", Calendar{Sources: []CalendarSource{
			{EntityType: "task", Date: "starts_at", EndDate: "ends_at", Summary: "title"},
		}}},
		{"summary omitted falls back to display property", Calendar{Sources: []CalendarSource{
			{EntityType: "task", Date: "due"},
		}}},
		{"multiple sources of different types", Calendar{Sources: []CalendarSource{
			{EntityType: "task", Date: "due", Summary: "title"},
			{EntityType: "meeting", Date: "starts_at", Summary: "name"},
		}}},
		{"all optional settings", Calendar{
			DefaultView: "week", WeekStart: "sunday",
			DayStart: "09:00", DayEnd: "17:30", MaxEventsPerDay: 6,
			Sources: []CalendarSource{{
				EntityType: "task", Date: "due", Summary: "title",
				Description: "body", Color: "blue", MaxSpan: 14,
				Where: []string{"status != done"},
			}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errs := validateCalendars(calendarCfg(tt.cal), meta); len(errs) > 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

// TestValidateCalendars_Invalid pins every rejection path. Each case carries
// one defect and asserts the message names it, so a regression that drops a
// check fails loudly rather than silently widening what is accepted.
func TestValidateCalendars_Invalid(t *testing.T) {
	meta := calendarMetamodel()
	tests := []struct {
		name string
		cal  Calendar
		want string
	}{
		{
			name: "no sources",
			cal:  Calendar{Title: "Empty"},
			want: "must declare at least one source",
		},
		{
			name: "unknown entity type",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "ghost", Date: "due"}}},
			want: `unknown entity type "ghost"`,
		},
		{
			name: "date missing",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Summary: "title"}}},
			want: "'date' is required",
		},
		{
			name: "date property not in metamodel",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "nope"}}},
			want: `date property "nope" not in metamodel`,
		},
		{
			name: "date property wrong type",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "status"}}},
			want: "must be date- or datetime-typed",
		},
		{
			name: "date property is a list",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due_list"}}},
			want: "is a list; a calendar needs a single date per entity",
		},
		{
			name: "date property has an unwritable custom format",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due_custom"}}},
			want: "which a calendar cannot write back",
		},
		{
			name: "end_date property not in metamodel",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due", EndDate: "nope"}}},
			want: `end_date property "nope" not in metamodel`,
		},
		{
			name: "date and end_date kinds mismatch",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due", EndDate: "ends_at"}}},
			want: "must be all-day or timed, not a mix",
		},
		{
			name: "summary omitted and no display property",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "note", Date: "on"}}},
			want: "no display property to fall back to",
		},
		{
			name: "summary property unknown",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due", Summary: "nope"}}},
			want: `summary property "nope" not in metamodel`,
		},
		{
			name: "description property unknown",
			cal:  Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due", Description: "nope"}}},
			want: `description property "nope" not in metamodel`,
		},
		{
			name: "where references unknown property",
			cal: Calendar{Sources: []CalendarSource{
				{EntityType: "task", Date: "due", Where: []string{"nope = x"}},
			}},
			want: `where[0] references unknown property "nope"`,
		},
		{
			name: "unknown color token",
			cal: Calendar{Sources: []CalendarSource{
				{EntityType: "task", Date: "due", Color: "#ff0000"},
			}},
			want: `unknown color "#ff0000"`,
		},
		{
			name: "negative max_span",
			cal: Calendar{Sources: []CalendarSource{
				{EntityType: "task", Date: "due", MaxSpan: -1},
			}},
			want: "max_span must not be negative",
		},
		{
			name: "invalid default_view",
			cal:  withShell(func(c *Calendar) { c.DefaultView = "day" }),
			want: `default_view "day" is not valid`,
		},
		{
			name: "invalid week_start",
			cal:  withShell(func(c *Calendar) { c.WeekStart = "tuesday" }),
			want: `week_start "tuesday" is not valid`,
		},
		{
			name: "malformed day_start",
			cal:  withShell(func(c *Calendar) { c.DayStart = "8am" }),
			want: `day_start "8am" is not a HH:MM time`,
		},
		{
			name: "day_start not before day_end",
			cal: withShell(func(c *Calendar) {
				c.DayStart = "18:00"
				c.DayEnd = "09:00"
			}),
			want: "must be before day_end",
		},
		{
			name: "negative max_events_per_day",
			cal:  withShell(func(c *Calendar) { c.MaxEventsPerDay = -1 }),
			want: "max_events_per_day must not be negative",
		},
		{
			name: "event field on no source type",
			cal: withShell(func(c *Calendar) {
				c.Event = CalendarEvent{Fields: []KanbanCardField{{Property: "nope"}}}
			}),
			want: `property "nope" is not on any of this calendar's entity types`,
		},
		{
			name: "event field named title on a type without a title property",
			cal: Calendar{
				// `meeting` has `name`, not `title`. The SPA renders a chip
				// field from entity.properties[name], so accepting this would
				// render nothing forever with no diagnostic.
				Sources: []CalendarSource{{EntityType: "meeting", Date: "starts_at", Summary: "name"}},
				Event:   CalendarEvent{Fields: []KanbanCardField{{Property: "title"}}},
			},
			want: `property "title" is not on any of this calendar's entity types`,
		},
		{
			name: "event field with neither property nor relation",
			cal: withShell(func(c *Calendar) {
				c.Event = CalendarEvent{Fields: []KanbanCardField{{Label: "x"}}}
			}),
			want: "must specify either property or relation",
		},
		{
			name: "event field unknown relation",
			cal: withShell(func(c *Calendar) {
				c.Event = CalendarEvent{Fields: []KanbanCardField{{Relation: "nope"}}}
			}),
			want: `references unknown relation "nope"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCalendars(calendarCfg(tt.cal), meta)
			if !containsSubstring(errs, tt.want) {
				t.Errorf("expected an error containing %q, got: %v", tt.want, errs)
			}
		})
	}
}

// withShell returns a valid calendar mutated by fn, for cases that test
// calendar-level settings rather than source fields.
func withShell(fn func(*Calendar)) Calendar {
	cal := validCalendar()
	fn(&cal)
	return cal
}

// TestValidateCalendars_EventFieldOnOneSourceOnly documents the deliberate
// asymmetry with kanban: a chip field only has to resolve on ONE source type,
// because sources are heterogeneous by design.
func TestValidateCalendars_EventFieldOnOneSourceOnly(t *testing.T) {
	meta := calendarMetamodel()
	cal := Calendar{
		Sources: []CalendarSource{
			{EntityType: "task", Date: "due", Summary: "title"},
			{EntityType: "meeting", Date: "starts_at", Summary: "name"},
		},
		// `room` exists on meeting but not on task; `priority` vice versa.
		Event: CalendarEvent{Fields: []KanbanCardField{{Property: "room"}, {Property: "priority"}}},
	}
	if errs := validateCalendars(calendarCfg(cal), meta); len(errs) > 0 {
		t.Errorf("a field resolving on one source must be accepted, got: %v", errs)
	}
}

// TestValidateCalendars_FormReferences covers edit_form/create_form, which are
// checked against the config rather than the metamodel.
func TestValidateCalendars_FormReferences(t *testing.T) {
	meta := calendarMetamodel()
	cfg := calendarCfg(withShell(func(c *Calendar) {
		c.EditForm = "missing_edit"
		c.CreateForm = "missing_create"
	}))
	errs := validateCalendars(cfg, meta)
	for _, want := range []string{
		`references unknown form "missing_edit" in edit_form`,
		`references unknown form "missing_create" in create_form`,
	} {
		if !containsSubstring(errs, want) {
			t.Errorf("expected an error containing %q, got: %v", want, errs)
		}
	}

	cfg.Forms = map[string]Form{"missing_edit": {}, "missing_create": {}}
	if errs := validateCalendars(cfg, meta); len(errs) > 0 {
		t.Errorf("forms that exist must validate, got: %v", errs)
	}
}

// TestNormalizeCalendars checks defaults are applied and authored values are
// left alone.
func TestNormalizeCalendars(t *testing.T) {
	cfg := calendarCfg(Calendar{Sources: []CalendarSource{{EntityType: "task", Date: "due"}}})
	NormalizeCalendars(cfg)
	got := cfg.Calendars["cal"]

	if got.DefaultView != "month" || got.WeekStart != "monday" {
		t.Errorf("view/week defaults = %q/%q, want month/monday", got.DefaultView, got.WeekStart)
	}
	if got.DayStart != "08:00" || got.DayEnd != "20:00" {
		t.Errorf("hour defaults = %q/%q, want 08:00/20:00", got.DayStart, got.DayEnd)
	}
	if got.MaxEventsPerDay != 4 {
		t.Errorf("max_events_per_day = %d, want 4", got.MaxEventsPerDay)
	}
	if got.Sources[0].MaxSpan != 31 {
		t.Errorf("max_span = %d, want 31", got.Sources[0].MaxSpan)
	}

	authored := calendarCfg(Calendar{
		DefaultView: "week", WeekStart: "sunday", DayStart: "06:00", DayEnd: "22:00",
		MaxEventsPerDay: 9,
		Sources:         []CalendarSource{{EntityType: "task", Date: "due", MaxSpan: 7}},
	})
	NormalizeCalendars(authored)
	if got := authored.Calendars["cal"]; got.DefaultView != "week" || got.WeekStart != "sunday" ||
		got.DayStart != "06:00" || got.DayEnd != "22:00" || got.MaxEventsPerDay != 9 ||
		got.Sources[0].MaxSpan != 7 {

		t.Errorf("normalization overwrote authored values: %+v", got)
	}
}

func TestParseClockMinutes(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{

		{"00:00", 0, true},
		{"08:00", 480, true},
		{"23:59", 1439, true},
		// A single-digit hour is what people actually type; rejecting it was an
		// artifact of validating the format by counting characters.
		{"8:00", 480, true},
		{"24:00", 0, false},
		{"08:60", 0, false},
		{"0800", 0, false},
		{"", 0, false},
		{"aa:bb", 0, false},
		// Atoi accepts a sign, so the hand-rolled parser used to read "+8:00"
		// as 08:00. time.Parse does not.
		{"+8:00", 0, false},
		{"08:00:00", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := parseClockMinutes(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("parseClockMinutes(%q) = %d,%v want %d,%v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestCheckUnknownKeys_AnchorHolder covers the key that makes the documented
// calendar/feed reuse pattern work.
//
// YAML resolves anchors at parse time, so a shared source list has to be
// DEFINED somewhere in the document — and every real top-level key is already
// claimed by a config section. Strict key checking (which is what makes a
// config typo loud) would otherwise reject the only place an author can put
// one. An underscore prefix marks the intent explicitly rather than the
// validator trying to infer which unknown keys are "probably anchors".
func TestCheckUnknownKeys_AnchorHolder(t *testing.T) {
	shared := []byte(`
_task_events: &task_events
  - entity_type: task
    date: due_date
feeds:
  tasks:
    sources: *task_events
calendars:
  schedule:
    sources: *task_events
`)
	if errs := checkUnknownKeys(shared); len(errs) > 0 {
		t.Errorf("an underscore-prefixed anchor holder must be accepted, got: %v", errs)
	}

	// A genuine typo must still be caught: the escape hatch is opt-in, not a
	// hole in the check.
	if errs := checkUnknownKeys([]byte("kanban:\n  board: {}\n")); len(errs) == 0 {
		t.Error("a mistyped top-level key must still be rejected")
	}
	if errs := checkUnknownKeys([]byte("nonsense:\n  x: 1\n")); len(errs) == 0 {
		t.Error("an unknown top-level key must still be rejected")
	}
}

// TestCalendarFeedAnchorSharing pins the reuse mechanism end to end: one
// anchored source list parses into BOTH a feed source and a calendar source,
// including when it carries a key the other side does not know.
func TestCalendarFeedAnchorSharing(t *testing.T) {
	var cfg Config
	yamlSrc := []byte(`
_events: &events
  - entity_type: task
    where: ["status != done"]
    date: due_date
    summary: title
    color: blue
feeds:
  tasks:
    sources: *events
calendars:
  schedule:
    sources: *events
`)
	if err := yaml.Unmarshal(yamlSrc, &cfg); err != nil {
		t.Fatalf("shared anchor failed to parse: %v", err)
	}

	feedSrc := cfg.Feeds["tasks"].Sources
	if len(feedSrc) != 1 || feedSrc[0].EntityType != "task" || feedSrc[0].Date != "due_date" {
		t.Errorf("feed source did not take the anchor: %+v", feedSrc)
	}
	calSrc := cfg.Calendars["schedule"].Sources
	if len(calSrc) != 1 || calSrc[0].EntityType != "task" || calSrc[0].Date != "due_date" {
		t.Errorf("calendar source did not take the anchor: %+v", calSrc)
	}
	// `color` is calendar-only; the feed ignores it rather than failing, which
	// is what lets the two structs diverge without breaking shared anchors.
	if calSrc[0].Color != "blue" {
		t.Errorf("calendar-only key lost: %+v", calSrc[0])
	}
}

// TestValidateCalendars_IDFieldExempt confirms `id` stays accepted: it is an
// entity-level key rather than a metamodel property, so requiring it to appear
// in Properties would reject a legitimate chip field.
func TestValidateCalendars_IDFieldExempt(t *testing.T) {
	meta := calendarMetamodel()
	cal := withShell(func(c *Calendar) {
		c.Event = CalendarEvent{Fields: []KanbanCardField{{Property: "id"}}}
	})
	if errs := validateCalendars(calendarCfg(cal), meta); len(errs) > 0 {
		t.Errorf("an `id` chip field must be accepted, got: %v", errs)
	}
}

// TestValidateCalendars_TitleAcceptedWhenReal is the other half of the `title`
// rule: a type that genuinely HAS a title property may name it.
func TestValidateCalendars_TitleAcceptedWhenReal(t *testing.T) {
	meta := calendarMetamodel()
	cal := withShell(func(c *Calendar) {
		// The default source is `task`, which does have `title`.
		c.Event = CalendarEvent{Fields: []KanbanCardField{{Property: "title"}}}
	})
	if errs := validateCalendars(calendarCfg(cal), meta); len(errs) > 0 {
		t.Errorf("a real title property must be accepted, got: %v", errs)
	}
}

// TestCheckUnknownKeys_UnderscoreDoesNotShadowSection pins the narrowing of the
// anchor escape hatch: `_kanbans:` is much more likely to be a section someone
// disabled by prefixing it than an anchor holder, and silently accepting it
// would defeat the very check the hatch is an exception to.
func TestCheckUnknownKeys_UnderscoreDoesNotShadowSection(t *testing.T) {
	if errs := checkUnknownKeys([]byte("_kanbans:\n  board: {}\n")); len(errs) == 0 {
		t.Error("an underscore-prefixed REAL section name must still be reported")
	}
	// A name that shadows nothing stays a valid anchor holder.
	if errs := checkUnknownKeys([]byte("_shared_sources:\n  - entity_type: task\n")); len(errs) > 0 {
		t.Errorf("a non-shadowing anchor holder must be accepted, got: %v", errs)
	}
}
