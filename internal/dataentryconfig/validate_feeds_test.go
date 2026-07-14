package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// feedMetamodel returns a metamodel with a date-bearing entity type for feed
// validation tests: `task` has a `due` (date), `starts_at`/`ends_at`
// (datetime), `title` (string, display), and a `status`; `note` has no display
// property.
func feedMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label:           "Task",
				IDPrefix:        "TSK-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":     {Type: metamodel.PropertyTypeString, Required: true},
					"due":       {Type: metamodel.PropertyTypeDate},
					"starts_at": {Type: metamodel.PropertyTypeDatetime},
					"ends_at":   {Type: metamodel.PropertyTypeDatetime},
					"status":    {Type: metamodel.PropertyTypeString},
					"body":      {Type: metamodel.PropertyTypeString},
				},
			},
			"note": {
				Label:    "Note",
				IDPrefix: "NOTE-",
				Properties: map[string]metamodel.PropertyDef{
					"text": {Type: metamodel.PropertyTypeString},
					"on":   {Type: metamodel.PropertyTypeDate},
				},
			},
		},
	}
}

func TestValidateFeeds_Valid(t *testing.T) {
	cfg := &Config{Feeds: map[string]Feed{
		"tasks": {
			Meta: FeedMeta{Name: "PIM tasks", Color: "#C2185B"},
			Sources: []FeedSource{
				{
					EntityType:  "task",
					Where:       []string{"status != done", "due != "},
					Date:        "due",
					Summary:     "title",
					Description: "body",
					Alarm:       "-PT9H",
				},
				{EntityType: "note", Date: "on", Summary: "text"}, // 2nd source (merge/OR)
			},
		},
	}}
	if errs := validateFeeds(cfg, feedMetamodel()); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidateFeeds_SummaryFallsBackToDisplayProperty(t *testing.T) {
	// task has DisplayProperty=title, so omitting summary is fine.
	cfg := &Config{Feeds: map[string]Feed{
		"tasks": {Sources: []FeedSource{{EntityType: "task", Date: "due"}}},
	}}
	if errs := validateFeeds(cfg, feedMetamodel()); len(errs) != 0 {
		t.Fatalf("summary should fall back to display property; got: %v", errs)
	}
}

func TestValidateFeeds_Errors(t *testing.T) {
	tests := []struct {
		name    string
		src     FeedSource
		wantSub string
	}{
		{"unknown entity type", FeedSource{EntityType: "widget", Date: "due"}, "unknown entity type"},
		{"missing date", FeedSource{EntityType: "task"}, "'date' is required"},
		{"unknown date prop", FeedSource{EntityType: "task", Date: "nope"}, "date property \"nope\" not in metamodel"},
		{"date not date-typed", FeedSource{EntityType: "task", Date: "title"}, "must be date- or datetime-typed"},
		{"summary omitted, no display prop", FeedSource{EntityType: "note", Date: "on", Summary: ""}, "no display property"},
		{"unknown summary prop", FeedSource{EntityType: "task", Date: "due", Summary: "nope"}, "summary property \"nope\""},
		{"unknown description prop", FeedSource{EntityType: "task", Date: "due", Description: "nope"}, "description property \"nope\""},
		{"unparseable where", FeedSource{EntityType: "task", Date: "due", Where: []string{"garbage"}}, "where[0]"},
		{"where unknown prop", FeedSource{EntityType: "task", Date: "due", Where: []string{"nope > 1"}}, "unknown property \"nope\""},
		{"bad alarm", FeedSource{EntityType: "task", Date: "due", Alarm: "9 hours"}, "not a valid RFC 5545 duration"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Feeds: map[string]Feed{"f": {Sources: []FeedSource{tc.src}}}}
			errs := validateFeeds(cfg, feedMetamodel())
			if !containsSub(errs, tc.wantSub) {
				t.Errorf("expected an error containing %q, got: %v", tc.wantSub, errs)
			}
		})
	}
}

func TestValidateFeeds_RruleAndEndDate(t *testing.T) {
	valid := []FeedSource{
		{EntityType: "task", Date: "due", Summary: "title", Rrule: "FREQ=DAILY"},           // literal
		{EntityType: "task", Date: "due", Summary: "title", Rrule: "FREQ=WEEKLY;COUNT=10"}, // bounded literal
		{EntityType: "task", Date: "due", Summary: "title", Rrule: "RRULE:FREQ=DAILY"},     // literal w/ prefix
		{EntityType: "note", Date: "on", Summary: "text", Rrule: "on"},                     // property ref (on is date-typed but exists)
		{EntityType: "note", Date: "on", Summary: "text", EndDate: "on"},                   // end_date property
		// Datetime sources (timed events) are now accepted.
		{EntityType: "task", Date: "starts_at", Summary: "title"},                          // datetime start, no end
		{EntityType: "task", Date: "starts_at", Summary: "title", EndDate: "ends_at"},      // datetime start + datetime end (same kind)
	}
	for i, src := range valid {
		cfg := &Config{Feeds: map[string]Feed{"f": {Sources: []FeedSource{src}}}}
		if errs := validateFeeds(cfg, feedMetamodel()); len(errs) != 0 {
			t.Errorf("valid[%d] %+v: unexpected errors: %v", i, src, errs)
		}
	}

	bad := []struct {
		src FeedSource
		sub string
	}{
		{FeedSource{EntityType: "task", Date: "due", Summary: "title", Rrule: "FREQ=NONSENSE"}, "not a valid RFC 5545"},
		{FeedSource{EntityType: "task", Date: "due", Summary: "title", Rrule: "nope"}, "neither a valid RRULE"},
		{FeedSource{EntityType: "task", Date: "due", Summary: "title", EndDate: "nope"}, "end_date property \"nope\""},
		{FeedSource{EntityType: "task", Date: "due", Summary: "title", EndDate: "title"}, "end_date property \"title\" must be date- or datetime-typed"},
		// Start/end kind mismatch is rejected (iCal forbids all-day + timed in one event), both directions.
		{FeedSource{EntityType: "task", Date: "due", Summary: "title", EndDate: "ends_at"}, "must be all-day or timed, not a mix"},
		{FeedSource{EntityType: "task", Date: "starts_at", Summary: "title", EndDate: "due"}, "must be all-day or timed, not a mix"},
	}
	for _, tc := range bad {
		cfg := &Config{Feeds: map[string]Feed{"f": {Sources: []FeedSource{tc.src}}}}
		if !containsSub(validateFeeds(cfg, feedMetamodel()), tc.sub) {
			t.Errorf("expected error containing %q for %+v; got %v", tc.sub, tc.src, validateFeeds(cfg, feedMetamodel()))
		}
	}
}

func TestValidateFeeds_NoSources(t *testing.T) {
	cfg := &Config{Feeds: map[string]Feed{"empty": {}}}
	errs := validateFeeds(cfg, feedMetamodel())
	if !containsSub(errs, "at least one source") {
		t.Errorf("expected 'at least one source' error, got: %v", errs)
	}
}

func TestICalDuration(t *testing.T) {
	valid := func(d string) bool { return icalDurationRe.MatchString(d) && hasDurationComponent(d) }
	for _, d := range []string{"-PT9H", "PT15M", "-P1D", "P1W", "PT1H30M", "-PT30S", "P2DT3H"} {
		if !valid(d) {
			t.Errorf("expected %q to be a valid duration", d)
		}
	}
	for _, d := range []string{"9h", "-9H", "PT", "", "1D", "P", "PTH", "abc"} {
		if valid(d) {
			t.Errorf("expected %q to be an invalid duration", d)
		}
	}
}

func containsSub(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}
