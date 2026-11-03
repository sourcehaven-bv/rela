package dataentryconfig

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// fieldsByTypeDisplays are the flat display modes that render `fields:`, and
// so the ones `fields_by_type` may refine.
var fieldsByTypeDisplays = map[string]bool{"properties": true, "list": true, "cards": true, "content": true}

// validateMentionsStep checks one `mentioned_by` traverse step (TKT-HF0D5W).
//
// The step reads the entry's backlinks, so it starts from `entry` only: from
// any other collection it would cost one index read per row. It walks no
// relation, so every relation-walk key is refused rather than ignored.
//
// Its `where:` must parse, as on every traverse step ([validateTraverseWhere]).
func validateMentionsStep(viewID string, i int, t ViewTraverse, view ViewConfig) []string {
	var errs []string
	prefix := fmt.Sprintf("view %q: traverse[%d]", viewID, i)
	if t.From != "entry" {
		errs = append(errs, fmt.Sprintf("%s sets mentioned_by, which takes from: entry only (got %q)", prefix, t.From))
	}
	for _, k := range []struct {
		key string
		set bool
	}{
		{"follow", t.Follow != ""},
		{"follow_incoming", t.FollowIncoming != ""},
		{"recursive", t.Recursive},
		{"max_depth", t.MaxDepth != 0},
	} {
		if k.set {
			errs = append(errs, fmt.Sprintf("%s sets mentioned_by, which cannot be combined with %s", prefix, k.key))
		}
	}
	if t.CollectAs == "" {
		errs = append(errs, prefix+" must specify collect_as")
	} else {
		for j, other := range view.Traverse {
			if j != i && other.CollectAs == t.CollectAs {
				errs = append(errs, fmt.Sprintf(
					"%s collects backlinks into %q, which traverse[%d] also collects into "+
						"(a mentioned_by step needs its own collection)", prefix, t.CollectAs, j))
				break
			}
		}
	}
	return append(errs, validateTraverseWhere(prefix, t)...)
}

// validateTraverseWhere refuses a traverse `where:` that does not parse
// (BUG-WHEREWIDE). The executor used to drop such a clause and keep every
// traversed entity, so a typo widened a step whose only job is to narrow.
func validateTraverseWhere(prefix string, t ViewTraverse) []string {
	if t.Where == "" {
		return nil
	}
	if _, err := filter.Parse(t.Where); err != nil {
		return []string{fmt.Sprintf("%s has an invalid where: %v", prefix, err)}
	}
	return nil
}

// validateMentionsUsed refuses a backlinks collection no section shows. Such
// a view would still hide the built-in "Mentioned by" panel, so the backlinks
// would vanish from the page without an error. A later traverse step reading
// the collection does not count: it shows what the backlinks lead to, not the
// backlinks.
func validateMentionsUsed(viewID string, view ViewConfig) []string {
	used := map[string]bool{}
	for _, s := range view.Sections {
		used[s.Source] = true
		if s.Children != "" {
			used[s.Children] = true
		}
	}
	var errs []string
	for i, t := range view.Traverse {
		if t.MentionedBy && t.CollectAs != "" && !used[t.CollectAs] {
			errs = append(errs, fmt.Sprintf(
				"view %q: traverse[%d] collects backlinks into %q, but no section shows them "+
					"(the built-in Mentioned by panel is hidden for this view)", viewID, i, t.CollectAs))
		}
	}
	return errs
}

// isMentionsCollection reports whether a mentioned_by step fills name.
func isMentionsCollection(view ViewConfig, name string) bool {
	for _, t := range view.Traverse {
		if t.MentionedBy && t.CollectAs == name {
			return true
		}
	}
	return false
}

// validateSectionByType checks `fields_by_type` and `columns_by_type` on one
// section, and the fallback `fields:`/`columns:` of a mixed collection.
//
// The rules follow [validateLevelColumns]: a key must be a type the
// collection can hold, and an entry must name a property of its own type.
// A fallback list applies to the types the map leaves out, so each of its
// properties must be declared by at least one of them.
//
// The fallback check runs only where the author opted into per-type lists
// or reads a backlinks collection. Applying it to every mixed relation
// section would start refusing configs that load today.
func validateSectionByType(viewID string, i int, s ViewSection, view ViewConfig, meta *metamodel.Metamodel) []string {
	prefix := fmt.Sprintf("view %q: section[%d]", viewID, i)
	if errs := validateByTypeShape(prefix, s); len(errs) > 0 {
		return errs
	}
	allowed := collectionTypes(view, s.Source, meta)
	errs := validateFieldsByType(prefix, s.FieldsByType, allowed, meta)
	for _, t := range sortedMapKeys(s.ColumnsByType) {
		if len(s.ColumnsByType[t]) == 0 {
			errs = append(errs, fmt.Sprintf("%s columns_by_type[%q] is empty (omit the type to use columns:)", prefix, t))
		}
	}
	errs = append(errs, validateLevelColumns(viewID, i, "columns_by_type", s.ColumnsByType, allowed, meta)...)

	if s.FieldsByType == nil && s.ColumnsByType == nil && !isMentionsCollection(view, s.Source) {
		return errs
	}
	return append(errs, validateByTypeFallback(prefix, s, allowed, meta)...)
}

// validateByTypeShape refuses a per-type map on a display or source that
// cannot use it.
func validateByTypeShape(prefix string, s ViewSection) []string {
	var errs []string
	if s.FieldsByType != nil && !fieldsByTypeDisplays[s.Display] {
		errs = append(errs, fmt.Sprintf("%s sets fields_by_type, which display %q does not take "+
			"(use it with list, cards, content or properties)", prefix, s.Display))
	}
	if s.ColumnsByType != nil {
		if s.Display != "table" {
			errs = append(errs, fmt.Sprintf("%s sets columns_by_type, which display %q does not take "+
				"(use it with table)", prefix, s.Display))
		}
		if s.GroupBy != "" {
			errs = append(errs, prefix+" sets both columns_by_type and group_by "+
				"(columns_by_type already groups the table by type)")
		}
	}
	if (s.FieldsByType != nil || s.ColumnsByType != nil) && s.Source == "entry" {
		errs = append(errs, prefix+" sets a per-type list on source: entry, which holds one entity")
	}
	return errs
}

// validateFieldsByType checks each `fields_by_type` entry against its type.
func validateFieldsByType(
	prefix string, byType map[string][]ViewSectionField, allowed []string, meta *metamodel.Metamodel,
) []string {
	keys := sortedMapKeys(byType)
	errs := validateFieldTypeKeys(prefix, keys, allowed, meta)
	for _, t := range keys {
		def, ok := meta.GetEntityDef(t)
		if !ok {
			continue
		}
		if len(byType[t]) == 0 {
			errs = append(errs, fmt.Sprintf("%s fields_by_type[%q] is empty (omit the type to use fields:)", prefix, t))
		}
		for j, f := range byType[t] {
			if !declaresProperty(def, f.Property) {
				errs = append(errs, fmt.Sprintf("%s fields_by_type[%q] field[%d] property %q not in entity %q",
					prefix, t, j, f.Property, t))
			}
			errs = append(errs, validateSpan(f.Span, fmt.Sprintf("%s fields_by_type[%q] field[%d]", prefix, t, j))...)
		}
	}
	return errs
}

// validateByTypeFallback checks the section's `fields:`/`columns:` against
// the types the per-type map leaves to them.
func validateByTypeFallback(prefix string, s ViewSection, allowed []string, meta *metamodel.Metamodel) []string {
	var errs []string
	uncovered := uncoveredTypes(allowed, s.FieldsByType, s.ColumnsByType, s.Display)
	if s.ColumnsByType != nil && len(uncovered) > 0 && len(s.Columns) == 0 {
		errs = append(errs, fmt.Sprintf("%s columns_by_type does not cover %s, so it needs a columns: fallback",
			prefix, strings.Join(uncovered, ", ")))
	}
	if len(uncovered) == 0 {
		return errs
	}
	for j, f := range s.Fields {
		if !declaredBySome(meta, uncovered, f.Property) {
			errs = append(errs, fmt.Sprintf("%s field[%d] property %q is declared by none of the types it applies to (%s)",
				prefix, j, f.Property, strings.Join(uncovered, ", ")))
		}
	}
	for j, c := range s.Columns {
		if c.Property != "" && !declaredBySome(meta, uncovered, c.Property) {
			errs = append(errs, fmt.Sprintf("%s column[%d] property %q is declared by none of the types it applies to (%s)",
				prefix, j, c.Property, strings.Join(uncovered, ", ")))
		}
	}
	return errs
}

// validateFieldTypeKeys checks that each fields_by_type key names a declared
// entity type the collection can hold. allowed nil means the set is not
// statically known. columns_by_type gets the same check from
// [validateLevelColumns].
func validateFieldTypeKeys(prefix string, keys, allowed []string, meta *metamodel.Metamodel) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, t := range allowed {
		allowedSet[t] = true
	}
	var errs []string
	for _, t := range keys {
		if _, ok := meta.GetEntityDef(t); !ok {
			errs = append(errs, fmt.Sprintf("%s fields_by_type references unknown entity type %q", prefix, t))
			continue
		}
		if len(allowed) > 0 && !allowedSet[t] {
			errs = append(errs, fmt.Sprintf("%s fields_by_type declares fields for %q, which this collection never holds (valid: %s)",
				prefix, t, strings.Join(allowed, ", ")))
		}
	}
	return errs
}

// uncoveredTypes returns the allowed types the display's per-type map omits.
func uncoveredTypes(
	allowed []string, fields map[string][]ViewSectionField, columns map[string][]ListColumn, display string,
) []string {
	var out []string
	for _, t := range allowed {
		if display == "table" {
			if _, ok := columns[t]; ok {
				continue
			}
		} else if _, ok := fields[t]; ok {
			continue
		}
		out = append(out, t)
	}
	return out
}

// declaresProperty reports whether def declares prop. "title", "id" and the
// empty name are accepted everywhere, as on `fields:` and `columns:`.
func declaresProperty(def *metamodel.EntityDef, prop string) bool {
	if prop == "" || prop == "title" || prop == "id" {
		return true
	}
	_, ok := def.Properties[prop]
	return ok
}

// declaredBySome reports whether at least one of types declares prop.
func declaredBySome(meta *metamodel.Metamodel, types []string, prop string) bool {
	for _, t := range types {
		if def, ok := meta.GetEntityDef(t); ok && declaresProperty(def, prop) {
			return true
		}
	}
	return false
}

// allEntityTypes is the type set of a backlinks collection: any entity can
// mention any other.
func allEntityTypes(meta *metamodel.Metamodel) []string {
	types := meta.EntityTypes()
	sort.Strings(types)
	return types
}

// validateSidePanelMentions refuses the backlink, per-type and `condition:`
// keys in a form's side panel, and a `where:` that does not parse. Side panels
// share the view types but are not validated like views, so these keys would
// otherwise be accepted and misapplied.
func validateSidePanelMentions(formID string, panel *SidePanelConfig) []string {
	if panel == nil {
		return nil
	}
	var errs []string
	for i, t := range panel.Traverse {
		if t.MentionedBy {
			errs = append(errs, fmt.Sprintf("form %q: side_panel traverse[%d] sets mentioned_by, "+
				"which side panels do not support (use it in a view)", formID, i))
		}
		if t.Condition != "" {
			errs = append(errs, fmt.Sprintf("form %q: side_panel traverse[%d] sets condition, "+
				"which side panels do not support (use where, or a view)", formID, i))
		}
		errs = append(errs, validateTraverseWhere(fmt.Sprintf("form %q: side_panel traverse[%d]", formID, i), t)...)
	}
	for i, s := range panel.Sections {
		if s.FieldsByType != nil || s.ColumnsByType != nil {
			errs = append(errs, fmt.Sprintf("form %q: side_panel section[%d] sets a per-type list, "+
				"which side panels do not support (use it in a view)", formID, i))
		}
	}
	return errs
}
