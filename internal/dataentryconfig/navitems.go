package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateNavItemsFrom checks `items_from:` on every navigation entry, in the
// top-level tree and in every space. It is separate from validateNavEntry
// because the initial's property and relation are checked against the
// metamodel.
func validateNavItemsFrom(cfg *Config, meta *metamodel.Metamodel) []string {
	trees := NavigationTrees(cfg)
	errs := make([]string, 0, len(trees))
	for _, tree := range trees {
		var treeErrs []string
		for _, nav := range tree.Entries {
			treeErrs = append(treeErrs, navItemsFromErrors(nav, cfg, meta)...)
			for _, child := range nav.Items {
				treeErrs = append(treeErrs, navItemsFromErrors(child, cfg, meta)...)
			}
		}
		if tree.Space != "" {
			for i, e := range treeErrs {
				treeErrs[i] = fmt.Sprintf("spaces[%s]: %s", tree.Space, e)
			}
		}
		errs = append(errs, treeErrs...)
	}
	return errs
}

// navItemsFromErrors checks one entry's `items_from:`.
func navItemsFromErrors(nav NavigationEntry, cfg *Config, meta *metamodel.Metamodel) []string {
	from := nav.ItemsFrom
	if from == nil {
		return nil
	}
	if !nav.IsGroup() {
		return []string{fmt.Sprintf(
			"navigation %q: items_from is only supported on a group (it fills the group with entries)", nav.Label)}
	}
	prefix := fmt.Sprintf("navigation: group %q: items_from", nav.Group)
	var errs []string
	if len(nav.Items) > 0 {
		errs = append(errs, prefix+" and items cannot both be set (the entries come from the list)")
	}
	if from.Limit < 0 || from.Limit > NavItemsMaxLimit {
		errs = append(errs, fmt.Sprintf("%s.limit must be between 1 and %d, got %d", prefix, NavItemsMaxLimit, from.Limit))
	}
	if from.List == "" {
		return append(errs, prefix+".list is required")
	}
	list, ok := cfg.Lists[from.List]
	if !ok {
		return append(errs, fmt.Sprintf("%s: references unknown list %q", prefix, from.List))
	}
	errs = append(errs, navItemsPageErrors(prefix, from.Page, list.EntityType, cfg)...)
	if from.Create && list.CreateForm == "" && !hasCreateForm(cfg, list.EntityType) {
		errs = append(errs, fmt.Sprintf(
			"%s.create: list %q has no create_form and no form creates %q", prefix, from.List, list.EntityType))
	}
	if from.Initial != nil {
		errs = append(errs, navItemsInitialErrors(prefix+".initial", *from.Initial, list.EntityType, meta)...)
	}
	if from.Status != nil {
		errs = append(errs, navItemsStatusErrors(prefix+".status", *from.Status, list.EntityType, cfg, meta)...)
	}
	if from.Children != nil {
		errs = append(errs, navItemsChildrenErrors(prefix+".children", *from.Children, list.EntityType, cfg, meta)...)
	}
	return errs
}

// navItemsChildrenErrors checks `items_from.children`: a known list whose
// rows relate to the parent rows through `via`, an entity page for its type,
// a limit in range and an optional status.
func navItemsChildrenErrors(
	prefix string, c NavItemsChildren, parentType string, cfg *Config, meta *metamodel.Metamodel,
) []string {
	var errs []string
	if c.Limit < 0 || c.Limit > NavItemsMaxLimit {
		errs = append(errs, fmt.Sprintf("%s.limit must be between 1 and %d, got %d", prefix, NavItemsMaxLimit, c.Limit))
	}
	childType, ok := navItemsRelatedList(prefix, c.List, c.Via, c.Direction, parentType, cfg, meta, &errs)
	if !ok {
		return errs
	}
	errs = append(errs, navItemsPageErrors(prefix, c.Page, childType, cfg)...)
	if c.Status != nil {
		errs = append(errs, navItemsStatusErrors(prefix+".status", *c.Status, childType, cfg, meta)...)
	}
	return errs
}

// navItemsStatusErrors checks a generated entry's `status:`: a known list
// whose rows relate to the entry through `via`, a known tone and a label.
func navItemsStatusErrors(
	prefix string, st NavItemsStatus, entryType string, cfg *Config, meta *metamodel.Metamodel,
) []string {
	var errs []string
	switch st.Tone {
	case NavStatusToneNew, NavStatusToneInfo, NavStatusToneWarning, NavStatusToneError, NavStatusToneSuccess:
	default:
		errs = append(errs, fmt.Sprintf("%s: unknown tone %q (valid: %s, %s, %s, %s, %s)", prefix, st.Tone,
			NavStatusToneNew, NavStatusToneInfo, NavStatusToneWarning, NavStatusToneError, NavStatusToneSuccess))
	}
	if strings.TrimSpace(st.Label) == "" {
		errs = append(errs, prefix+": label is required")
	}
	navItemsRelatedList(prefix, st.List, st.Via, st.Direction, entryType, cfg, meta, &errs)
	return errs
}

// navItemsRelatedList checks a list named by children or status, and that
// `via` connects the list's rows (on their side, in dir) to otherType. It
// returns the list's entity type, and ok=false when the list is unusable.
func navItemsRelatedList(
	prefix, listID, via string, dir Direction, otherType string, cfg *Config, meta *metamodel.Metamodel,
	errs *[]string,
) (string, bool) {
	if listID == "" {
		*errs = append(*errs, prefix+".list is required")
		return "", false
	}
	list, ok := cfg.Lists[listID]
	if !ok {
		*errs = append(*errs, fmt.Sprintf("%s: references unknown list %q", prefix, listID))
		return "", false
	}
	if via == "" {
		*errs = append(*errs, prefix+".via is required")
		return list.EntityType, false
	}
	if meta == nil {
		return list.EntityType, true
	}
	def, ok := meta.GetRelationDef(via)
	if !ok {
		*errs = append(*errs, fmt.Sprintf("%s: relation %q not in metamodel", prefix, via))
		return list.EntityType, false
	}
	if e := CheckAmbiguousDirection(prefix, list.EntityType, via, dir, meta); e != nil {
		*errs = append(*errs, e...)
		return list.EntityType, false
	}
	resolved := NavItemsInitial{Relation: via, Direction: dir}.ResolvedDirection(list.EntityType, meta)
	rowSide, otherSide := def.From, def.To
	if resolved.IsIncoming() {
		rowSide, otherSide = def.To, def.From
	}
	if !slices.Contains(rowSide, list.EntityType) || !slices.Contains(otherSide, otherType) {
		*errs = append(*errs, fmt.Sprintf("%s: relation %q does not connect %q (list %q) to %q (from: %s; to: %s)",
			prefix, via, list.EntityType, listID, otherType, strings.Join(def.From, ", "), strings.Join(def.To, ", ")))
		return list.EntityType, false
	}
	return list.EntityType, true
}

// navItemsPageErrors checks `items_from.page`: an entity page for the list's
// entity type, since each entry opens that page for its row.
func navItemsPageErrors(prefix, pageID, rowType string, cfg *Config) []string {
	if pageID == "" {
		return nil
	}
	page, ok := cfg.Pages[pageID]
	switch {
	case !ok:
		return []string{fmt.Sprintf("%s: references unknown page %q", prefix, pageID)}
	case !page.IsEntityPage():
		return []string{fmt.Sprintf(
			"%s: page %q has no entity_type (an entry opens the page for its row, so it must be an entity page)",
			prefix, pageID)}
	case page.EntityType != rowType:
		return []string{fmt.Sprintf(
			"%s: page %q shows %q but the list shows %q", prefix, pageID, page.EntityType, rowType)}
	}
	return nil
}

// navItemsInitialErrors checks `items_from.initial`: exactly one source, a
// property of the row type or a relation that reaches the row type in the
// given direction.
func navItemsInitialErrors(prefix string, in NavItemsInitial, rowType string, meta *metamodel.Metamodel) []string {
	switch {
	case in.Property == "" && in.Relation == "":
		return []string{prefix + ": set property or relation"}
	case in.Property != "" && in.Relation != "":
		return []string{prefix + ": set property or relation, not both"}
	case in.Property != "" && in.Direction != "":
		return []string{prefix + ": direction only applies to a relation"}
	}
	if meta == nil {
		return nil
	}
	if in.Property != "" {
		def, ok := meta.GetEntityDef(rowType)
		if !ok {
			return nil // an unknown list type is reported with the list
		}
		if _, ok := def.Properties[in.Property]; !ok {
			return []string{fmt.Sprintf("%s: property %q not in metamodel for entity %q", prefix, in.Property, rowType)}
		}
		return nil
	}
	def, ok := meta.GetRelationDef(in.Relation)
	if !ok {
		return []string{fmt.Sprintf("%s: relation %q not in metamodel", prefix, in.Relation)}
	}
	if errs := CheckAmbiguousDirection(prefix, rowType, in.Relation, in.Direction, meta); errs != nil {
		return errs
	}
	dir := in.ResolvedDirection(rowType, meta)
	rowSide := def.From
	if dir.IsIncoming() {
		rowSide = def.To
	}
	if !slices.Contains(rowSide, rowType) {
		return []string{fmt.Sprintf("%s: relation %q does not connect %q %s (from: %s; to: %s)",
			prefix, in.Relation, rowType, dir, strings.Join(def.From, ", "), strings.Join(def.To, ", "))}
	}
	return nil
}
