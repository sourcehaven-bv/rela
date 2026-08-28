package dataentryconfig

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validCalendarViews are the period layouts a calendar may open on. Day and
// year are deliberately absent — they are a separate ticket, and accepting a
// value that then renders as something else is worse than rejecting it.
var validCalendarViews = map[string]bool{"month": true, "week": true}

// validCalendarWeekStarts are the accepted first-day-of-week values.
var validCalendarWeekStarts = map[string]bool{"monday": true, "sunday": true}

// Defaults applied at load so the wire value is never empty and the SPA does
// not have to re-implement them.
const (
	defaultCalendarView            = "month"
	defaultCalendarWeekStart       = "monday"
	defaultCalendarDayStart        = "08:00"
	defaultCalendarDayEnd          = "20:00"
	defaultCalendarMaxEventsPerDay = 3
	defaultCalendarMaxSpan         = 31
)

// supportedCalendarDateFormats are the property `format:` layouts a calendar
// can write back when an event is dragged.
//
// This is narrower than what the metamodel accepts, deliberately. `format:` is
// a Go time layout, and honoring an arbitrary one in the browser would mean
// shipping a Go-layout interpreter in TypeScript — a whole subsystem with its
// own bug surface. Rejecting the unsupported case at load is honest about what
// drag-to-reschedule can do; a calendar over a custom-format property is a
// documented follow-up, not a silent failure on the user's first drag.
var supportedCalendarDateFormats = map[string]bool{
	"":           true, // unset — the metamodel default (2006-01-02)
	"2006-01-02": true,
}

// validateCalendars checks each calendar against the metamodel. The individual
// rules live on [validateCalendarShell] and [validateCalendarSource].
//
// Errors name the calendar and source index so an author can pinpoint the
// problem, and surface at config load rather than on first render — a calendar
// whose date property is unusable would otherwise fail on the user's first
// drag, far from the config line that caused it.
func validateCalendars(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for calID, cal := range cfg.Calendars {
		errs = append(errs, validateCalendarShell(cfg, calID, cal)...)

		if len(cal.Sources) == 0 {
			errs = append(errs, fmt.Sprintf("calendar %q: must declare at least one source", calID))
			continue
		}
		for i, src := range cal.Sources {
			errs = append(errs, validateCalendarSource(calID, i, src, meta)...)
		}
		errs = append(errs, validateCalendarEventFields(calID, cal, meta)...)
	}
	return errs
}

// validateCalendarShell checks the calendar-level settings that do not depend
// on a source: view, week start, hour bounds, caps and form references.
func validateCalendarShell(cfg *Config, calID string, cal Calendar) []string {
	var errs []string
	prefix := fmt.Sprintf("calendar %q", calID)

	if cal.DefaultView != "" && !validCalendarViews[cal.DefaultView] {
		errs = append(errs, fmt.Sprintf("%s: default_view %q is not valid (valid: %s)",
			prefix, cal.DefaultView, strings.Join(sortedMapKeys(validCalendarViews), ", ")))
	}
	if cal.WeekStart != "" && !validCalendarWeekStarts[cal.WeekStart] {
		errs = append(errs, fmt.Sprintf("%s: week_start %q is not valid (valid: %s)",
			prefix, cal.WeekStart, strings.Join(sortedMapKeys(validCalendarWeekStarts), ", ")))
	}

	startMin, startOK := parseClockMinutes(cal.DayStart)
	if cal.DayStart != "" && !startOK {
		errs = append(errs, fmt.Sprintf("%s: day_start %q is not a HH:MM time", prefix, cal.DayStart))
	}
	endMin, endOK := parseClockMinutes(cal.DayEnd)
	if cal.DayEnd != "" && !endOK {
		errs = append(errs, fmt.Sprintf("%s: day_end %q is not a HH:MM time", prefix, cal.DayEnd))
	}
	// An inverted or empty range renders a zero-height week grid, so reject it
	// here rather than letting the SPA divide by zero.
	if startOK && endOK && cal.DayStart != "" && cal.DayEnd != "" && startMin >= endMin {
		errs = append(errs, fmt.Sprintf("%s: day_start %q must be before day_end %q",
			prefix, cal.DayStart, cal.DayEnd))
	}

	if cal.MaxEventsPerDay < 0 {
		errs = append(errs, prefix+": max_events_per_day must not be negative")
	}

	if cal.EditForm != "" {
		if _, ok := cfg.Forms[cal.EditForm]; !ok {
			errs = append(errs, fmt.Sprintf("%s: references unknown form %q in edit_form", prefix, cal.EditForm))
		}
	}
	if cal.CreateForm != "" {
		if _, ok := cfg.Forms[cal.CreateForm]; !ok {
			errs = append(errs, fmt.Sprintf("%s: references unknown form %q in create_form", prefix, cal.CreateForm))
		}
	}
	return errs
}

// validateCalendarSource checks one source against the metamodel.
func validateCalendarSource(calID string, i int, src CalendarSource, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("calendar %q: source[%d]", calID, i)

	entDef, ok := meta.GetEntityDef(src.EntityType)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown entity type %q", prefix, src.EntityType))
	}

	// Date: required, must exist, must be date- or datetime-typed, must be a
	// single value, and must use a format the drag path can write back.
	dateDef, dateOK := entDef.Properties[src.Date]
	switch {
	case src.Date == "":
		errs = append(errs, prefix+": 'date' is required")
	case !dateOK:
		errs = append(errs, fmt.Sprintf("%s: date property %q not in metamodel for entity %q",
			prefix, src.Date, src.EntityType))
	default:
		errs = append(errs, validateCalendarDateProperty(prefix, "date", src.Date, dateDef)...)
	}

	// EndDate: optional; same checks, plus it must be the same kind as Date so
	// an event is wholly all-day or wholly timed.
	if src.EndDate != "" {
		endDef, endOK := entDef.Properties[src.EndDate]
		switch {
		case !endOK:
			errs = append(errs, fmt.Sprintf("%s: end_date property %q not in metamodel for entity %q",
				prefix, src.EndDate, src.EntityType))
		default:
			errs = append(errs, validateCalendarDateProperty(prefix, "end_date", src.EndDate, endDef)...)
			if dateOK && feedKindMismatch(dateDef.Type, endDef.Type) {
				errs = append(errs, fmt.Sprintf(
					"%s: date property %q is %q but end_date property %q is %q — "+
						"an event must be all-day or timed, not a mix",
					prefix, src.Date, dateDef.Type, src.EndDate, endDef.Type))
			}
		}
	}

	// Summary: optional, but without it the type needs a display property to
	// fall back on, or events would render untitled.
	if src.Summary == "" {
		if entDef.GetPrimaryProperty() == "" {
			errs = append(errs, fmt.Sprintf(
				"%s: 'summary' omitted and entity %q has no display property to fall back to",
				prefix, src.EntityType))
		}
	} else if _, ok := entDef.Properties[src.Summary]; !ok {
		errs = append(errs, fmt.Sprintf("%s: summary property %q not in metamodel for entity %q",
			prefix, src.Summary, src.EntityType))
	}

	if src.Description != "" {
		if _, ok := entDef.Properties[src.Description]; !ok {
			errs = append(errs, fmt.Sprintf("%s: description property %q not in metamodel for entity %q",
				prefix, src.Description, src.EntityType))
		}
	}

	// Where: each clause must parse and reference a real property.
	for j, clause := range src.Where {
		f, err := filter.Parse(clause)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: where[%d] %q: %v", prefix, j, clause, err))
			continue
		}
		if entity.IsEntityPropertyKey(f.Property) {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf("%s: where[%d] references unknown property %q",
					prefix, j, f.Property))
			}
		}
	}

	errs = append(errs, validateCalendarColor(src.Color, prefix)...)

	if src.MaxSpan < 0 {
		errs = append(errs, prefix+": max_span must not be negative")
	}

	return errs
}

// validateCalendarDateProperty checks that a property named as a calendar's
// start or end is usable as one: right type, single-valued, writable format.
func validateCalendarDateProperty(prefix, key, name string, def metamodel.PropertyDef) []string {
	var errs []string

	if !isFeedDateType(def.Type) {
		errs = append(errs, fmt.Sprintf("%s: %s property %q must be date- or datetime-typed, is %q",
			prefix, key, name, def.Type))
		// The remaining checks assume a date type; reporting them too would be
		// noise on top of the real problem.
		return errs
	}
	// A multi-valued date has no single position on a grid, and dragging it
	// would have to guess which value to move.
	if def.List {
		errs = append(errs, fmt.Sprintf(
			"%s: %s property %q is a list; a calendar needs a single date per entity",
			prefix, key, name))
	}
	if def.Type == metamodel.PropertyTypeDate && !supportedCalendarDateFormats[def.Format] {
		errs = append(errs, fmt.Sprintf(
			"%s: %s property %q uses format %q, which a calendar cannot write back "+
				"(supported: the default %q)",
			prefix, key, name, def.Format, "2006-01-02"))
	}
	return errs
}

// validateCalendarEventFields checks chip fields against the sources' types.
//
// A field must resolve on AT LEAST ONE source, not on all of them: sources may
// be different entity types, and requiring every field to exist everywhere
// would make a multi-type calendar nearly unconfigurable. A field that resolves
// nowhere is still an error — it is dead config that can never render.
func validateCalendarEventFields(calID string, cal Calendar, meta *metamodel.Metamodel) []string {
	var errs []string

	for i, f := range cal.Event.Fields {
		prefix := fmt.Sprintf("calendar %q: event.fields[%d]", calID, i)
		if f.Property != "" && f.Relation != "" {
			errs = append(errs, prefix+": specify either property or relation, not both")
			continue
		}
		switch {
		case f.Relation != "":
			if _, ok := meta.GetRelationDef(f.Relation); !ok {
				errs = append(errs, fmt.Sprintf("%s: references unknown relation %q", prefix, f.Relation))
			}
		case f.Property == "":
			errs = append(errs, prefix+": must specify either property or relation")
		case f.Property == "id":
			// `id` is an entity-level key, not a metamodel property. `title` is
			// deliberately NOT exempt: the SPA renders a chip field from
			// `entity.properties[name]`, so a type whose display property is
			// something else (`name`, say) would accept `property: title` at
			// load and then render nothing, forever, with no diagnostic.
		default:
			if !propertyOnAnySource(cal.Sources, f.Property, meta) {
				errs = append(errs, fmt.Sprintf(
					"%s: property %q is not on any of this calendar's entity types", prefix, f.Property))
			}
		}
	}
	return errs
}

// propertyOnAnySource reports whether name is a property of at least one of the
// calendar's source entity types.
func propertyOnAnySource(sources []CalendarSource, name string, meta *metamodel.Metamodel) bool {
	for _, src := range sources {
		entDef, ok := meta.GetEntityDef(src.EntityType)
		if !ok {
			continue // already reported by validateCalendarSource
		}
		if _, ok := entDef.Properties[name]; ok {
			return true
		}
	}
	return false
}

// parseClockMinutes parses a "HH:MM" clock time into minutes past midnight.
//
// Uses time.Parse rather than hand-splitting: the manual version rejected the
// "8:00" every human writes while accepting "+8:00" (Atoi takes a sign), which
// is the usual outcome of validating a format by counting characters.
//
// Nil: never returns a negative count — ok=false means the value is malformed.
func parseClockMinutes(v string) (minutes int, ok bool) {
	for _, layout := range []string{"15:04", "3:04"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.Hour()*60 + t.Minute(), true
		}
	}
	return 0, false
}

// NormalizeCalendars fills in calendar defaults after load so the wire value is
// never empty and the SPA has one source of truth for them.
func NormalizeCalendars(cfg *Config) {
	for id, cal := range cfg.Calendars {
		if cal.DefaultView == "" {
			cal.DefaultView = defaultCalendarView
		}
		if cal.WeekStart == "" {
			cal.WeekStart = defaultCalendarWeekStart
		}
		if cal.DayStart == "" {
			cal.DayStart = defaultCalendarDayStart
		}
		if cal.DayEnd == "" {
			cal.DayEnd = defaultCalendarDayEnd
		}
		if cal.MaxEventsPerDay == 0 {
			cal.MaxEventsPerDay = defaultCalendarMaxEventsPerDay
		}
		for i, src := range cal.Sources {
			if src.MaxSpan == 0 {
				src.MaxSpan = defaultCalendarMaxSpan
				cal.Sources[i] = src
			}
		}
		cfg.Calendars[id] = cal
	}
}
