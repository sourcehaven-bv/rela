package metamodel

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// RollupFunc names the aggregate a [RollupDef] computes.
type RollupFunc string

// The roll-up functions. Exactly one is declared per roll-up.
const (
	RollupCount  RollupFunc = "count"
	RollupExists RollupFunc = "exists"
	RollupMin    RollupFunc = "min"
	RollupMax    RollupFunc = "max"
	RollupSum    RollupFunc = "sum"
	RollupLatest RollupFunc = "latest_by"
	// RollupAvg and RollupLatestRow are the functions of a roll-up over a
	// record type (TKT-VMSWSWP): the average, and the value of the newest
	// row, of one field.
	RollupAvg       RollupFunc = "avg"
	RollupLatestRow RollupFunc = "latest"
	// RollupBodyTasks counts the markdown task lines in the entity's own
	// body (TKT-SQ8O0EQ); [RollupDef.BodyTasks] says which count.
	RollupBodyTasks RollupFunc = "body_tasks"
)

// The counts a body_tasks roll-up may name.
const (
	BodyTasksDone  = "done"
	BodyTasksOpen  = "open"
	BodyTasksTotal = "total"
)

// RollupDef declares a property whose value aggregates the entities related
// through one relation, or through a path of up to [MaxRollupPath] relations
// (TKT-6I2OPG, TKT-LTSNHD):
//
//	bewijs_aantal:
//	  type: integer
//	  rollup: { via: evidencedBy, count: true, where: { status: geldig } }
//	laatste_testresultaat:
//	  type: testresultaat
//	  rollup: { via: testedBy, pick: resultaat, latest_by: datum }
//	gedekt:
//	  type: boolean
//	  rollup: { via: [verwijst_naar, wordt_geimplementeerd_door], exists: true }
//	saldo:
//	  type: decimal
//	  rollup: { records: boeking, sum: bedrag, window: 30d }
//	taken_klaar:
//	  type: integer
//	  rollup: { body_tasks: done }
//
// A roll-up with `body_tasks` counts the task lines (`- [ ]`, `- [x]`) of
// the entity's own markdown body: `done`, `open` or `total` (TKT-SQ8O0EQ).
// It declares no other option. It has a value only where the reader may
// read the body.
//
// A roll-up with `records` reads the rows of a record type owned by the
// entity instead of related entities (TKT-VMSWSWP): `latest`, `sum` or `avg`
// of one field, over the rows of the last `window` (all rows when empty).
// The rows are as visible as their owner.
//
// The value is computed when the entity is read, with the reader's access:
// a related entity the reader cannot read does not count, an entity on the
// path the reader cannot read breaks the path, and a related entity on which
// the reader cannot see a property the roll-up reads is left out. A related
// entity reached by several paths counts once. A path that leads back to the
// source type can reach the source itself, and then counts it. It is never stored, so it
// cannot be written. A list filters and sorts on it by computing it for
// every scoped row first.
//
// The loader checks the declaration's shape. The checks that need the
// relation and the related type (does `via` exist, is `pick` declared there,
// do the types agree, is there a chain) run in internal/rollup, at the same
// point computed expressions are compiled.
type RollupDef struct {
	// Via is the path from the entity to the related entities: one relation
	// name, or a list of up to [MaxRollupPath]. A canonical name walks
	// FROM -> TO, the relation's inverse id walks TO -> FROM, as in
	// related().
	Via StringOrSlice `yaml:"via,omitempty"`
	// Type picks the related type when the last relation's far side names
	// several.
	Type string `yaml:"type,omitempty"`
	// Where keeps the related entities whose properties equal these values.
	// It filters the far end of the path only.
	Where map[string]string `yaml:"where,omitempty"`

	// Records names a record type owned by the entity type; the roll-up
	// then reads its rows and declares none of via, type, where, pick or
	// the relation functions.
	Records string `yaml:"records,omitempty"`
	// Window keeps the rows of the last window, such as "30d" (the units
	// of a record retention). Only with Records.
	Window string `yaml:"window,omitempty"`

	Count    bool   `yaml:"count,omitempty"`
	Exists   bool   `yaml:"exists,omitempty"`
	Min      string `yaml:"min,omitempty"`
	Max      string `yaml:"max,omitempty"`
	Sum      string `yaml:"sum,omitempty"`
	Pick     string `yaml:"pick,omitempty"`
	LatestBy string `yaml:"latest_by,omitempty"`
	// Avg and Latest are the record functions: the average of the named
	// field, and its value on the newest row that holds one.
	AvgOf    string `yaml:"avg,omitempty"`
	LatestOf string `yaml:"latest,omitempty"`
	// BodyTasks is the count of a roll-up over the entity's own body:
	// done, open or total.
	BodyTasks string `yaml:"body_tasks,omitempty"`

	// Along is the walk of an inherited roll-up: segment relation names, a
	// `*` suffix repeating a segment zero or more times. The roll-up folds
	// every entity the walk visits after one or more hops, within MaxDepth
	// hops. Condition maps a visited type to the predicate expression that
	// makes one of its entities count; a visited type without an entry is
	// walked through but never counts. A via of one relation may carry a
	// Condition too, keyed by the related type, instead of Where and Type.
	// A condition may read roll-ups of the type it judges, one level deep.
	Along     StringOrSlice     `yaml:"along,omitempty"`
	MaxDepth  int               `yaml:"max_depth,omitempty"`
	Condition map[string]string `yaml:"condition,omitempty"`

	SiblingsDef `yaml:",inline"`
}

// SiblingsDef holds the options of an earlier-siblings roll-up (design
// gap 2). SiblingsBefore names the ordered relation: the entity's siblings
// are the other sources of the relation into its parent, and the roll-up
// reads those that come before it in the incoming order. Where filters the
// siblings, WhenParent the parent; Climb judges each ancestor along the
// relation too, within MaxDepth hops.
type SiblingsDef struct {
	SiblingsBefore string            `yaml:"siblings_before,omitempty"`
	WhenParent     map[string]string `yaml:"when_parent,omitempty"`
	Climb          bool              `yaml:"climb,omitempty"`
}

// AlongRepeat is the suffix that makes an along segment repeat.
const AlongRepeat = "*"

// ParseAlongSegment splits one along segment into its relation name and
// whether it repeats, trimming space around both parts.
func ParseAlongSegment(seg string) (name string, repeat bool) {
	name, repeat = strings.CutSuffix(strings.TrimSpace(seg), AlongRepeat)
	return strings.TrimSpace(name), repeat
}

// MaxRollupPath is the most relations a roll-up's via may name. Each hop is
// one more join in the statement that computes the roll-up for a page, and
// the number of (entity, related entity) pairs it carries grows with the
// fan-out of every hop. Three covers the GRC paths (requirement -> control
// -> measure -> evidence); a longer chain is better modeled with a direct
// relation.
const MaxRollupPath = 3

// RollupNames returns the roll-up properties of entityType, sorted. The
// entity read surfaces (data-entry, MCP show_entity, CLI show and list) drop
// these names from a stored row before they add the computed values, so they
// never serve a value stored under a roll-up name. Other readers (Lua,
// search hits) see the stored row as it is.
func (m *Metamodel) RollupNames(entityType string) []string {
	def, ok := m.GetEntityDef(entityType)
	if !ok {
		return nil
	}
	var out []string
	for name, pd := range def.Properties {
		if pd.Rollup != nil {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Func returns the declared function and the related property it reads
// ("" for count and exists). For latest_by the property is the PICK; the
// ordering property is [RollupDef.LatestBy]. ok is false unless exactly one
// function is declared.
func (r *RollupDef) Func() (fn RollupFunc, property string, ok bool) {
	var found []RollupFunc
	if r.Count {
		found, fn = append(found, RollupCount), RollupCount
	}
	if r.Exists {
		found, fn = append(found, RollupExists), RollupExists
	}
	for _, c := range []struct {
		fn   RollupFunc
		prop string
	}{{RollupMin, r.Min}, {RollupMax, r.Max}, {RollupSum, r.Sum}, {RollupLatest, r.LatestBy},
		{RollupAvg, r.AvgOf}, {RollupLatestRow, r.LatestOf}, {RollupBodyTasks, r.BodyTasks}} {
		if strings.TrimSpace(c.prop) != "" {
			found, fn, property = append(found, c.fn), c.fn, c.prop
		}
	}
	if fn == RollupLatest {
		property = r.Pick
	}
	return fn, property, len(found) == 1
}

// validateRollupShape checks what the loader can check without the related
// type: exactly one function, latest_by with pick and pick only with
// latest_by, the declared type of count and exists, and no option a stored
// property has.
func validateRollupShape(schemaName, propName string, pd PropertyDef, allowRollup bool) []string {
	r := pd.Rollup
	if r == nil {
		return nil
	}
	at := fmt.Sprintf("%s: property %q: rollup", schemaName, propName)
	var errs []string
	if !allowRollup {
		errs = append(errs, at+" is only supported on entity properties")
	}
	switch {
	case r.BodyTasks != "":
		errs = append(errs, validateBodyTasksRollupShape(at, r, pd)...)
	case r.Records != "":
		errs = append(errs, validateRecordsRollupShape(at, r)...)
	case strings.TrimSpace(r.SiblingsBefore) != "":
		errs = append(errs, validateSiblingsRollupShape(at, r)...)
	case len(r.Along) > 0:
		errs = append(errs, validateAlongRollupShape(at, r)...)
	case len(r.WhenParent) > 0 || r.Climb:
		errs = append(errs, at+": when_parent and climb are only valid with siblings_before")
	case r.MaxDepth != 0:
		errs = append(errs, at+": max_depth is only valid with along")
	case len(r.Condition) > 0:
		errs = append(errs, validateViaConditionShape(at, r)...)
	case r.Window != "":
		errs = append(errs, at+": window is only valid with records")
	case r.AvgOf != "" || r.LatestOf != "":
		errs = append(errs, at+": avg and latest are only valid with records")
	case len(r.Via) == 0, len(r.Via) == 1 && strings.TrimSpace(r.Via[0]) == "":
		// A blank scalar via is a missing via, not an empty path element.
		errs = append(errs, at+" needs via: the relation to aggregate over")
	case len(r.Via) > MaxRollupPath:
		errs = append(errs, fmt.Sprintf("%s: via names %d relations; a roll-up follows at most %d",
			at, len(r.Via), MaxRollupPath))
	default:
		for i, name := range r.Via {
			if strings.TrimSpace(name) == "" {
				errs = append(errs, fmt.Sprintf("%s: via[%d] is empty", at, i))
			}
		}
	}
	fn, _, ok := r.Func()
	if !ok {
		errs = append(errs, at+" must declare exactly one of count, exists, min, max, sum, avg, latest, latest_by or body_tasks")
	}
	latestBy, pick := strings.TrimSpace(r.LatestBy) != "", strings.TrimSpace(r.Pick) != ""
	if latestBy && !pick {
		errs = append(errs, at+": latest_by needs pick: the property to read from the latest entity")
	}
	if pick && !latestBy {
		errs = append(errs, at+": pick is only valid with latest_by")
	}
	for _, name := range slices.Sorted(maps.Keys(r.Where)) {
		if strings.TrimSpace(r.Where[name]) == "" {
			errs = append(errs, fmt.Sprintf("%s: where %q has an empty value", at, name))
		}
	}
	if ok {
		switch {
		case fn == RollupCount && pd.Type != PropertyTypeInteger:
			errs = append(errs, fmt.Sprintf("%s: count needs type integer, not %q", at, pd.Type))
		case fn == RollupExists && pd.Type != PropertyTypeBoolean:
			errs = append(errs, fmt.Sprintf("%s: exists needs type boolean, not %q", at, pd.Type))
		}
	}
	for _, c := range []struct {
		set  bool
		name string
	}{
		{pd.Computed != "", "computed"},
		{pd.List, "list"},
		{pd.Required, "required"},
		{pd.Default != "", "default"},
		{pd.Unique, "unique"},
		{pd.Type == PropertyTypeFile, "a file type"},
	} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s cannot be combined with %s: a roll-up is computed on read and never stored",
				at, c.name))
		}
	}
	return errs
}

// validateRecordsRollupShape checks a roll-up over a record type: one of
// sum, avg or latest, a valid window, and none of the relation options.
func validateRecordsRollupShape(at string, r *RollupDef) []string {
	if len(r.Along) > 0 || strings.TrimSpace(r.SiblingsBefore) != "" {
		return []string{at + ": records reads the entity's own rows and cannot walk a path; " +
			"declare records or along/siblings_before, not both"}
	}
	var errs []string
	for _, c := range []struct {
		set  bool
		name string
	}{
		{len(r.Via) > 0, "via"}, {r.Type != "", "type"}, {len(r.Where) > 0, "where"},
		{r.Pick != "", "pick"}, {r.LatestBy != "", "latest_by"}, {r.Min != "", "min"},
		{r.Max != "", "max"}, {r.Count, "count"}, {r.Exists, "exists"},
		{len(r.Condition) > 0, "condition"}, {r.MaxDepth != 0, "max_depth"}, {r.Climb, "climb"},
		{len(r.WhenParent) > 0, "when_parent"},
	} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s: records cannot be combined with %s", at, c.name))
		}
	}
	if fn, _, ok := r.Func(); ok && fn != RollupSum && fn != RollupAvg && fn != RollupLatestRow {
		errs = append(errs, at+": a roll-up over records declares sum, avg or latest")
	}
	if _, err := ParseRetention(r.Window); err != nil {
		errs = append(errs, fmt.Sprintf("%s: window: %s", at, err))
	}
	return errs
}

// validateBodyTasksRollupShape checks a roll-up over the entity's own body:
// one of the counts, type integer, and no relation or record option.
func validateBodyTasksRollupShape(at string, r *RollupDef, pd PropertyDef) []string {
	var errs []string
	for _, c := range []struct {
		set  bool
		name string
	}{
		{len(r.Via) > 0, "via"}, {r.Type != "", "type"}, {len(r.Where) > 0, "where"},
		{r.Records != "", "records"}, {r.Window != "", "window"}, {len(r.Along) > 0, "along"},
		{r.SiblingsBefore != "", "siblings_before"}, {len(r.Condition) > 0, "condition"},
	} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s: body_tasks cannot be combined with %s", at, c.name))
		}
	}
	switch strings.TrimSpace(r.BodyTasks) {
	case BodyTasksDone, BodyTasksOpen, BodyTasksTotal:
	default:
		errs = append(errs, fmt.Sprintf("%s: body_tasks is %q; it must be done, open or total", at, r.BodyTasks))
	}
	if pd.Type != PropertyTypeInteger {
		errs = append(errs, fmt.Sprintf("%s: body_tasks needs type integer, not %q", at, pd.Type))
	}
	return errs
}

// validateAlongRollupShape checks an inherited roll-up's own shape: named
// segments, no far-end option, and a non-empty expression per condition
// entry. The checks that need the relations run in internal/rollup.
func validateAlongRollupShape(at string, r *RollupDef) []string {
	var errs []string
	for i, seg := range r.Along {
		if name, _ := ParseAlongSegment(seg); name == "" {
			errs = append(errs, fmt.Sprintf("%s: along[%d] names no relation", at, i))
		}
	}
	for _, c := range []struct {
		set  bool
		name string
	}{
		{len(r.Via) > 0, "via"}, {r.Type != "", "type"}, {len(r.Where) > 0, "where"},
		{r.Window != "", "window"}, {r.Pick != "", "pick"}, {r.Climb, "climb"},
		{len(r.WhenParent) > 0, "when_parent"},
	} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s: along cannot be combined with %s", at, c.name))
		}
	}
	if len(r.Condition) == 0 {
		errs = append(errs, at+": along needs a condition for at least one visited type")
	}
	return append(errs, conditionEntryErrors(at, r.Condition)...)
}

// conditionEntryErrors reports each condition entry with an empty type or
// expression.
func conditionEntryErrors(at string, cond map[string]string) []string {
	var errs []string
	for _, name := range slices.Sorted(maps.Keys(cond)) {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(cond[name]) == "" {
			errs = append(errs, fmt.Sprintf("%s: condition %q has an empty type or expression", at, name))
		}
	}
	return errs
}

// validateViaConditionShape checks a via roll-up with a per-type
// `condition:`: one relation, no where or type (the condition keys pick the
// related types), and a non-empty expression per entry.
func validateViaConditionShape(at string, r *RollupDef) []string {
	var errs []string
	if len(r.Via) != 1 || strings.TrimSpace(r.Via[0]) == "" {
		errs = append(errs, at+": a via with condition names exactly one relation")
	}
	for _, c := range []struct {
		set  bool
		name string
	}{{r.Type != "", "type"}, {len(r.Where) > 0, "where"}, {r.Pick != "", "pick"}} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s: a via with condition cannot be combined with %s", at, c.name))
		}
	}
	return append(errs, conditionEntryErrors(at, r.Condition)...)
}

// validateSiblingsRollupShape checks an earlier-siblings roll-up's own
// shape: no option of another roll-up form, a non-empty value per
// when_parent entry, and max_depth only with climb. The checks that need
// the relation run in internal/rollup.
func validateSiblingsRollupShape(at string, r *RollupDef) []string {
	var errs []string
	for _, c := range []struct {
		set  bool
		name string
	}{
		{len(r.Via) > 0, "via"}, {len(r.Along) > 0, "along"}, {r.Type != "", "type"},
		{r.Records != "", "records"}, {r.Window != "", "window"}, {r.Pick != "", "pick"},
		{len(r.Condition) > 0, "condition"},
	} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s: siblings_before cannot be combined with %s", at, c.name))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(r.WhenParent)) {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(r.WhenParent[name]) == "" {
			errs = append(errs, fmt.Sprintf("%s: when_parent %q has an empty property or value", at, name))
		}
	}
	if r.MaxDepth != 0 && !r.Climb {
		errs = append(errs, at+": max_depth is only valid with climb")
	}
	return errs
}
