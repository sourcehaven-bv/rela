package datamigration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A rename_entity_type step with `where:` or `ids:` renames a subset of the
// type. A file uses it to split one type over several, keeping every id:
//
//	- rename_entity_type: {from: taak, to: projecten.taak, where: {soort: [ontwikkeling]}}
//	- rename_entity_type: {from: taak, to: isms.taak}   # the rest
//
// Every rename of a type that has a filtered rename in the same file is part
// of the split. Those steps run on [renameEntityTypeStep.runSplit] rather than
// forEachEntity: they decide per id rather than per row, check the edges of
// the ids they move, and leave the ids an earlier filtered step claimed out
// of a dry run. A rename of a type that is not split keeps the plain path.

// listLimit caps the items an error lists.
const listLimit = 50

// renameWhere is the `where:` of a filtered rename: property name to the
// stored values that select an entity. Each key takes one value or a list.
//
// The shape follows the roll-up `where:` in schema.yaml (a mapping of
// property to value). It is not the list of filter strings that validations
// and schedules use under the same key, so that form gets its own error.
type renameWhere map[string][]string

// UnmarshalYAML refuses an empty value. It can arrive tagged null or, after
// decodeStrict re-encodes the step, as an empty string; either would read as
// "no filter" and rename every entity of the type.
func (w *renameWhere) UnmarshalYAML(node *yaml.Node) error {
	if node.ShortTag() == "!!null" || (node.Kind == yaml.ScalarNode && node.Value == "") {
		return errors.New("where: is empty; give a value or remove the key")
	}
	if node.Kind == yaml.SequenceNode {
		return errors.New("where: must be a mapping of property to value(s), like " +
			"`where: {soort: [a, b]}`; the filter-string list form (`where: [\"soort=a\"]`) is not supported here")
	}
	if node.Kind != yaml.MappingNode {
		return errors.New("where: must be a mapping of property to value(s)")
	}
	out := make(renameWhere, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, val := node.Content[i].Value, node.Content[i+1]
		switch val.Kind {
		case yaml.ScalarNode:
			out[key] = []string{val.Value}
		case yaml.SequenceNode:
			vals := make([]string, 0, len(val.Content))
			for _, item := range val.Content {
				if item.Kind != yaml.ScalarNode {
					return fmt.Errorf("where: %s: each value must be a scalar", key)
				}
				vals = append(vals, item.Value)
			}
			out[key] = vals
		default:
			return fmt.Errorf("where: %s: takes a value or a list of values", key)
		}
	}
	*w = out
	return nil
}

func (s *renameEntityTypeStep) filtered() bool { return s.Where != nil || s.IDs != nil }

// validateFilter checks `where:` and `ids:` against the from-schema. A
// typo'd property or enum value is a parse error, never a step that quietly
// matches nothing.
func (s *renameEntityTypeStep) validateFilter(from metamodel.ShapeProjection) error {
	if s.Where != nil && s.IDs != nil {
		return errors.New("where and ids cannot be combined in one step; " +
			"use a separate rename_entity_type step for each")
	}
	if s.IDs != nil {
		if len(s.IDs) == 0 {
			return errors.New("ids must list at least one id")
		}
		for _, id := range s.IDs {
			if strings.TrimSpace(id) == "" {
				return errors.New("ids must not contain an empty id")
			}
		}
		return nil
	}
	if s.Where == nil {
		return nil
	}
	if len(s.Where) == 0 {
		return errors.New("where must name at least one property")
	}
	for _, prop := range slices.Sorted(maps.Keys(s.Where)) {
		if err := validateWhereProperty(from, s.From, prop, s.Where[prop]); err != nil {
			return err
		}
	}
	return nil
}

// validateWhereProperty checks one where: key against the from-schema only:
// the step reads the names rows carry before the file runs, so a property an
// earlier rename_property in this file renamed goes by its old name.
func validateWhereProperty(from metamodel.ShapeProjection, typ, prop string, vals []string) error {
	ps, ok := from.Entities[typ].Properties[prop]
	if !ok {
		return fmt.Errorf("where: property %s.%s is not in the from-schema", typ, prop)
	}
	if len(vals) == 0 {
		return fmt.Errorf("where: %s needs at least one value", prop)
	}
	if ps.List {
		return fmt.Errorf("where: property %s.%s is list-typed; filter on a scalar property or use ids", typ, prop)
	}
	switch {
	case ps.Type == metamodel.PropertyTypeEnum || len(ps.Values) > 0 || from.Types[ps.Type] != nil:
		declared := enumValuesIn(from, typ, prop)
		for _, v := range vals {
			if !slices.Contains(declared, v) {
				return fmt.Errorf("where: %q is not a value of %s.%s (declared: %s); "+
					"reach rows holding undeclared values with ids",
					v, typ, prop, strings.Join(declared, ", "))
			}
		}
	case ps.Type == "string":
	case ps.Type == "boolean":
		for _, v := range vals {
			if v != "true" && v != "false" {
				return fmt.Errorf("where: %s.%s is boolean, so %q must be true or false", typ, prop, v)
			}
		}
	case ps.Type == "integer":
		for _, v := range vals {
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				return fmt.Errorf("where: %s.%s is an integer, so %q must be one", typ, prop, v)
			}
		}
	default:
		return fmt.Errorf("where: %s.%s has type %q; only string, enum, boolean and integer "+
			"properties can select rows (use ids otherwise)", typ, prop, ps.Type)
	}
	return nil
}

// validateSplits checks the rename_entity_type steps of one file as a set and
// marks the steps that take part in a split.
//
// Two orders are refused. A filtered rename of X after an unfiltered one can
// match nothing, because the unfiltered step already moved every row. And a
// filtered rename of X that no unfiltered rename of X follows leaves the rest
// at X; when the to-schema no longer declares X, that rest is orphaned data,
// which the GC sweep deletes after its grace period. A drop_entities does not
// count as the closing step: a filter that selects too little would then
// delete the rest at once, with no grace period.
func validateSplits(name string, f *File) error {
	unfilteredAt := map[string]int{}
	split := map[string]bool{}
	for i, st := range f.Steps {
		s, ok := st.(*renameEntityTypeStep)
		if !ok {
			continue
		}
		if !s.filtered() {
			if _, seen := unfilteredAt[s.From]; !seen {
				unfilteredAt[s.From] = i + 1
			}
			continue
		}
		split[s.From] = true
		if at, seen := unfilteredAt[s.From]; seen {
			return fmt.Errorf(
				"datamigration: %s: step %d (rename_entity_type) renames part of %q, but step %d already "+
					"renamed all of it, so this step would match nothing; put the filtered steps first",
				name, i+1, s.From, at)
		}
	}
	for _, typ := range slices.Sorted(maps.Keys(split)) {
		if _, ok := unfilteredAt[typ]; ok || entityInShape(f.ToProjection, typ) {
			continue
		}
		return fmt.Errorf(
			"datamigration: %s: the file renames only part of %q, and the to-schema no longer declares "+
				"it, so the rest would be orphaned and later deleted by GC; end the split with an "+
				"unfiltered `rename_entity_type: {from: %s, to: ...}`",
			name, typ, typ)
	}
	for _, st := range f.Steps {
		if s, ok := st.(*renameEntityTypeStep); ok && split[s.From] {
			s.split = true
		}
	}
	return nil
}

// runSplit renames the selected ids of a split type. A dry run writes
// nothing, so a later step of the split would still see the ids an earlier
// one moves; those are left out, so each step's count is what it would do on
// apply. The notes list every selected id, uncapped: a split decides per
// record which type it gets, so the output must say which records each step
// changed.
func (s *renameEntityTypeStep) runSplit(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}

	ids, rows, err := collectEntityFaces(ctx, x.Store, s.From)
	if err != nil {
		return res, err
	}
	if !x.Apply {
		ids = x.unclaimed(s.From, ids)
	}

	selected, err := s.selectIDs(ids, rows, &res)
	if err != nil {
		return res, err
	}
	if len(selected) == 0 {
		if s.filtered() {
			res.Notes = append(res.Notes, "matched no entities")
		}
		return res, nil
	}
	res.Notes = append(res.Notes, "selected: "+strings.Join(selected, ", "))
	if err := s.checkEdges(ctx, x.Store, selected); err != nil {
		return res, err
	}

	var changed []*entity.Entity
	for _, id := range selected {
		for _, e := range rows[id] {
			e.Type = s.To
			changed = append(changed, e)
		}
	}
	res.Affected = len(changed)
	if !x.Apply {
		if s.filtered() {
			x.claim(s.From, selected)
		}
		return res, nil
	}
	return res, x.writeBatches(ctx, changed)
}

// selectIDs returns the ids the step moves, in listing order. The decision
// is per id: every face row of an id must agree, because the rows of one id
// share one type.
func (s *renameEntityTypeStep) selectIDs(
	ids []string, rows map[string][]*entity.Entity, res *StepResult,
) ([]string, error) {
	if s.IDs != nil {
		want := map[string]bool{}
		for _, id := range s.IDs {
			want[id] = true
		}
		var selected []string
		for _, id := range ids {
			if want[id] {
				selected = append(selected, id)
				delete(want, id)
			}
		}
		for _, id := range slices.Sorted(maps.Keys(want)) {
			res.Notes = append(res.Notes, fmt.Sprintf("%s: not of type %q (already renamed or absent)", id, s.From))
		}
		return selected, nil
	}
	if s.Where == nil {
		return ids, nil
	}
	var selected, mixed []string
	for _, id := range ids {
		first := s.matches(rows[id][0])
		agree := true
		for _, e := range rows[id][1:] {
			if s.matches(e) != first {
				agree = false
				break
			}
		}
		switch {
		case !agree:
			mixed = append(mixed, id)
		case first:
			selected = append(selected, id)
		}
	}
	if len(mixed) > 0 {
		return nil, fmt.Errorf(
			"the content states of %s disagree on the where: properties, so the step cannot tell "+
				"which type they belong to; nothing was renamed. Move them with an earlier "+
				"`rename_entity_type: {from: %s, to: ..., ids: [...]}` step",
			limitList(mixed), s.From)
	}
	return selected, nil
}

// matches reports whether every where: property holds one of its values.
func (s *renameEntityTypeStep) matches(e *entity.Entity) bool {
	for prop, vals := range s.Where {
		v, ok := canonicalScalar(e.Properties[prop])
		if !ok || !slices.Contains(vals, v) {
			return false
		}
	}
	return true
}

// canonicalScalar is the string a where: value is compared with. A list or
// an absent value never matches.
func canonicalScalar(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case bool:
		return strconv.FormatBool(val), true
	case int:
		return strconv.Itoa(val), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10), true
		}
		return strconv.FormatFloat(val, 'f', -1, 64), true
	}
	return "", false
}

// checkEdges refuses the step when a moved id sits on an edge whose type, in
// the to-schema, does not allow the new type at that end. Variants can narrow
// a relation to one of them, so a split can put an entity on the wrong side
// of one.
//
// Only the moved end is checked. The other end may still be waiting for a
// later step of the split, at a type the to-schema no longer declares. An
// edge whose type the to-schema does not declare is left to the step that
// renames or drops that type.
func (s *renameEntityTypeStep) checkEdges(ctx context.Context, st store.Store, selected []string) error {
	moved := make(map[string]bool, len(selected))
	for _, id := range selected {
		moved[id] = true
	}
	var bad []string
	q := store.RelationQuery{EntityIDs: selected, Direction: store.DirectionBoth}
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return err
		}
		shape, ok := s.toRelations[r.Type]
		if !ok {
			continue
		}
		if moved[r.From] && !slices.Contains(shape.From, s.To) {
			bad = append(bad, fmt.Sprintf("%s -%s-> %s (%q not allowed as source)", r.From, r.Type, r.To, s.To))
		}
		if moved[r.To] && !slices.Contains(shape.To, s.To) {
			bad = append(bad, fmt.Sprintf("%s -%s-> %s (%q not allowed as target)", r.From, r.Type, r.To, s.To))
		}
	}
	if len(bad) == 0 {
		return nil
	}
	slices.Sort(bad)
	return fmt.Errorf("renaming would leave %d relation(s) the to-schema does not allow, so nothing "+
		"was renamed; move these entities to another type or remove the relations first: %s",
		len(bad), limitList(bad))
}

// limitList joins items, cut off after listLimit.
func limitList(items []string) string {
	if len(items) <= listLimit {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:listLimit], ", ") + fmt.Sprintf(" (and %d more)", len(items)-listLimit)
}
