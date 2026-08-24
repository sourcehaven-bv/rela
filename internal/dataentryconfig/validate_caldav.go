package dataentryconfig

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateCalDAV checks every `caldav:` collection against the metamodel.
//
// It follows validateFeeds: flat, independent checks, human-readable messages
// prefixed with the collection name so an author can find the exact YAML node,
// and an early return once the entity type fails to resolve (every later check
// needs it).
//
// One check departs from the rest of this file's conventions and is called out
// where it happens: a create-target that cannot be constructed from a bare
// SUMMARY is a HARD config error, whereas at write time a missing required
// property is only a warning (DEC-HWZHA).
func validateCalDAV(cfg *Config, meta *metamodel.Metamodel) []string {
	errs := make([]string, 0, len(cfg.CalDAV.Static)+len(cfg.CalDAV.Dynamic))
	for _, name := range sortedCalDAVNames(cfg.CalDAV.Static) {
		errs = append(errs, validateCalDAVCollection(name, cfg.CalDAV.Static[name], meta)...)
	}
	dynNames := make([]string, 0, len(cfg.CalDAV.Dynamic))
	for name := range cfg.CalDAV.Dynamic {
		dynNames = append(dynNames, name)
	}
	sort.Strings(dynNames)
	for _, name := range dynNames {
		errs = append(errs, validateCalDAVDynamic(name, cfg.CalDAV.Dynamic[name], cfg, meta)...)
	}
	return errs
}

// validateCalDAVDynamic checks a pattern: the whole static mapping, plus the
// driver type and relation that make it expand.
func validateCalDAVDynamic(
	name string, c CalDAVDynamicCollection, cfg *Config, meta *metamodel.Metamodel,
) []string {
	prefix := fmt.Sprintf("caldav dynamic %q", name)
	errs := validateCalDAVCollection(name, c.CalDAVCollection, meta)

	// The key becomes a URL segment as `<key>--<driverID>`, so a key containing
	// the separator would make the split ambiguous and could address a
	// different pattern than the operator wrote.
	if strings.Contains(name, dynamicNameSep) {
		errs = append(errs, fmt.Sprintf("%s: key must not contain %q — it separates the "+
			"pattern from the driver id in the URL segment", prefix, dynamicNameSep))
	}
	// A static key and a pattern key cannot collide: both resolve from the same
	// path segment, and the static lookup wins, silently shadowing the pattern.
	if _, clash := cfg.CalDAV.Static[name]; clash {
		errs = append(errs, fmt.Sprintf("%s: a static collection has the same key — "+
			"the static one would shadow this pattern", prefix))
	}

	if c.DriverType == "" {
		errs = append(errs, prefix+": 'driver_type' is required")
	} else if _, ok := meta.GetEntityDef(c.DriverType); !ok {
		errs = append(errs, fmt.Sprintf("%s: unknown driver_type %q", prefix, c.DriverType))
	}
	if c.Relation == "" {
		errs = append(errs, prefix+": 'relation' is required — it selects a collection's "+
			"members AND is the edge a client-created entry receives")
	} else if _, ok := meta.Relations[c.Relation]; !ok {
		errs = append(errs, fmt.Sprintf("%s: unknown relation %q", prefix, c.Relation))
	} else {
		// The edge runs member→driver, so the MEMBER type (entity_type) is the
		// one whose side decides the direction — not driver_type. A wrong
		// direction here is a write bug, not a display one: dynamicMembers
		// queries the mirror side, so a client-created entry lands in the
		// entity type but in NO collection and vanishes on the next sync.
		errs = append(errs, CheckAmbiguousDirection(prefix, c.EntityType, c.Relation, c.Direction, meta)...)
	}
	return errs
}

// sortedCalDAVNames yields collection names in a stable order so config errors
// are deterministic across runs (Go map iteration is randomized).
func sortedCalDAVNames(m map[string]CalDAVCollection) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateCalDAVCollection(name string, c CalDAVCollection, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("caldav %q", name)

	// Component must be one of the two known kinds. Checked before the entity
	// type so a typo here is reported even if the type is also wrong.
	component := c.ComponentOrDefault()
	if component != CalDAVComponentTodo && component != CalDAVComponentEvent {
		errs = append(errs, fmt.Sprintf("%s: unknown component %q (must be %q or %q)",
			prefix, c.Component, CalDAVComponentTodo, CalDAVComponentEvent))
	}

	if c.EntityType == "" {
		return append(errs, prefix+": 'entity_type' is required")
	}
	entDef, ok := meta.GetEntityDef(c.EntityType)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown entity type %q", prefix, c.EntityType))
	}

	// Due is optional (a to-do without a deadline is legal) but must be a
	// date/datetime property when present. A VEVENT collection needs one,
	// because an event with no time is not an event.
	switch c.Due {
	case "":
		if component == CalDAVComponentEvent {
			errs = append(errs, prefix+": 'due' is required for a vevent collection")
		}
	default:
		if def, propOK := entDef.Properties[c.Due]; !propOK {
			errs = append(errs, fmt.Sprintf("%s: due property %q not in metamodel for entity %q",
				prefix, c.Due, c.EntityType))
		} else if !isFeedDateType(def.Type) {
			errs = append(errs, fmt.Sprintf("%s: due property %q must be date- or datetime-typed, is %q",
				prefix, c.Due, def.Type))
		}
	}

	// Summary is optional, but without one the type needs a display property to
	// fall back to — an entry with no title is useless in every client.
	if c.Summary == "" {
		if entDef.GetPrimaryProperty() == "" {
			errs = append(errs, fmt.Sprintf("%s: 'summary' omitted and entity %q has no display property to fall back to",
				prefix, c.EntityType))
		}
	} else if _, propOK := entDef.Properties[c.Summary]; !propOK {
		errs = append(errs, fmt.Sprintf("%s: summary property %q not in metamodel for entity %q",
			prefix, c.Summary, c.EntityType))
	}

	errs = append(errs, validateCalDAVOptionalProps(prefix, c, entDef, meta)...)
	errs = append(errs, validateCalDAVWhere(prefix, c, entDef)...)
	errs = append(errs, validateCalDAVReadOnly(prefix, c)...)
	errs = append(errs, validateCalDAVCompletion(prefix, c, component, entDef, meta)...)
	errs = append(errs, validateCalDAVCompletionReachable(prefix, c)...)
	errs = append(errs, validateCalDAVOnDelete(prefix, c, entDef, meta)...)
	// Defaults are written verbatim on every client-side create, so an invalid
	// value here is not a one-off — it produces an invalid entity every time.
	errs = append(errs, validatePropertyAssignments(prefix+": defaults", c, c.Defaults, entDef, meta)...)
	errs = append(errs, validateCalDAVConstructible(prefix, c, entDef, meta)...)
	return errs
}

// validateCalDAVOptionalProps checks the plain property references that need
// nothing beyond existence (and a type check for priority).
func validateCalDAVOptionalProps(
	prefix string, c CalDAVCollection, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	var errs []string
	switch {
	case c.Description == CalDAVDescriptionBody:
		// The sentinel maps DESCRIPTION to the entity's markdown body, so there
		// is no property to resolve. Warn when a real property of that name is
		// shadowed, rather than silently preferring one reading: the operator
		// wrote something ambiguous and only they know which they meant.
		if _, shadowed := entDef.Properties[CalDAVDescriptionBody]; shadowed {
			errs = append(errs, fmt.Sprintf(
				"%s: description %q is the reserved word for the entity body, but entity %q also has a "+
					"property named %q — rename the property or it cannot be mapped here",
				prefix, CalDAVDescriptionBody, c.EntityType, CalDAVDescriptionBody))
		}
	case c.Description != "":
		if _, ok := entDef.Properties[c.Description]; !ok {
			errs = append(errs, fmt.Sprintf("%s: description property %q not in metamodel for entity %q",
				prefix, c.Description, c.EntityType))
		}
	}
	if c.Priority != "" {
		if def, ok := entDef.Properties[c.Priority]; !ok {
			errs = append(errs, fmt.Sprintf("%s: priority property %q not in metamodel for entity %q",
				prefix, c.Priority, c.EntityType))
		} else if def.Type != metamodel.PropertyTypeInteger {
			// PRIORITY is an RFC 5545 integer 0-9; a non-integer property
			// could not be mapped onto it.
			errs = append(errs, fmt.Sprintf("%s: priority property %q must be integer-typed, is %q",
				prefix, c.Priority, def.Type))
		}
	}
	errs = append(errs, validateCalDAVPriorityMap(prefix, c, entDef, meta)...)
	errs = append(errs, validateCalDAVExtraProps(prefix, c, entDef)...)
	return errs
}

// validateCalDAVExtraProps checks location, categories, start and rrule.
func validateCalDAVExtraProps(prefix string, c CalDAVCollection, entDef *metamodel.EntityDef) []string {
	var errs []string
	for _, ref := range []struct{ kind, prop string }{
		{"location", c.Location},
		{"categories", c.Categories},
	} {
		if ref.prop == "" {
			continue
		}
		if _, ok := entDef.Properties[ref.prop]; !ok {
			errs = append(errs, fmt.Sprintf("%s: %s property %q not in metamodel for entity %q",
				prefix, ref.kind, ref.prop, c.EntityType))
		}
	}
	if c.Start != "" {
		def, ok := entDef.Properties[c.Start]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("%s: start property %q not in metamodel for entity %q",
				prefix, c.Start, c.EntityType))
		case def.Type != metamodel.PropertyTypeDate && def.Type != metamodel.PropertyTypeDatetime:
			errs = append(errs, fmt.Sprintf("%s: start property %q must be date- or datetime-typed, is %q",
				prefix, c.Start, def.Type))
		}
	}
	if c.Rrule != "" {
		switch {
		case rruleIsLiteral(c.Rrule):
			if err := parseLiteralRRule(c.Rrule); err != nil {
				errs = append(errs, fmt.Sprintf("%s: rrule %q is not a valid RFC 5545 recurrence rule: %v",
					prefix, c.Rrule, err))
			}
		default:
			if _, ok := entDef.Properties[c.Rrule]; !ok {
				errs = append(errs, fmt.Sprintf("%s: rrule property %q not in metamodel for entity %q",
					prefix, c.Rrule, c.EntityType))
			}
		}
	}
	return errs
}

// validateCalDAVPriorityMap checks the bucketed priority mapping.
//
// The load-bearing check is COVERAGE: PRIORITY is 1-9 and each client picks its
// own number within a band (verified on the wire: Thunderbird sends 1 for its
// "high", Apple Reminders sends 9 for its "low"). A gap in the buckets means
// some client's write silently changes nothing — the reverting-checkbox failure
// mode again, applied to priority. So an uncovered value is an error.
func validateCalDAVPriorityMap(
	prefix string, c CalDAVCollection, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	pm := c.PriorityMap
	if pm == nil {
		return nil
	}
	var errs []string
	if c.Priority != "" {
		errs = append(errs, fmt.Sprintf(
			"%s: priority and priority_map are mutually exclusive — priority maps the raw "+
				"0-9 integer, priority_map buckets it onto another property", prefix))
	}
	if pm.Property == "" {
		return append(errs, prefix+": priority_map.property is required")
	}
	def, ok := entDef.Properties[pm.Property]
	if !ok {
		return append(errs, fmt.Sprintf("%s: priority_map property %q not in metamodel for entity %q",
			prefix, pm.Property, c.EntityType))
	}
	if len(pm.Buckets) == 0 {
		return append(errs, prefix+": priority_map.buckets must not be empty")
	}

	covered := map[int]bool{}
	for i, b := range pm.Buckets {
		at := fmt.Sprintf("%s: priority_map.buckets[%d]", prefix, i)
		if b.Value == "" {
			errs = append(errs, at+": value is required")
		}
		switch {
		case b.From < minPriorityValue || b.To > maxPriorityValue:
			errs = append(errs, fmt.Sprintf("%s: range %d-%d is outside the RFC 5545 PRIORITY range %d-%d",
				at, b.From, b.To, minPriorityValue, maxPriorityValue))
		case b.From > b.To:
			errs = append(errs, fmt.Sprintf("%s: range %d-%d is inverted", at, b.From, b.To))
		default:
			for v := b.From; v <= b.To; v++ {
				covered[v] = true
			}
		}
		if e := b.EmitValue(); e < minPriorityValue || e > maxPriorityValue {
			errs = append(errs, fmt.Sprintf("%s: emit %d is outside %d-%d",
				at, e, minPriorityValue, maxPriorityValue))
		}
		errs = append(errs, validateCalDAVEnumValue(at, b.Value, enumValues(meta, def))...)
	}

	var gaps []string
	for v := 1; v <= maxPriorityValue; v++ {
		if !covered[v] {
			gaps = append(gaps, strconv.Itoa(v))
		}
	}
	if len(gaps) > 0 {
		errs = append(errs, fmt.Sprintf(
			"%s: priority_map leaves PRIORITY %s uncovered — a client that sends one of those "+
				"(each picks its own number within a band) would silently change nothing",
			prefix, strings.Join(gaps, ", ")))
	}
	return errs
}

// validateCalDAVEnumValue reports a value the property's enum does not allow.
func validateCalDAVEnumValue(at, value string, allowed []string) []string {
	if value == "" || len(allowed) == 0 {
		return nil
	}
	if slices.Contains(allowed, value) {
		return nil
	}
	return []string{fmt.Sprintf("%s: value %q is not one of the property's values (%s)",
		at, value, strings.Join(allowed, ", "))}
}

// RFC 5545 3.8.1.9 PRIORITY bounds. 0 means "undefined" and is deliberately not
// required to be covered by a bucket.
const (
	minPriorityValue = 0
	maxPriorityValue = 9
)

// validateCalDAVWhere parses each filter clause and checks the property it
// names exists, matching validateFeedSource's treatment.
func validateCalDAVWhere(prefix string, c CalDAVCollection, entDef *metamodel.EntityDef) []string {
	var errs []string
	for i, clause := range c.Where {
		f, err := filter.Parse(clause)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: where[%d]: %v", prefix, i, err))
			continue
		}
		if !entity.IsEntityPropertyKey(f.Property) {
			continue
		}
		if _, ok := entDef.Properties[f.Property]; !ok {
			errs = append(errs, fmt.Sprintf("%s: where[%d]: property %q not in metamodel for entity %q",
				prefix, i, f.Property, c.EntityType))
		}
	}
	return errs
}

// validateCalDAVReadOnly checks the `read_only:` list.
//
// A name that is not a mapping field is an ERROR rather than a warning, because
// the failure is invisible in the direction that matters: a typo'd
// `read_only: [summry]` leaves the field writable, and nothing about a client
// happily editing a title tells the operator their lock was never applied. A
// startup error is the only place this gets noticed.
//
// Naming a field the collection does not map is likewise reported. It is
// harmless at runtime — there is no mapping for a client to write through — but
// it is always one of two mistakes: a name meant for a different collection, or
// a mapping the operator believes exists and does not. Both are worth a word.
func validateCalDAVReadOnly(prefix string, c CalDAVCollection) []string {
	var errs []string
	seen := map[string]bool{}
	for i, raw := range c.ReadOnly {
		field := strings.ToLower(strings.TrimSpace(raw))
		at := fmt.Sprintf("%s: read_only[%d]", prefix, i)
		if !slices.Contains(CalDAVReadOnlyFields, field) {
			errs = append(errs, fmt.Sprintf("%s: unknown field %q (must be one of: %s)",
				at, raw, strings.Join(CalDAVReadOnlyFields, ", ")))
			continue
		}
		if seen[field] {
			errs = append(errs, fmt.Sprintf("%s: %q is listed more than once", at, field))
			continue
		}
		seen[field] = true
		if !calDAVFieldIsMapped(c, field) {
			errs = append(errs, fmt.Sprintf(
				"%s: %q is read-only but the collection does not map it, so the entry has no effect",
				at, field))
		}
	}
	return errs
}

// calDAVFieldIsMapped reports whether the collection maps a `read_only:` field
// name to anything. `priority` is mapped by EITHER spelling, and `summary`
// falls back to the entity's display property when unset — the same resolution
// the mapper does.
func calDAVFieldIsMapped(c CalDAVCollection, field string) bool {
	switch field {
	case CalDAVFieldSummary:
		return true // always mapped: `summary:` or the display-property fallback
	case CalDAVFieldDescription:
		return c.Description != ""
	case CalDAVFieldDue:
		return c.Due != ""
	case CalDAVFieldPriority:
		return c.Priority != "" || c.PriorityMap != nil
	case CalDAVFieldLocation:
		return c.Location != ""
	case CalDAVFieldCategories:
		return c.Categories != ""
	case CalDAVFieldStart:
		return c.Start != ""
	case CalDAVFieldCompletion:
		return c.Completion != nil
	default:
		return false
	}
}

// validateCalDAVCompletion checks the completion mapping — the block that makes
// a to-do checkable. Without it a vtodo collection is read-only in practice,
// which defeats the point of CalDAV over an ICS feed, so it is required there.
func validateCalDAVCompletion(
	prefix string, c CalDAVCollection, component string,
	entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	var errs []string
	if c.Completion == nil {
		if component == CalDAVComponentTodo {
			errs = append(errs, prefix+": 'completion' is required for a vtodo collection "+
				"(without it a client cannot check anything off)")
		}
		return errs
	}
	if component == CalDAVComponentEvent {
		return append(errs, prefix+": 'completion' is not valid for a vevent collection (events have no completion state)")
	}

	comp := c.Completion
	if comp.StatusProperty == "" {
		return append(errs, prefix+": completion.status_property is required")
	}
	statusDef, ok := entDef.Properties[comp.StatusProperty]
	if !ok {
		return append(errs, fmt.Sprintf("%s: completion.status_property %q not in metamodel for entity %q",
			prefix, comp.StatusProperty, c.EntityType))
	}

	// Both values must be members of the status property's enum when it has
	// one. A value outside the enum would write an entity the metamodel
	// rejects, on every single check-off.
	allowed := enumValues(meta, statusDef)
	for _, v := range []struct{ field, value string }{
		{"completed_value", comp.CompletedValue},
		{"pending_value", comp.PendingValue},
	} {
		if v.value == "" {
			errs = append(errs, fmt.Sprintf("%s: completion.%s is required", prefix, v.field))
			continue
		}
		if len(allowed) > 0 && !slices.Contains(allowed, v.value) {
			errs = append(errs, fmt.Sprintf("%s: completion.%s %q is not a valid value for property %q (allowed: %s)",
				prefix, v.field, v.value, comp.StatusProperty, strings.Join(allowed, ", ")))
		}
	}
	if comp.CompletedValue != "" && comp.CompletedValue == comp.PendingValue {
		errs = append(errs, fmt.Sprintf("%s: completion.completed_value and pending_value are both %q — "+
			"completion would be indistinguishable from pending", prefix, comp.CompletedValue))
	}

	if comp.CompletedAt != "" {
		if def, propOK := entDef.Properties[comp.CompletedAt]; !propOK {
			errs = append(errs, fmt.Sprintf("%s: completion.completed_at %q not in metamodel for entity %q",
				prefix, comp.CompletedAt, c.EntityType))
		} else if def.Type != metamodel.PropertyTypeDatetime {
			// COMPLETED is an RFC 5545 instant, never a day, so a date-typed
			// property would silently lose the time of day.
			errs = append(errs, fmt.Sprintf("%s: completion.completed_at %q must be datetime-typed, is %q",
				prefix, comp.CompletedAt, def.Type))
		}
	}
	return errs
}

// validateCalDAVCompletionReachable rejects a `where:` clause that excludes the
// very state a completion writes.
//
// This is the config footgun that produces the single most baffling client
// symptom in CalDAV: the checkbox appears not to work. Ticking a to-do writes
// the completed value, the entity immediately stops matching the filter, the
// resource disappears from the collection, and the client — which cannot
// distinguish a filtered-out resource from a deleted one (RFC 6578 §3.5.2) —
// restores its local copy UNCHECKED. Nothing errors anywhere; the tick just
// silently reverts.
//
// Observed against a real Apple Reminders sync during the TKT-MF1CWZ demo,
// with `where: ["status != done"]` and `completed_value: done`.
//
// Hiding completed to-dos is the CLIENT's job, not the server's: RFC 4791
// §7.8.9 defines the canonical "pending to-dos" query for exactly this, and
// Reminders already files completed items into their own section. So the right
// fix is to drop the clause, which the error says.
func validateCalDAVCompletionReachable(prefix string, c CalDAVCollection) []string {
	comp := c.Completion
	if comp == nil || comp.StatusProperty == "" || comp.CompletedValue == "" {
		return nil // nothing to contradict; other checks report those
	}

	var errs []string
	for i, clause := range c.Where {
		f, err := filter.Parse(clause)
		if err != nil || f.Property != comp.StatusProperty {
			continue // unparseable clauses are reported by validateCalDAVWhere
		}
		// Only exact-match operators are checked. A range or regex clause could
		// also exclude the completed value, but deciding that in general means
		// evaluating the filter language against a hypothetical entity — and a
		// wrong guess here would reject a working config, which is worse than
		// missing an exotic one.
		switch {
		case f.Operator == filter.OpNotEqual && f.Value == comp.CompletedValue:
			errs = append(errs, fmt.Sprintf(
				"%s: where[%d] %q excludes the completed value, so checking a to-do off in a "+
					"client makes it vanish and the client silently reverts the tick. "+
					"Remove the clause — hiding completed to-dos is the client's job (RFC 4791 §7.8.9)",
				prefix, i, clause))
		case f.Operator == filter.OpEqual && !f.IsGlob && f.Value != comp.CompletedValue:
			errs = append(errs, fmt.Sprintf(
				"%s: where[%d] %q pins %s to %q, so a to-do completed as %q leaves the collection "+
					"and the client silently reverts the tick. "+
					"Widen the clause — hiding completed to-dos is the client's job (RFC 4791 §7.8.9)",
				prefix, i, clause, comp.StatusProperty, f.Value, comp.CompletedValue))
		}
	}
	return errs
}

// validateCalDAVOnDelete checks the delete mapping. Set and Hard are mutually
// exclusive; absent means DELETE is refused, which is a legitimate choice.
func validateCalDAVOnDelete(
	prefix string, c CalDAVCollection, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	var errs []string
	if c.OnDelete == nil {
		return errs
	}
	hasSet := len(c.OnDelete.Set) > 0
	switch {
	case hasSet && c.OnDelete.Hard:
		errs = append(errs, prefix+": on_delete.set and on_delete.hard are mutually exclusive")
	case !hasSet && !c.OnDelete.Hard:
		errs = append(errs, prefix+": on_delete must specify either 'set' or 'hard: true'")
	}
	errs = append(errs, validatePropertyAssignments(prefix+": on_delete.set", c, c.OnDelete.Set, entDef, meta)...)
	return errs
}

// validatePropertyAssignments checks a literal property→value map (on_delete.set
// or defaults) against the metamodel: the property must exist, and the value
// must be a member of its enum when it has one.
func validatePropertyAssignments(
	prefix string, c CalDAVCollection, assignments map[string]string,
	entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	var errs []string
	keys := make([]string, 0, len(assignments))
	for k := range assignments {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, prop := range keys {
		def, ok := entDef.Properties[prop]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: property %q not in metamodel for entity %q",
				prefix, prop, c.EntityType))
			continue
		}
		allowed := enumValues(meta, def)
		if len(allowed) > 0 && !slices.Contains(allowed, assignments[prop]) {
			errs = append(errs, fmt.Sprintf("%s: %q is not a valid value for property %q (allowed: %s)",
				prefix, assignments[prop], prop, strings.Join(allowed, ", ")))
		}
	}
	return errs
}

// validateCalDAVConstructible is the check that turns a runtime surprise into a
// startup error: every REQUIRED property of the create-target must be
// satisfiable when a client creates an entry.
//
// This matters because a client-created to-do carries almost nothing — verified
// against Apple Reminders, a new to-do arrives with SUMMARY, STATUS and
// timestamps and nothing else. So a required property must come from the summary
// mapping, a `defaults:` literal, or a metamodel default.
//
// DELIBERATE DEPARTURE FROM DEC-HWZHA. At write time a missing required property
// is SOFT — the write succeeds with a warning, because a hand-edited markdown
// file can legitimately be in that state. Here it is a HARD config error,
// because the failure mode is different in kind: an unsatisfiable mapping does
// not produce one warning on one entity, it silently produces an invalid entity
// on EVERY client-side create, forever, with nobody watching the server log. The
// operator can see and fix this at startup; a warning stream nobody reads is not
// a control.
//
// Template defaults are NOT consulted: templates live on disk under
// templates/entities/ and this validator receives only (cfg, meta). An operator
// relying on a template default must repeat it in `defaults:` — the error
// message says so.
func validateCalDAVConstructible(
	prefix string, c CalDAVCollection, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	var missing []string
	for _, prop := range sortedPropertyNames(entDef) {
		def := entDef.Properties[prop]
		if !def.Required {
			continue
		}
		switch {
		case prop == c.Summary:
			// Supplied by SUMMARY, which every client sends.
		case c.Summary == "" && prop == entDef.GetPrimaryProperty():
			// Same, via the display-property fallback.
		case c.Defaults[prop] != "":
			// Supplied by a literal.
		case def.Default != "":
			// Supplied by a metamodel property default.
		case meta.GetTypeDefault(def.Type) != "":
			// Supplied by the custom type's default (how `status` usually
			// gets its value).
		case c.Completion != nil && prop == c.Completion.StatusProperty:
			// The completion mapping always writes this property.
		default:
			missing = append(missing, prop)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s: entity %q cannot be created from a CalDAV client: required propert%s %s "+
			"%s no value (a client-created entry supplies only a summary). "+
			"Add %s to 'defaults:', or a default in the metamodel",
		prefix, c.EntityType,
		plural(len(missing), "y", "ies"), strings.Join(quoteAll(missing), ", "),
		plural(len(missing), "has", "have"),
		plural(len(missing), "it", "them"),
	)}
}

// sortedPropertyNames yields an entity's property names in a stable order so
// the constructibility error lists them deterministically.
func sortedPropertyNames(entDef *metamodel.EntityDef) []string {
	names := make([]string, 0, len(entDef.Properties))
	for name := range entDef.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// enumValues returns the allowed values for a property, resolving a named
// custom type. Empty means the property is not enum-constrained.
func enumValues(meta *metamodel.Metamodel, def metamodel.PropertyDef) []string {
	if len(def.Values) > 0 {
		return def.Values
	}
	if ct, ok := meta.Types[def.Type]; ok {
		return ct.Values
	}
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}
