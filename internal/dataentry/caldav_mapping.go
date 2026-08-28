package dataentry

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// caldavMapper converts between a rela entity and a calfeed.Todo for one
// configured collection, in both directions.
//
// The two directions are deliberately implemented from the SAME config
// (dataentryconfig.CalDAVCollection): a collection declares one entity_type and
// one property mapping, so the outbound projection and the inbound patch cannot
// describe different things. That symmetry is why the config has no separate
// create block.
type caldavMapper struct {
	name string
	cfg  dataentryconfig.CalDAVCollection
	meta *metamodel.Metamodel
	link deepLinker
	// ignoreReadOnly relaxes `read_only:` for the create path. Set only on a
	// COPY of the mapper (see patchIgnoringReadOnly), never on a shared one.
	ignoreReadOnly bool
}

func newCalDAVMapper(
	name string, cfg dataentryconfig.CalDAVCollection, meta *metamodel.Metamodel, link deepLinker,
) *caldavMapper {
	return &caldavMapper{name: name, cfg: cfg, meta: meta, link: link}
}

// summaryProperty is the property mapped to SUMMARY, falling back to the entity
// type's display property. Config validation guarantees one of the two exists.
func (m *caldavMapper) summaryProperty() string {
	if m.cfg.Summary != "" {
		return m.cfg.Summary
	}
	if def, ok := m.meta.GetEntityDef(m.cfg.EntityType); ok {
		return def.GetPrimaryProperty()
	}
	return ""
}

// toTodo projects an entity into a to-do.
//
// Properties absent from the entity simply do not appear on the to-do: a
// missing due date is a to-do with no deadline, which is legal and common.
func (m *caldavMapper) toTodo(e *entitypkg.Entity, uid, url string) calfeed.Todo {
	td := calfeed.Todo{
		UID:     uid,
		Summary: e.GetString(m.summaryProperty()),
		URL:     url,
	}
	switch {
	case m.mapsDescriptionToBody():
		td.Description = e.Content
	case m.cfg.Description != "":
		td.Description = e.GetString(m.cfg.Description)
	}
	if m.cfg.Due != "" {
		if due, timed, ok := m.parseDue(e); ok {
			td.Due, td.Timed = due, timed
		}
	}
	if m.cfg.Priority != "" {
		if n, err := strconv.Atoi(e.GetString(m.cfg.Priority)); err == nil {
			td.Priority = n // calfeed clamps to the RFC range
		}
	}
	if pm := m.cfg.PriorityMap; pm != nil {
		if v := e.GetString(pm.Property); v != "" {
			for _, b := range pm.Buckets {
				if b.Value == v {
					td.Priority = b.EmitValue()
					break
				}
			}
		}
	}
	if m.cfg.Location != "" {
		td.Location = e.GetString(m.cfg.Location)
	}
	if m.cfg.Categories != "" {
		td.Categories = e.GetAttributeStrings(m.cfg.Categories)
	}
	if m.cfg.Start != "" {
		if start, timed, ok := m.parseDateProperty(e, m.cfg.Start); ok {
			td.Start = start
			// DTSTART and DUE must agree on value type (RFC 5545 3.8.2.4). DUE
			// already set Timed when present; only adopt DTSTART's shape when
			// it is the sole date on the to-do.
			if td.Due.IsZero() {
				td.Timed = timed
			}
		}
	}
	td.RRule = m.rruleFor(e)
	m.applyCompletionToTodo(e, &td)
	return td
}

// parseDue reads the due property, reporting whether it carries a time of day.
// The declared property TYPE decides: a `date` is all-day, a `datetime` is an
// instant — the same rule the ICS feed uses.
func (m *caldavMapper) parseDue(e *entitypkg.Entity) (due time.Time, timed, ok bool) {
	return m.parseDateProperty(e, m.cfg.Due)
}

// parseDateProperty reads any date- or datetime-typed property, reporting
// whether it carries a time of day.
func (m *caldavMapper) parseDateProperty(
	e *entitypkg.Entity, prop string,
) (when time.Time, timed, ok bool) {
	def, defOK := m.propertyDef(prop)
	if !defOK {
		return time.Time{}, false, false
	}
	parsed, parsedOK := entityTimeValue(e, prop, &def)
	if !parsedOK {
		return time.Time{}, false, false
	}
	return parsed, def.Type == metamodel.PropertyTypeDatetime, true
}

// rruleFor resolves the collection's recurrence rule for one entity.
//
// Two forms, matching `feeds:`: a literal rule (recognizable by its "=") is
// used verbatim, and a bare identifier names a property whose per-entity value
// supplies the rule.
func (m *caldavMapper) rruleFor(e *entitypkg.Entity) string {
	spec := m.cfg.Rrule
	if spec == "" {
		return ""
	}
	if strings.Contains(spec, "=") {
		return spec
	}
	return e.GetString(spec)
}

// entityTimeValue reads a date/datetime property as a time.Time.
//
// It must accept BOTH shapes the store can hold. yaml.v3 decodes an unquoted
// scalar like `due: 2026-08-12` straight to time.Time, while a quoted or
// machine-written value arrives as a string — and Entity.GetString returns ""
// for the former, so reading a date through it silently drops every
// hand-authored due date. (The same latent hazard exists in the ICS feed's
// mapEntity; see RR notes on TKT-MF1CWZ.)
func entityTimeValue(e *entitypkg.Entity, key string, def *metamodel.PropertyDef) (time.Time, bool) {
	switch v := e.Properties[key].(type) {
	case time.Time:
		return v, true
	case string:
		if v == "" {
			return time.Time{}, false
		}
		parsed, err := metamodel.ParseDateValue(v, def)
		if err != nil {
			return time.Time{}, false
		}
		return parsed, true
	default:
		return time.Time{}, false
	}
}

// applyCompletionToTodo maps the entity's status onto the completion trio.
// calfeed.Todo.Complete sets all three together, which is why the mapping only
// has to decide "is this done".
func (m *caldavMapper) applyCompletionToTodo(e *entitypkg.Entity, td *calfeed.Todo) {
	comp := m.cfg.Completion
	if comp == nil {
		return
	}
	if e.GetString(comp.StatusProperty) != comp.CompletedValue {
		td.Status = calfeed.TodoNeedsAction
		return
	}
	// Prefer a recorded completion timestamp; fall back to the due date, then
	// to a zero time. calfeed normalizes whatever we produce, so a completed
	// to-do can never end up without its COMPLETED property.
	at := td.Due
	if comp.CompletedAt != "" {
		if def, ok := m.propertyDef(comp.CompletedAt); ok {
			if parsed, parsedOK := entityTimeValue(e, comp.CompletedAt, &def); parsedOK {
				at = parsed
			}
		}
	}
	if at.IsZero() {
		// A to-do marked done with no timestamp anywhere: use the epoch-free
		// fallback of "now" rather than dropping COMPLETED, because RFC 4791
		// §7.8.9's pending filter keys on its ABSENCE — omitting it would make
		// a completed to-do reappear as pending in filter-driven clients.
		at = time.Now().UTC()
	}
	td.Complete(at)
}

// patchFor converts an inbound to-do into the property changes it implies.
//
// Only properties the collection maps AND the client actually SENT are touched.
// Both halves matter: PatchEntity preserves properties the patch does not name,
// so naming one is a write — and naming it with an empty value is exactly as
// destructive as an unset. A client that omits DESCRIPTION (Apple omits it
// whenever the note is empty) must not blank the mapped property.
func (m *caldavMapper) patchFor(in inboundTodo) (entitypkg.Patch, error) {
	td := in.Todo
	props := map[string]any{}
	var unset []string

	if sp := m.summaryProperty(); sp != "" && m.writable(dataentryconfig.CalDAVFieldSummary) && in.has(ical.PropSummary) {
		props[sp] = td.Summary
	}
	writableDesc := m.cfg.Description != "" && m.writable(dataentryconfig.CalDAVFieldDescription)
	if writableDesc && in.has(ical.PropDescription) && !m.mapsDescriptionToBody() {
		props[m.cfg.Description] = td.Description
	}
	if m.cfg.Priority != "" && m.writable(dataentryconfig.CalDAVFieldPriority) && in.has(ical.PropPriority) {
		props[m.cfg.Priority] = td.Priority
	}
	if err := m.applyExtrasToPatch(in, props, &unset); err != nil {
		return entitypkg.Patch{}, err
	}

	if m.cfg.Due != "" && m.writable(dataentryconfig.CalDAVFieldDue) && in.has(ical.PropDue) {
		switch {
		case td.Due.IsZero():
			// The client sent DUE with an empty value: an explicit clear, so an
			// explicit unset. (A client that simply omits DUE lands in the
			// has() check above and changes nothing.)
			unset = append(unset, m.cfg.Due)
		default:
			formatted, err := m.formatDue(td)
			if err != nil {
				return entitypkg.Patch{}, err
			}
			props[m.cfg.Due] = formatted
		}
	}

	m.applyCompletionToPatch(in, props, &unset)

	patch := entitypkg.Patch{Properties: props, MetaUnset: unset}
	// The body is replaced through Patch.Content, never through Properties:
	// Content is a tri-state pointer, so leaving it nil preserves the existing
	// body — the same "unnamed properties are preserved" guarantee, applied to
	// the one field that is not a property.
	if m.mapsDescriptionToBody() && m.writable(dataentryconfig.CalDAVFieldDescription) && in.has(ical.PropDescription) {
		body := td.Description
		patch.Content = &body
	}
	return patch, nil
}

// writable reports whether inbound edits to a mapped field are applied.
//
// Read-only is enforced HERE, at the single point where an inbound to-do
// becomes a patch, rather than by filtering the parsed VTODO or by rejecting
// the request. Both alternatives were considered and are worse:
//
//   - Stripping properties from the inbound VTODO would make the field look
//     UNSENT, and unsent is already meaningful — patchFor uses it to decide
//     "the client said nothing, preserve what is stored". Conflating "refused"
//     with "not mentioned" works today only because both outcomes happen to be
//     "change nothing", and would silently break the moment a field needs to
//     distinguish them (an explicit clear already does: see the DUE branch).
//   - Refusing the whole PUT with a 403 would discard the fields that ARE
//     writable in the same request. A client sends the entire VTODO on every
//     edit, so a user who ticks a to-do off in an app that also touches DTSTAMP
//     would lose the tick to a field they never edited.
//
// So a read-only field is dropped from the patch and everything else applies.
func (m *caldavMapper) writable(field string) bool {
	return m.ignoreReadOnly || !m.cfg.IsReadOnly(field)
}

// droppedReadOnly reports whether this to-do carries a value for a field the
// collection refuses to accept — i.e. whether the stored result will DIFFER
// from what the client submitted.
//
// This drives ETag suppression, not the write itself. RFC 4791 §5.3.4 lets a
// server return a strong ETag on PUT only when the stored representation is
// octet-equal to the submitted one, so a discarded field means no ETag, and the
// client re-reads to discover what was actually kept.
//
// Only fields the client SENT count. A read-only field the client never
// mentioned changes nothing about the stored result, so the representations
// still match and the ETag stands.
func (m *caldavMapper) droppedReadOnly(in inboundTodo) bool {
	for _, f := range []struct {
		field string
		prop  string
		sent  bool
	}{
		{dataentryconfig.CalDAVFieldSummary, m.summaryProperty(), in.has(ical.PropSummary)},
		{dataentryconfig.CalDAVFieldDescription, m.cfg.Description, in.has(ical.PropDescription)},
		{dataentryconfig.CalDAVFieldDue, m.cfg.Due, in.has(ical.PropDue)},
		{dataentryconfig.CalDAVFieldLocation, m.cfg.Location, in.has(ical.PropLocation)},
		{dataentryconfig.CalDAVFieldCategories, m.cfg.Categories, in.has(ical.PropCategories)},
		{dataentryconfig.CalDAVFieldStart, m.cfg.Start, in.has(ical.PropDateTimeStart)},
	} {
		if f.sent && f.prop != "" && !m.writable(f.field) {
			return true
		}
	}
	// Priority is mapped by either spelling, and completion by the trio.
	priorityMapped := m.cfg.Priority != "" || m.cfg.PriorityMap != nil
	if in.has(ical.PropPriority) && priorityMapped && !m.writable(dataentryconfig.CalDAVFieldPriority) {
		return true
	}
	sentCompletion := in.has(ical.PropStatus) || in.has(ical.PropCompleted)
	if sentCompletion && m.cfg.Completion != nil && !m.writable(dataentryconfig.CalDAVFieldCompletion) {
		return true
	}
	return false
}

// patchIgnoringReadOnly builds a patch with every mapped field treated as
// writable, for the create path (see createPatch for why).
//
// Implemented by copying the mapper with the flag set, rather than by setting
// and clearing it on the receiver: a caldavMapper is shared across concurrent
// requests (mapperFor builds it from config, and PUTs run in parallel), so a
// mutate-then-restore would let one request's create silently disable read-only
// for another request's update.
func (m *caldavMapper) patchIgnoringReadOnly(in inboundTodo) (entitypkg.Patch, error) {
	relaxed := *m
	relaxed.ignoreReadOnly = true
	return relaxed.patchFor(in)
}

// mapsDescriptionToBody reports whether `description:` targets the entity's
// markdown body rather than a property.
func (m *caldavMapper) mapsDescriptionToBody() bool {
	return m.cfg.Description == dataentryconfig.CalDAVDescriptionBody
}

// applyExtrasToPatch maps the optional properties added after the initial
// mapping set: bucketed priority, location, categories and start date. Split
// out of patchFor to keep that function's branching legible.
func (m *caldavMapper) applyExtrasToPatch(in inboundTodo, props map[string]any, unset *[]string) error {
	td := in.Todo
	writablePriority := m.writable(dataentryconfig.CalDAVFieldPriority)
	if pm := m.cfg.PriorityMap; pm != nil && writablePriority && in.has(ical.PropPriority) {
		// The FIRST bucket containing the value wins, so overlapping ranges
		// resolve deterministically by declaration order rather than by map
		// iteration. A value outside every bucket (including the RFC's 0 =
		// undefined) leaves the property alone rather than guessing.
		for _, b := range pm.Buckets {
			if td.Priority >= b.From && td.Priority <= b.To {
				props[pm.Property] = b.Value
				break
			}
		}
	}
	if m.cfg.Location != "" && m.writable(dataentryconfig.CalDAVFieldLocation) && in.has(ical.PropLocation) {
		props[m.cfg.Location] = td.Location
	}
	if m.cfg.Categories != "" && m.writable(dataentryconfig.CalDAVFieldCategories) && in.has(ical.PropCategories) {
		props[m.cfg.Categories] = td.Categories
	}
	if m.cfg.Start != "" && m.writable(dataentryconfig.CalDAVFieldStart) && in.has(ical.PropDateTimeStart) {
		switch {
		case td.Start.IsZero():
			*unset = append(*unset, m.cfg.Start)
		default:
			formatted, err := m.formatDate(td.Start, m.cfg.Start)
			if err != nil {
				return err
			}
			props[m.cfg.Start] = formatted
		}
	}

	return nil
}

// formatDue renders a to-do's due date in the target property's declared
// format, so the stored value round-trips through the same parser the outbound
// direction uses.
func (m *caldavMapper) formatDue(td calfeed.Todo) (string, error) {
	return m.formatDate(td.Due, m.cfg.Due)
}

// formatDate renders t in the target property's declared format, so the stored
// value round-trips through the same parser the outbound direction uses.
func (m *caldavMapper) formatDate(t time.Time, prop string) (string, error) {
	def, ok := m.propertyDef(prop)
	if !ok {
		return "", fmt.Errorf("caldav %q: property %q is not in the metamodel", m.name, prop)
	}
	if def.Type == metamodel.PropertyTypeDatetime {
		return t.UTC().Format(time.RFC3339), nil
	}
	return t.Format(def.GetDateFormat()), nil
}

// applyCompletionToPatch maps an inbound completion state onto entity
// properties.
//
// ONLY the two states that have a rela meaning are mapped. RFC 5545 also
// defines IN-PROCESS and CANCELLED for a VTODO, and neither corresponds to the
// binary done/pending the collection declares — mapping them onto the pending
// value would silently reset a task the user moved to "doing" in rela, and
// resurrect one they cancelled. An unrecognized or unsent STATUS leaves the
// property untouched.
func (m *caldavMapper) applyCompletionToPatch(in inboundTodo, props map[string]any, unset *[]string) {
	comp := m.cfg.Completion
	if comp == nil || !m.writable(dataentryconfig.CalDAVFieldCompletion) {
		return
	}
	// A COMPLETED timestamp is itself a completion signal, so either property
	// arriving counts as the client having spoken about completion.
	if !in.has(ical.PropStatus) && !in.has(ical.PropCompleted) {
		return
	}

	switch in.Todo.Status {
	case calfeed.TodoCompleted:
		props[comp.StatusProperty] = comp.CompletedValue
		if comp.CompletedAt != "" && !in.Todo.Completed.IsZero() {
			props[comp.CompletedAt] = in.Todo.Completed.UTC().Format(time.RFC3339)
		}
	case calfeed.TodoNeedsAction:
		// Re-opened: restore the pending value and clear any completion stamp,
		// or the entity would claim a completion time for unfinished work.
		props[comp.StatusProperty] = comp.PendingValue
		if comp.CompletedAt != "" {
			*unset = append(*unset, comp.CompletedAt)
		}
	default:
		// IN-PROCESS / CANCELLED / unset: no rela meaning, leave it alone.
	}
}

// createProperties builds the property set for a CLIENT-created entry.
//
// A client-created to-do carries almost nothing — verified against Apple
// Reminders, which sends only SUMMARY, STATUS and timestamps — so the
// collection's `defaults:` supply whatever the metamodel requires. Config
// validation has already proven this produces a constructible entity.
// Returns the whole patch, not just the property map: when `description:` maps
// to the body, the new entity's content rides in Patch.Content, and a caller
// handed only the map would silently create the entity with an empty body.
// `read_only:` does NOT apply here, and that is the whole reason this function
// exists separately rather than being patchFor plus defaults.
//
// Read-only means "a client cannot CHANGE the stored value". On a create there
// is no stored value to protect: the alternative reading — drop the field —
// would take the summary the client just typed and create a titleless entity,
// because SUMMARY is the one field every client sends and the usual read-only
// list starts with it. The operator's `read_only: [summary]` means "the web app
// owns the title from here on", not "discard the title I am creating with".
//
// The gap this leaves is narrow and closes itself: a client can set a read-only
// field exactly once, at creation, and never again. An operator who wants no
// client-created entries at all withholds that separately — `defaults:` cannot
// satisfy the required properties, or the collection refuses creates outright.
func (m *caldavMapper) createPatch(in inboundTodo) (entitypkg.Patch, error) {
	patch, err := m.patchIgnoringReadOnly(in)
	if err != nil {
		return entitypkg.Patch{}, err
	}
	// Defaults do NOT override the client's values: a default is what to use
	// when the client said nothing.
	for k, v := range m.cfg.Defaults {
		if _, set := patch.Properties[k]; !set {
			patch.Properties[k] = v
		}
	}
	return patch, nil
}

// deleteePatch is the mutation a client DELETE applies, and reports whether the
// entity should instead be really deleted.
//
// The default is a status transition rather than a delete: the client gesture
// is a swipe, rela has no soft-delete, and DeleteEntity cascades to relations,
// so a mis-swipe would destroy a graph node and its edges.
func (m *caldavMapper) deletePatch() (patch entitypkg.Patch, hard, configured bool) {
	od := m.cfg.OnDelete
	if od == nil {
		return entitypkg.Patch{}, false, false
	}
	if od.Hard {
		return entitypkg.Patch{}, true, true
	}
	props := make(map[string]any, len(od.Set))
	for k, v := range od.Set {
		props[k] = v
	}
	return entitypkg.Patch{Properties: props}, false, true
}

// propertyDef resolves a property definition on the collection's entity type.
func (m *caldavMapper) propertyDef(name string) (metamodel.PropertyDef, bool) {
	def, ok := m.meta.GetEntityDef(m.cfg.EntityType)
	if !ok {
		return metamodel.PropertyDef{}, false
	}
	pd, ok := def.Properties[name]
	return pd, ok
}
