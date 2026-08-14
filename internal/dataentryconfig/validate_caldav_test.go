package dataentryconfig

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// caldavMetamodel returns a metamodel shaped like a real PIM project: `task`
// has an enum-typed status (via a named custom type, the usual shape), a
// required title, a date due, a datetime completion stamp and an integer
// priority. `strict` additionally has a required property that NOTHING can
// supply from a CalDAV create — the constructibility case.
func caldavMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Types: map[string]metamodel.CustomType{
			"task_status": {Values: []string{"todo", "doing", "done"}, Default: "todo"},
		},
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label:           "Task",
				IDPrefix:        "TSK-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":        {Type: metamodel.PropertyTypeString, Required: true},
					"due":          {Type: metamodel.PropertyTypeDate},
					"starts_at":    {Type: metamodel.PropertyTypeDatetime},
					"status":       {Type: "task_status", Required: true},
					"completed_at": {Type: metamodel.PropertyTypeDatetime},
					"notes":        {Type: metamodel.PropertyTypeString},
					"rank":         {Type: metamodel.PropertyTypeInteger},
				},
			},
			// Required `owner` with no default and no mapping source.
			"strict": {
				Label:           "Strict",
				IDPrefix:        "STR-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString, Required: true},
					"owner":  {Type: metamodel.PropertyTypeString, Required: true},
					"status": {Type: "task_status"},
				},
			},
			"note": {
				Label:      "Note",
				IDPrefix:   "NOTE-",
				Properties: map[string]metamodel.PropertyDef{"text": {Type: metamodel.PropertyTypeString}},
			},
		},
	}
}

// validTaskCollection is the canonical well-formed collection the negative
// cases mutate, so each test states only what it is exercising.
func validTaskCollection() CalDAVCollection {
	return CalDAVCollection{
		Meta:       FeedMeta{Name: "rela Tasks", Color: "#C2185B"},
		Component:  CalDAVComponentTodo,
		EntityType: "task",
		// Deliberately NOT "status != done": that excludes the completed value
		// and is now rejected — see TestValidateCalDAV_CompletionReachable.
		Where:       []string{"status != cancelled"},
		Due:         "due",
		Summary:     "title",
		Description: "notes",
		Priority:    "rank",
		Completion: &CalDAVCompletion{
			StatusProperty: "status",
			CompletedValue: "done",
			PendingValue:   "todo",
			CompletedAt:    "completed_at",
		},
		OnDelete: &CalDAVOnDelete{Set: map[string]string{"status": "done"}},
	}
}

func TestValidateCalDAV_Valid(t *testing.T) {
	cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": validTaskCollection()}}}
	if errs := validateCalDAV(cfg, caldavMetamodel()); len(errs) > 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// TestValidateCalDAV_MultipleCollections is the shape that replaces the feeds
// package's multi-source list: one collection per entity type, all reachable
// from one account URL.
func TestValidateCalDAV_MultipleCollections(t *testing.T) {
	tasks := validTaskCollection()
	notes := CalDAVCollection{
		EntityType: "note",
		Summary:    "text",
		Completion: &CalDAVCompletion{
			StatusProperty: "text", // not enum-constrained, so any value is legal
			CompletedValue: "done",
			PendingValue:   "open",
		},
	}
	cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": tasks, "notes": notes}}}
	if errs := validateCalDAV(cfg, caldavMetamodel()); len(errs) > 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateCalDAV_Errors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CalDAVCollection)
		wantSub string
	}{
		{
			name:    "unknown entity type",
			mutate:  func(c *CalDAVCollection) { c.EntityType = "nope" },
			wantSub: `unknown entity type "nope"`,
		},
		{
			name:    "missing entity type",
			mutate:  func(c *CalDAVCollection) { c.EntityType = "" },
			wantSub: "'entity_type' is required",
		},
		{
			name:    "unknown component",
			mutate:  func(c *CalDAVCollection) { c.Component = "vjournal" },
			wantSub: `unknown component "vjournal"`,
		},
		{
			name:    "due property not in metamodel",
			mutate:  func(c *CalDAVCollection) { c.Due = "nope" },
			wantSub: `due property "nope" not in metamodel`,
		},
		{
			name:    "due property wrong type",
			mutate:  func(c *CalDAVCollection) { c.Due = "title" },
			wantSub: "must be date- or datetime-typed",
		},
		{
			name:    "summary property not in metamodel",
			mutate:  func(c *CalDAVCollection) { c.Summary = "nope" },
			wantSub: `summary property "nope" not in metamodel`,
		},
		{
			name:    "description property not in metamodel",
			mutate:  func(c *CalDAVCollection) { c.Description = "nope" },
			wantSub: `description property "nope" not in metamodel`,
		},
		{
			name:    "priority must be integer",
			mutate:  func(c *CalDAVCollection) { c.Priority = "title" },
			wantSub: "must be integer-typed",
		},
		{
			name:    "unparseable where clause",
			mutate:  func(c *CalDAVCollection) { c.Where = []string{"!!!"} },
			wantSub: "where[0]",
		},
		{
			name:    "where references unknown property",
			mutate:  func(c *CalDAVCollection) { c.Where = []string{"nope != x"} },
			wantSub: `property "nope" not in metamodel`,
		},
		{
			name:    "vtodo without completion",
			mutate:  func(c *CalDAVCollection) { c.Completion = nil },
			wantSub: "'completion' is required for a vtodo collection",
		},
		{
			name: "completion status property unknown",
			mutate: func(c *CalDAVCollection) {
				c.Completion.StatusProperty = "nope"
			},
			wantSub: `completion.status_property "nope" not in metamodel`,
		},
		{
			name: "completed_value outside the enum",
			mutate: func(c *CalDAVCollection) {
				c.Completion.CompletedValue = "finished"
			},
			wantSub: `completion.completed_value "finished" is not a valid value`,
		},
		{
			name: "pending_value outside the enum",
			mutate: func(c *CalDAVCollection) {
				c.Completion.PendingValue = "pending"
			},
			wantSub: `completion.pending_value "pending" is not a valid value`,
		},
		{
			name: "completed and pending values identical",
			mutate: func(c *CalDAVCollection) {
				c.Completion.PendingValue = "done"
			},
			wantSub: "would be indistinguishable from pending",
		},
		{
			name: "completed_at must be datetime",
			mutate: func(c *CalDAVCollection) {
				c.Completion.CompletedAt = "due" // date, not datetime
			},
			wantSub: "must be datetime-typed",
		},
		{
			name: "on_delete set and hard are exclusive",
			mutate: func(c *CalDAVCollection) {
				c.OnDelete = &CalDAVOnDelete{Set: map[string]string{"status": "done"}, Hard: true}
			},
			wantSub: "mutually exclusive",
		},
		{
			name:    "on_delete with neither set nor hard",
			mutate:  func(c *CalDAVCollection) { c.OnDelete = &CalDAVOnDelete{} },
			wantSub: "must specify either 'set' or 'hard: true'",
		},
		{
			name: "on_delete sets an unknown property",
			mutate: func(c *CalDAVCollection) {
				c.OnDelete = &CalDAVOnDelete{Set: map[string]string{"nope": "x"}}
			},
			wantSub: `property "nope" not in metamodel`,
		},
		{
			name: "on_delete sets a value outside the enum",
			mutate: func(c *CalDAVCollection) {
				c.OnDelete = &CalDAVOnDelete{Set: map[string]string{"status": "archived"}}
			},
			wantSub: `"archived" is not a valid value for property "status"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validTaskCollection()
			tc.mutate(&c)
			cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": c}}}
			errs := validateCalDAV(cfg, caldavMetamodel())
			if !containsSubstring(errs, tc.wantSub) {
				t.Errorf("want an error containing %q, got: %v", tc.wantSub, errs)
			}
			// Every message must name the collection so an author can find the
			// YAML node.
			for _, e := range errs {
				if !strings.Contains(e, `caldav "tasks"`) {
					t.Errorf("error does not identify the collection: %q", e)
				}
			}
		})
	}
}

// TestValidateCalDAV_Constructible is the DEC-HWZHA departure: a create-target
// whose required properties cannot be satisfied from a bare SUMMARY is rejected
// at CONFIG LOAD, where at write time the same condition is only a warning.
//
// A client-created to-do carries only a summary (verified against Apple
// Reminders), so an unsatisfiable mapping would silently produce an invalid
// entity on every client-side create rather than failing once, visibly.
func TestValidateCalDAV_Constructible(t *testing.T) {
	base := func() CalDAVCollection {
		return CalDAVCollection{
			EntityType: "strict",
			Summary:    "title",
			Completion: &CalDAVCompletion{
				StatusProperty: "status", CompletedValue: "done", PendingValue: "todo",
			},
		}
	}

	t.Run("unsatisfiable required property is rejected", func(t *testing.T) {
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"s": base()}}}
		errs := validateCalDAV(cfg, caldavMetamodel())
		if !containsSubstring(errs, `"owner"`) {
			t.Errorf("want the unsatisfiable property named, got: %v", errs)
		}
		if !containsSubstring(errs, "cannot be created from a CalDAV client") {
			t.Errorf("want the constructibility error, got: %v", errs)
		}
	})

	t.Run("a defaults literal satisfies it", func(t *testing.T) {
		c := base()
		c.Defaults = map[string]string{"owner": "unassigned"}
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"s": c}}}
		if errs := validateCalDAV(cfg, caldavMetamodel()); len(errs) > 0 {
			t.Errorf("expected no errors once defaults supply owner, got: %v", errs)
		}
	})

	t.Run("defaults value is itself validated", func(t *testing.T) {
		c := base()
		c.Defaults = map[string]string{"owner": "unassigned", "status": "bogus"}
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"s": c}}}
		if !containsSubstring(validateCalDAV(cfg, caldavMetamodel()), `"bogus" is not a valid value`) {
			t.Error("a defaults value outside the enum should be rejected")
		}
	})

	t.Run("required title is satisfied by the summary mapping", func(t *testing.T) {
		// `task.title` is required and mapped from SUMMARY, and `task.status`
		// is required but has a custom-type default — so the canonical
		// collection is constructible with no defaults block at all.
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": validTaskCollection()}}}
		if errs := validateCalDAV(cfg, caldavMetamodel()); len(errs) > 0 {
			t.Errorf("expected no errors, got: %v", errs)
		}
	})

	t.Run("display-property fallback satisfies a required title", func(t *testing.T) {
		c := base()
		c.Summary = "" // falls back to the display property, which IS title
		c.Defaults = map[string]string{"owner": "unassigned"}
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"s": c}}}
		if errs := validateCalDAV(cfg, caldavMetamodel()); len(errs) > 0 {
			t.Errorf("expected the display-property fallback to satisfy title, got: %v", errs)
		}
	})
}

// TestValidateCalDAV_VEventCollection covers the other component: an event
// collection needs a due/start but must NOT carry completion state.
func TestValidateCalDAV_VEventCollection(t *testing.T) {
	t.Run("valid vevent collection", func(t *testing.T) {
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"cal": {
			Component: CalDAVComponentEvent, EntityType: "task", Due: "due", Summary: "title",
		}}}}
		if errs := validateCalDAV(cfg, caldavMetamodel()); len(errs) > 0 {
			t.Errorf("expected no errors, got: %v", errs)
		}
	})

	t.Run("vevent without a date is rejected", func(t *testing.T) {
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"cal": {
			Component: CalDAVComponentEvent, EntityType: "task", Summary: "title",
		}}}}
		if !containsSubstring(validateCalDAV(cfg, caldavMetamodel()), "'due' is required for a vevent collection") {
			t.Error("a vevent collection with no date should be rejected")
		}
	})

	t.Run("vevent with completion is rejected", func(t *testing.T) {
		cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"cal": {
			Component: CalDAVComponentEvent, EntityType: "task", Due: "due", Summary: "title",
			Completion: &CalDAVCompletion{StatusProperty: "status", CompletedValue: "done", PendingValue: "todo"},
		}}}}
		if !containsSubstring(validateCalDAV(cfg, caldavMetamodel()), "not valid for a vevent collection") {
			t.Error("completion on a vevent collection should be rejected")
		}
	})
}

// TestValidateCalDAV_ErrorsAreDeterministic guards against Go's randomized map
// iteration producing a different error order per run, which would make a
// config failure frustrating to diff.
func TestValidateCalDAV_ErrorsAreDeterministic(t *testing.T) {
	bad := CalDAVCollection{EntityType: "nope"}
	cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{
		"a": bad, "b": bad, "c": bad, "d": bad, "e": bad,
	}}}
	meta := caldavMetamodel()
	first := strings.Join(validateCalDAV(cfg, meta), "\n")
	for range 20 {
		if got := strings.Join(validateCalDAV(cfg, meta), "\n"); got != first {
			t.Fatalf("error order is not deterministic:\n%s\n---\n%s", first, got)
		}
	}
}

// TestValidateConfig_IncludesCalDAV pins the wiring: an invalid caldav block
// must fail ValidateConfig, not just the standalone validator.
func TestValidateConfig_IncludesCalDAV(t *testing.T) {
	cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": {EntityType: "nope"}}}}
	err := ValidateConfig(nil, cfg, caldavMetamodel())
	if err == nil {
		t.Fatal("ValidateConfig should reject an invalid caldav collection")
	}
	if !strings.Contains(err.Error(), "unknown entity type") {
		t.Errorf("want the caldav error surfaced, got: %v", err)
	}
}

func containsSubstring(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// TestValidateCalDAV_CompletionReachable covers the config footgun that
// produces the most baffling client symptom in CalDAV: the checkbox appears
// not to work.
//
// Observed against a real Apple Reminders sync — ticking a to-do wrote the
// completed value, the entity stopped matching the filter, the resource
// vanished, and Reminders restored its local copy UNCHECKED with no error
// anywhere. Catching it at config load turns that into a startup message.
func TestValidateCalDAV_CompletionReachable(t *testing.T) {
	tests := []struct {
		name    string
		where   []string
		wantSub string // "" means the config must be accepted
	}{
		{
			name:    "!= the completed value excludes it",
			where:   []string{"status != done"},
			wantSub: "excludes the completed value",
		},
		{
			name:    "= a non-completed value pins it out",
			where:   []string{"status = todo"},
			wantSub: `pins status to "todo"`,
		},
		{
			name:  "excluding an unrelated value is fine",
			where: []string{"status != cancelled"},
		},
		{
			name:  "a clause on another property is fine",
			where: []string{"title != scratch"},
		},
		{
			name:  "= the completed value is odd but not the footgun",
			where: []string{"status = done"},
		},
		{
			name:  "no clauses at all",
			where: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validTaskCollection()
			c.Where = tc.where
			cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": c}}}
			errs := validateCalDAV(cfg, caldavMetamodel())

			if tc.wantSub == "" {
				for _, e := range errs {
					if strings.Contains(e, "revert") || strings.Contains(e, "excludes the completed") {
						t.Errorf("a valid filter was rejected: %s", e)
					}
				}
				return
			}
			if !containsSubstring(errs, tc.wantSub) {
				t.Errorf("want an error containing %q, got: %v", tc.wantSub, errs)
			}
		})
	}
}

// TestValidateCalDAV_CompletionReachableNeedsAMapping: the check must not fire
// when there is no completion mapping to contradict — those cases are reported
// by validateCalDAVCompletion instead, and a duplicate error is noise.
func TestValidateCalDAV_CompletionReachableNeedsAMapping(t *testing.T) {
	c := validTaskCollection()
	c.Completion = nil
	c.Where = []string{"status != done"}
	cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": c}}}

	for _, e := range validateCalDAV(cfg, caldavMetamodel()) {
		if strings.Contains(e, "excludes the completed value") {
			t.Errorf("the reachability check fired with no completion mapping: %s", e)
		}
	}
}

// TestValidateCalDAV_DescriptionBodySentinel: `description: body` maps to the
// entity's markdown body, so it must not be resolved as a property name — and
// an entity that genuinely has a property called "body" must be called out
// rather than silently resolved one way or the other.
func TestValidateCalDAV_DescriptionBodySentinel(t *testing.T) {
	tests := []struct {
		name        string
		bodyProp    bool // entity declares a property literally named "body"
		wantErrPart string
	}{
		{
			name: "sentinel needs no property to exist",
		},
		{
			name:        "a real body property is ambiguous and reported",
			bodyProp:    true,
			wantErrPart: "reserved word for the entity body",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := caldavMetamodel()
			if tc.bodyProp {
				def := meta.Entities["task"]
				def.Properties["body"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
				meta.Entities["task"] = def
			}
			coll := validTaskCollection()
			coll.Description = CalDAVDescriptionBody
			cfg := &Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": coll}}}

			errs := validateCalDAV(cfg, meta)

			switch tc.wantErrPart {
			case "":
				if len(errs) != 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
			default:
				mentions := func(e string) bool { return strings.Contains(e, tc.wantErrPart) }
				if !slices.ContainsFunc(errs, mentions) {
					t.Errorf("errors = %v, want one mentioning %q", errs, tc.wantErrPart)
				}
			}
		})
	}
}

// TestValidateCalDAV_PriorityMap covers the bucketed priority mapping, whose
// load-bearing check is COVERAGE.
//
// PRIORITY is 1-9 and each client picks its own number inside a band — verified
// on the wire, Thunderbird sends 1 for its "high" and Apple Reminders sends 9
// for its "low". A gap means some client's write silently changes nothing,
// which is the reverting-checkbox failure mode applied to priority.
func TestValidateCalDAV_PriorityMap(t *testing.T) {
	full := []CalDAVPriorityBucket{
		{Value: "high", From: 1, To: 4, Emit: 1},
		{Value: "normal", From: 5, To: 5},
		{Value: "low", From: 6, To: 9, Emit: 9},
	}
	tests := []struct {
		name        string
		buckets     []CalDAVPriorityBucket
		property    string
		alsoRawPrio bool
		wantErrPart string
	}{
		{name: "full coverage of 1-9", buckets: full, property: "urgency"},
		{
			name:        "a gap is rejected",
			buckets:     []CalDAVPriorityBucket{{Value: "high", From: 1, To: 4}, {Value: "low", From: 6, To: 9}},
			property:    "urgency",
			wantErrPart: "leaves PRIORITY 5 uncovered",
		},
		{
			name:        "an inverted range is rejected",
			buckets:     []CalDAVPriorityBucket{{Value: "high", From: 9, To: 1}},
			property:    "urgency",
			wantErrPart: "inverted",
		},
		{
			name:        "a value outside the enum is rejected",
			buckets:     []CalDAVPriorityBucket{{Value: "URGENT", From: 1, To: 9}},
			property:    "urgency",
			wantErrPart: "not one of the property's values",
		},
		{
			name:        "an unknown property is rejected",
			buckets:     full,
			property:    "nope",
			wantErrPart: "not in metamodel",
		},
		{
			name:        "priority and priority_map together are rejected",
			buckets:     full,
			property:    "urgency",
			alsoRawPrio: true,
			wantErrPart: "mutually exclusive",
		},
		{
			name:        "empty buckets are rejected",
			buckets:     []CalDAVPriorityBucket{},
			property:    "urgency",
			wantErrPart: "must not be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := caldavMetamodel()
			def := meta.Entities["task"]
			def.Properties["urgency"] = metamodel.PropertyDef{Type: "task_priority"}
			meta.Entities["task"] = def
			if meta.Types == nil {
				meta.Types = map[string]metamodel.CustomType{}
			}
			meta.Types["task_priority"] = metamodel.CustomType{Values: []string{"high", "normal", "low"}}

			coll := validTaskCollection()
			coll.PriorityMap = &CalDAVPriorityMap{Property: tc.property, Buckets: tc.buckets}
			if !tc.alsoRawPrio {
				coll.Priority = ""
			}
			errs := validateCalDAV(&Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": coll}}}, meta)

			if tc.wantErrPart == "" {
				if len(errs) != 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
				return
			}
			mentions := func(e string) bool { return strings.Contains(e, tc.wantErrPart) }
			if !slices.ContainsFunc(errs, mentions) {
				t.Errorf("errors = %v, want one mentioning %q", errs, tc.wantErrPart)
			}
		})
	}
}

func TestValidateCalDAV_ReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CalDAVCollection)
		wantErr string
	}{
		{
			name: "every known field is accepted",
			mutate: func(c *CalDAVCollection) {
				c.Location, c.Categories, c.Start = "notes", "notes", "starts_at"
				c.ReadOnly = CalDAVReadOnlyFields
			},
		},
		{
			name:   "case is ignored",
			mutate: func(c *CalDAVCollection) { c.ReadOnly = []string{"Summary", " DUE "} },
		},
		{
			name:    "a typo is an error, not a silent no-op",
			mutate:  func(c *CalDAVCollection) { c.ReadOnly = []string{"summry"} },
			wantErr: `unknown field "summry"`,
		},
		{
			name:    "rrule is not a read_only field",
			mutate:  func(c *CalDAVCollection) { c.ReadOnly = []string{"rrule"} },
			wantErr: `unknown field "rrule"`,
		},
		{
			name:    "a duplicate is reported",
			mutate:  func(c *CalDAVCollection) { c.ReadOnly = []string{"due", "due"} },
			wantErr: `is listed more than once`,
		},
		{
			name: "an unmapped field has no effect and says so",
			mutate: func(c *CalDAVCollection) {
				c.Location = ""
				c.ReadOnly = []string{"location"}
			},
			wantErr: "does not map it",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validTaskCollection()
			tc.mutate(&c)
			errs := validateCalDAV(&Config{CalDAV: CalDAVConfig{Static: map[string]CalDAVCollection{"tasks": c}}}, caldavMetamodel())
			joined := strings.Join(errs, "\n")
			switch tc.wantErr {
			case "":
				if len(errs) > 0 {
					t.Errorf("expected no errors, got: %v", errs)
				}
			default:
				if !strings.Contains(joined, tc.wantErr) {
					t.Errorf("expected an error containing %q, got: %v", tc.wantErr, errs)
				}
			}
		})
	}
}

// TestCalDAVCollection_IsReadOnly pins the lookup the mapper depends on.
func TestCalDAVCollection_IsReadOnly(t *testing.T) {
	c := CalDAVCollection{ReadOnly: []string{"Summary", " due "}}
	for _, field := range []string{CalDAVFieldSummary, CalDAVFieldDue} {
		if !c.IsReadOnly(field) {
			t.Errorf("IsReadOnly(%q) = false, want true", field)
		}
	}
	if c.IsReadOnly(CalDAVFieldCompletion) {
		t.Error("IsReadOnly(completion) = true, want false")
	}
	if (CalDAVCollection{}).IsReadOnly(CalDAVFieldSummary) {
		t.Error("an empty read_only must leave every field writable")
	}
}
