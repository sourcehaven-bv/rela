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
	return errs
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
