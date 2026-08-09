package dataentryconfig

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// Valid top-level keys in data-entry.yaml
var validTopLevelKeys = map[string]bool{
	"version":      true,
	"app":          true,
	"git":          true,
	"styles":       true,
	"forms":        true,
	"lists":        true,
	"views":        true,
	"entity_views": true,
	"kanbans":      true,
	"documents":    true,
	"feeds":        true,
	"dashboard":    true,
	"commands":     true,
	"actions":      true,
	"navigation":   true,
	"palette":      true,
}

// Known typos with suggestions
var knownTypos = map[string]string{
	"form":        "forms",
	"list":        "lists",
	"view":        "views",
	"kanban":      "kanbans",
	"command":     "commands",
	"style":       "styles",
	"nav":         "navigation",
	"navaigation": "navigation",
}

// Valid filter operators for list/kanban static filters. This is the UI
// operator set the SPA can translate to a wire operator the API evaluates
// (utils/filters.ts OPERATOR_MAP → eq/ne/contains/in/lt/lte/gt/gte) — the
// docs table in docs/data-entry.md "Static Filters" is the same set. `=~`
// (regex, from search/calfeed/CLI filter syntax) was wrongly allowed here
// for months while no layer below could evaluate it: the SPA degraded it
// to `eq` and the config's list silently showed zero rows.
var validFilterOperators = map[string]bool{
	"=":  true,
	"==": true, // alias for "="
	"!=": true,
	"~":  true, // substring, case-insensitive
	"<":  true,
	"<=": true,
	">":  true,
	">=": true,
	"in": true, // comma-separated list, matches any
}

// Valid sort directions
var validSortDirections = map[string]bool{
	"":     true, // default (asc)
	"asc":  true,
	"desc": true,
}

// Valid display modes for view sections
var validSectionDisplayModes = map[string]bool{
	"properties": true,
	"content":    true,
	"table":      true,
	"list":       true,
	"cards":      true,
	"breakdown":  true,
}

// Valid display modes for dashboard cards
var validDashboardDisplayModes = map[string]bool{
	"count":     true,
	"table":     true,
	"breakdown": true,
}

// Valid command contexts
var validCommandContexts = map[string]bool{
	"entity": true,
	"list":   true,
	"view":   true,
	"global": true,
}

// Valid relation directions
var validRelationDirections = map[Direction]bool{
	"":                true, // default (outgoing)
	DirectionOutgoing: true,
	DirectionIncoming: true,
}

// Valid relation widgets
var validRelationWidgets = map[string]bool{
	"":                true, // default (auto-detect from cardinality)
	WidgetSelect:      true,
	WidgetMultiSelect: true,
	WidgetCards:       true,
}

// ValidateConfig performs comprehensive validation of a data-entry config.
// It returns a ConfigValidationError containing all validation issues,
// or nil if the config is valid.
func ValidateConfig(data []byte, cfg *Config, meta *metamodel.Metamodel) error {
	errs := make([]string, 0) //nolint:prealloc // total capacity unknown; aggregated from multiple helpers

	// Phase 1: Structural validation (unknown keys)
	errs = append(errs, checkUnknownKeys(data)...)

	// Phase 2: Semantic validation (cross-references, types, etc.)
	errs = append(errs, validateNavigation(cfg)...)
	errs = append(errs, validateForms(cfg, meta)...)
	errs = append(errs, validateLists(cfg, meta)...)
	errs = append(errs, validateViews(cfg, meta)...)
	errs = append(errs, validateEntityViews(cfg, meta)...)
	errs = append(errs, validateKanbans(cfg, meta)...)
	errs = append(errs, validateDashboard(cfg, meta)...)
	errs = append(errs, validateCommands(cfg, meta)...)
	errs = append(errs, validateActions(cfg, meta)...)
	errs = append(errs, validateApp(cfg)...)
	errs = append(errs, validateDocuments(cfg)...)
	errs = append(errs, validateFeeds(cfg, meta)...)
	errs = append(errs, validateStyles(cfg, meta)...)
	errs = append(errs, validateCrossReferences(cfg)...)

	if len(errs) > 0 {
		natsort.Strings(errs)
		return &ConfigValidationError{Errors: errs}
	}
	return nil
}

// checkUnknownKeys detects unknown top-level keys in the config YAML.
func checkUnknownKeys(data []byte) []string {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // struct unmarshal already caught this
	}

	var errs []string
	for key := range raw {
		if validTopLevelKeys[key] {
			continue
		}
		if suggestion, ok := knownTypos[key]; ok {
			errs = append(errs, fmt.Sprintf("unknown key %q (did you mean %q?)", key, suggestion))
		} else {
			keys := sortedMapKeys(validTopLevelKeys)
			errs = append(errs, fmt.Sprintf("unknown key %q (valid keys: %s)", key, strings.Join(keys, ", ")))
		}
	}
	return errs
}

// validateNavigation validates navigation entries.
func validateNavigation(cfg *Config) []string {
	var errs []string

	for _, nav := range cfg.Navigation {
		if nav.IsGroup() {
			for _, child := range nav.Items {
				if child.IsGroup() {
					errs = append(errs, fmt.Sprintf(
						"navigation: group %q contains nested group %q (nested groups are not supported)",
						nav.Group, child.Group))
				}
			}
		}
	}

	// Validate list references in navigation
	for _, nav := range cfg.Navigation {
		errs = append(errs, validateNavEntry(nav, cfg)...)
	}

	return errs
}

func validateNavEntry(nav NavigationEntry, cfg *Config) []string {
	var errs []string

	if nav.List != "" {
		if _, ok := cfg.Lists[nav.List]; !ok {
			errs = append(errs, fmt.Sprintf(
				"navigation: references unknown list %q", nav.List))
		}
	}

	if nav.Kanban != "" {
		if _, ok := cfg.Kanbans[nav.Kanban]; !ok {
			errs = append(errs, fmt.Sprintf(
				"navigation: references unknown kanban %q", nav.Kanban))
		}
	}

	if nav.Action != "" {
		if _, ok := cfg.Actions[nav.Action]; !ok {
			errs = append(errs, fmt.Sprintf(
				"navigation: references unknown action %q", nav.Action))
		}
	}

	if nav.Document != "" {
		doc, ok := cfg.Documents[nav.Document]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf(
				"navigation: references unknown document %q", nav.Document))
		case !doc.IsStandalone():
			// An entity-anchored document needs an entry id in its URL, and a
			// sidebar entry has no entity to supply one. Rejecting at config
			// load beats emitting a link that always 400s.
			errs = append(errs, fmt.Sprintf(
				"navigation: document %q has entity_type %q so it cannot be a navigation entry "+
					"(only documents without entity_type can; they render at /document/%s)",
				nav.Document, doc.EntityType, nav.Document))
		}
	}

	if nav.IsGroup() {
		// A group is a container, not a destination — there is nothing for a
		// permission to gate, and a gated group would be ambiguous with the
		// empty-group rule (a group disappears on its own once every child is
		// filtered out). Rejecting is clearer than silently ignoring it.
		if nav.Permission != "" {
			errs = append(errs, fmt.Sprintf(
				"navigation: group %q cannot have a permission (set it on the items instead; "+
					"a group is hidden automatically when all its items are)", nav.Group))
		}
		for _, child := range nav.Items {
			errs = append(errs, validateNavEntry(child, cfg)...)
		}
	}

	return errs
}

// validateSidePanelSpans checks `span` on a form's side-panel section fields.
//
// Side panels reuse ViewSection/ViewSectionField and render through the same
// buildSections path as a view, so a span authored there reaches the wire — but
// no other validator descends into form.SidePanel, so it would otherwise go
// unchecked. Nil panel is the common case and yields nothing.
func validateSidePanelSpans(formID string, panel *SidePanelConfig) []string {
	if panel == nil {
		return nil
	}
	var errs []string
	for i, sec := range panel.Sections {
		for j, f := range sec.Fields {
			errs = append(errs, validateSpan(f.Span,
				fmt.Sprintf("form %q: side_panel section[%d] field[%d]", formID, i, j))...)
		}
	}
	return errs
}

// validateForms validates form definitions.
func validateForms(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for formID, form := range cfg.Forms {
		entDef, ok := meta.GetEntityDef(form.EntityType)
		if !ok {
			errs = append(errs, fmt.Sprintf("form %q: unknown entity type %q", formID, form.EntityType))
			continue
		}

		// A form is single-page OR a wizard, never both.
		hasFlat := len(form.Fields) > 0 || len(form.Relations) > 0
		if len(form.Steps) > 0 && hasFlat {
			errs = append(errs, fmt.Sprintf(
				"form %q: a form may define either top-level fields/relations OR steps, not both",
				formID))
		}

		// Flat (single-page) fields/relations.
		for i, f := range form.Fields {
			errs = append(errs, validateFormField(formID, "", i, f, form.EntityType, entDef, meta)...)
		}
		for i, r := range form.Relations {
			errs = append(errs, validateFormRelation(formID, "", i, r, form.EntityType, meta)...)
		}

		// Wizard steps: each step's fields/relations reuse the same checks.
		for si, step := range form.Steps {
			ctx := fmt.Sprintf("step[%d] ", si)
			if step.Title == "" {
				errs = append(errs, fmt.Sprintf("form %q: %shas no title", formID, ctx))
			}
			for i, f := range step.Fields {
				errs = append(errs, validateFormField(formID, ctx, i, f, form.EntityType, entDef, meta)...)
			}
			for i, r := range step.Relations {
				errs = append(errs, validateFormRelation(formID, ctx, i, r, form.EntityType, meta)...)
			}
		}

		// Side-panel sections carry ViewSectionField, the same struct used by
		// views — so they carry `span` too, and executeSidePanel renders them
		// through the same buildSections path. Nothing else in this file
		// descends into form.SidePanel, so without this a bad span there is
		// accepted in silence: exactly the failure validateSpan exists to
		// prevent, on a real config path.
		//
		// Scoped to span deliberately. Side-panel property names have never
		// been validated either, and fixing that is a wider behavior change
		// (it could start rejecting configs that load today) that belongs in
		// its own ticket rather than riding along with a layout PR.
		errs = append(errs, validateSidePanelSpans(formID, form.SidePanel)...)
	}

	return errs
}

// validateFormField checks one form field against the metamodel. ctx is an
// optional location prefix (e.g. "step[0] ") so wizard and flat forms share
// the rule set while keeping distinct error messages.
func validateFormField(
	formID, ctx string, i int, f FormField, entityType string,
	entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	var errs []string
	if _, ok := entDef.Properties[f.Property]; !ok {
		errs = append(errs, fmt.Sprintf(
			"form %q: %sfield[%d] property %q not in metamodel for entity %q",
			formID, ctx, i, f.Property, entityType))
	}
	if len(f.Transitions) > 0 {
		if propDef, hasProp := entDef.Properties[f.Property]; hasProp {
			errs = append(errs, validateTransitions(formID, i, f, propDef, meta)...)
		}
	}
	errs = append(errs, validateSpan(f.Span, fmt.Sprintf("form %q: %sfield[%d]", formID, ctx, i))...)
	return errs
}

// validateFormRelation checks one form relation against the metamodel. ctx is
// an optional location prefix (see validateFormField).
func validateFormRelation(
	formID, ctx string, i int, r FormRelation, entityType string, meta *metamodel.Metamodel,
) []string {
	var errs []string

	// A span on a relation is a config mistake, not a layout instruction:
	// RelationCards / RelationPicker never read it, so it would be silently
	// discarded. Saying so beats leaving the author to conclude the feature
	// is broken. See FormRelation.Span.
	if r.Span != 0 {
		errs = append(errs, fmt.Sprintf(
			"form %q: %srelation[%d] cannot have a span (relation widgets always take the full row)",
			formID, ctx, i))
	}

	relDef, ok := meta.GetRelationDef(r.Relation)
	switch {
	case ok:
		// Canonical name resolved — check that the form's entity type is on
		// the correct side of the edge for the chosen direction. Wrong-side
		// configs are silently broken otherwise: the widget searches the wrong
		// target type and never shows existing edges.
		errs = append(errs, validateFormRelationSide(formID, i, entityType, r, relDef)...)
	default:
		if canonical, isInverse := meta.InverseOwner(r.Relation); isInverse {
			errs = append(errs, fmt.Sprintf(
				"form %q: %srelation[%d] uses inverse name %q; use `relation: %s` with `direction: incoming` to bind the inverse of %q",
				formID, ctx, i, r.Relation, canonical, canonical))
		} else {
			errs = append(errs, fmt.Sprintf(
				"form %q: %srelation[%d] references unknown relation %q",
				formID, ctx, i, r.Relation))
		}
	}

	if !validRelationDirections[r.Direction] {
		errs = append(errs, fmt.Sprintf(
			"form %q: %srelation[%d] has invalid direction %q (valid: outgoing, incoming)",
			formID, ctx, i, r.Direction))
	}

	if !validRelationWidgets[r.Widget] {
		errs = append(errs, fmt.Sprintf(
			"form %q: %srelation[%d] has invalid widget %q (valid: select, multi-select, cards)",
			formID, ctx, i, r.Widget))
	}

	return errs
}

// validateFormRelationSide checks that the form's entity type sits on
// the side of the relation that matches the chosen direction. An
// outgoing relation must be authored from a `From:` type; an incoming
// one must be authored from a `To:` type. Mismatch returns an error;
// when the entity is on the opposite side the message hints at
// flipping the direction.
func validateFormRelationSide(
	formID string, i int, entityType string, r FormRelation, relDef *metamodel.RelationDef,
) []string {
	if entityType == "" {
		return nil
	}
	incoming := r.Direction.IsIncoming()
	expected, opposite := relDef.From, relDef.To
	expectedSide, oppositeSide := "from", "to"
	flipDir := DirectionIncoming
	if incoming {
		expected, opposite = relDef.To, relDef.From
		expectedSide, oppositeSide = "to", "from"
		flipDir = DirectionOutgoing
	}
	if slices.Contains(expected, entityType) {
		return nil
	}
	hint := ""
	if r.Direction == "" && slices.Contains(opposite, entityType) {
		hint = fmt.Sprintf(" (set `direction: %s` to bind the %s side of %q)", flipDir, oppositeSide, r.Relation)
	}
	return []string{fmt.Sprintf(
		"form %q: relation[%d] entity type %q is not a %s of relation %q (valid: %s)%s",
		formID, i, entityType, expectedSide, r.Relation, strings.Join(expected, ", "), hint)}
}

// validateTransitions checks that transition values are valid for the property's type.
func validateTransitions(
	formID string, fieldIdx int, field FormField, propDef metamodel.PropertyDef, meta *metamodel.Metamodel,
) []string {
	var errs []string

	// Get valid values for this property type
	validValues := GetValidEnumValues(propDef, meta)
	if len(validValues) == 0 {
		return errs // not an enum type, skip validation
	}

	validSet := make(map[string]bool)
	for _, v := range validValues {
		validSet[v] = true
	}

	// Check all transition keys and values
	for fromState, toStates := range field.Transitions {
		if !validSet[fromState] {
			errs = append(errs, fmt.Sprintf(
				"form %q: field[%d] transitions has invalid from-state %q",
				formID, fieldIdx, fromState))
		}
		for _, toState := range toStates {
			if !validSet[toState] {
				errs = append(errs, fmt.Sprintf(
					"form %q: field[%d] transitions has invalid to-state %q",
					formID, fieldIdx, toState))
			}
		}
	}

	return errs
}

// GetValidEnumValues returns the valid values for an enum or custom type property.
func GetValidEnumValues(propDef metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	if propDef.Type == metamodel.PropertyTypeEnum {
		return propDef.Values
	}
	// Check if it's a custom type
	if customType, ok := meta.Types[propDef.Type]; ok {
		return customType.Values
	}
	return nil
}

// validateEntityViews validates entity_views entries: each key must be a known
// entity type, and each detail_view must reference an existing view.
func validateEntityViews(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string
	for entityType, ev := range cfg.EntityViews {
		if _, ok := meta.GetEntityDef(entityType); !ok {
			errs = append(errs, fmt.Sprintf(
				"entity_views: unknown entity type %q", entityType))
		}
		if ev.DetailView == "" {
			errs = append(errs, fmt.Sprintf(
				"entity_views[%q]: detail_view is empty (omit the entry instead)", entityType))
			continue
		}
		if _, ok := cfg.Views[ev.DetailView]; ok {
			continue
		}
		if suggestion := suggestView(ev.DetailView, cfg); suggestion != "" {
			errs = append(errs, fmt.Sprintf(
				"entity_views[%q]: references unknown view %q in detail_view (did you mean %q?)",
				entityType, ev.DetailView, suggestion))
		} else {
			errs = append(errs, fmt.Sprintf(
				"entity_views[%q]: references unknown view %q in detail_view",
				entityType, ev.DetailView))
		}
	}
	return errs
}

// validateExportRenderShape validates an `export_render:` script path's shape.
// Shape only — whether the file exists on disk is checked at app construction,
// which is where the project root is known (see dataentry.NewApp).
//
// Shared by the list and view overrides so the two cannot fail differently:
// before this, a typo'd list path failed here while the identical typo on a
// view fell through to a generic loader error at boot. kind/id name the owner
// for the message ("list"/"view").
func validateExportRenderShape(kind, id, path string) []string {
	if path == "" {
		return nil
	}
	var errs []string
	if !strings.HasSuffix(path, ".lua") {
		errs = append(errs, fmt.Sprintf(
			"%s %q: export_render must be a .lua script path, got %q", kind, id, path))
	}
	if !filepath.IsLocal(path) {
		errs = append(errs, fmt.Sprintf(
			"%s %q: export_render must be a local path under scripts/, got %q", kind, id, path))
	}
	return errs
}

// validateLists validates list definitions.
//
//nolint:gocognit // linear validation dispatcher: one independent config-vs-metamodel check per branch; splitting would scatter the rule set without lowering real complexity.
func validateLists(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for listID, list := range cfg.Lists {
		entDef, ok := meta.GetEntityDef(list.EntityType)
		if !ok {
			errs = append(errs, fmt.Sprintf("list %q: unknown entity type %q", listID, list.EntityType))
			continue
		}

		errs = append(errs, validateExportRenderShape("list", listID, list.ExportRender)...)

		// Validate columns
		for i, c := range list.Columns {
			if c.Relation != "" {
				if _, ok := meta.GetRelationDef(c.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: column[%d] references unknown relation %q",
						listID, i, c.Relation))
				}
			} else if c.Property != "" {
				if _, ok := entDef.Properties[c.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: column[%d] property %q not in metamodel for entity %q",
						listID, i, c.Property, list.EntityType))
				}
			}
		}

		// Validate sort
		for i, s := range list.Sort {
			if !validSortDirections[s.Direction] {
				errs = append(errs, fmt.Sprintf(
					"list %q: sort[%d] has invalid direction %q (valid: asc, desc)",
					listID, i, s.Direction))
			}
			if s.Property != "" && s.Property != "id" && s.Property != "modified" {
				if _, ok := entDef.Properties[s.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: sort[%d] references unknown property %q",
						listID, i, s.Property))
				}
			}
		}

		// Validate filters
		for i, f := range list.Filters {
			if !validFilterOperators[f.Operator] {
				errs = append(errs, fmt.Sprintf(
					"list %q: filter[%d] has invalid operator %q (valid: %s)",
					listID, i, f.Operator, joinMapKeys(validFilterOperators)))
			}
			if f.HasProperty() && entity.IsEntityPropertyKey(f.Property) {
				if _, ok := entDef.Properties[f.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: filter[%d] references unknown property %q",
						listID, i, f.Property))
				}
			}
		}

		// Validate filter_controls
		for i, fc := range list.FilterControls {
			if fc.Property == "" && fc.Relation == "" {
				errs = append(errs, fmt.Sprintf(
					"list %q: filter_controls[%d] must specify either property or relation",
					listID, i))
			}
			if fc.Property != "" {
				if _, ok := entDef.Properties[fc.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: filter_controls[%d] references unknown property %q",
						listID, i, fc.Property))
				}
			}
			if fc.Relation != "" {
				if _, ok := meta.GetRelationDef(fc.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: filter_controls[%d] references unknown relation %q",
						listID, i, fc.Relation))
				}
			}
		}
	}

	return errs
}

// CollectConfigWarnings returns non-fatal configuration issues that should be
// surfaced at load time (logged) but must NOT abort startup — unlike
// ValidateConfig, whose findings are hard errors. Kept as a pure,
// deterministically-ordered []string so the caller can log each and tests can
// assert on the set.
//
// It flags two conditions:
//
//   - A relation FilterControl declared `direction: incoming` whose list
//     entity_type is not a `to:` side of the relation. Such a filter follows
//     incoming edges into a type that is never the target, so it can only ever
//     match zero rows — a config smell, but not fatal (the filter still
//     behaves, it just filters everything out).
//   - Two lists of the same entity type configuring the same relation filter
//     with conflicting directions. The shared list pipeline is keyed by entity
//     type (not list ID), so RelationFilterDirection resolves a single winner
//     (lowest list ID); the loser's declared direction is silently ignored,
//     which the author should know about (RR-9MJRJG).
func CollectConfigWarnings(cfg *Config, meta *metamodel.Metamodel) []string {
	var warnings []string
	for _, listID := range sortedListIDs(cfg) {
		list := cfg.Lists[listID]
		for i, fc := range list.FilterControls {
			if fc.Relation == "" || !fc.Direction.IsIncoming() {
				continue
			}
			relDef, ok := meta.GetRelationDef(fc.Relation)
			if !ok {
				continue // unknown relation already caught as a hard error
			}
			if slices.Contains(relDef.To, list.EntityType) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"list %q: filter_controls[%d] relation %q with direction: incoming targets entity type %q, "+
					"which is not a `to:` side of the relation (valid: %s); this filter will match no rows",
				listID, i, fc.Relation, list.EntityType, strings.Join(relDef.To, ", ")))
		}
	}
	warnings = append(warnings, conflictingRelationDirectionWarnings(cfg)...)
	warnings = append(warnings, relationPropertyNameCollisionWarnings(cfg, meta)...)
	warnings = append(warnings, viewCommandPermissionWarnings(cfg)...)
	return warnings
}

// viewCommandPermissionWarnings flags `permission:` on a `context: view`
// command. The key is not honored for view commands — they are denied outright
// under any configured acl.yaml (TKT-MJ02AO) — so an author who sets it would
// otherwise believe they had granted access that silently does not apply.
// Warn rather than error: the config is not wrong, it is inert, and erroring
// would break projects that pre-emptively annotate their view commands.
func viewCommandPermissionWarnings(cfg *Config) []string {
	var warnings []string
	ids := make([]string, 0, len(cfg.Commands))
	for id := range cfg.Commands {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		cmd := cfg.Commands[id]
		if cmd.Context == "view" && cmd.Permission != "" {
			warnings = append(warnings, fmt.Sprintf(
				"command %q: permission %q is ignored for context: view — view commands are denied "+
					"under any configured acl.yaml and cannot yet be granted per-command",
				id, cmd.Permission))
		}
	}
	return warnings
}

// relationPropertyNameCollisionWarnings flags a relation FilterControl whose
// name also names a property of the list's entity type, with no property
// FilterControl to disambiguate. Properties (per-type) and relations (global)
// are disjoint namespaces, so nothing forbids the collision; the runtime
// resolves it in favor of the PROPERTY (safe, backward-compatible), which means
// the relation filter the author configured silently does nothing. Warn so the
// ambiguity is visible at load (RR-0HWAS0).
func relationPropertyNameCollisionWarnings(cfg *Config, meta *metamodel.Metamodel) []string {
	var warnings []string
	for _, listID := range sortedListIDs(cfg) {
		list := cfg.Lists[listID]
		entDef, ok := meta.GetEntityDef(list.EntityType)
		if !ok {
			continue
		}
		for i, fc := range list.FilterControls {
			if fc.Relation == "" {
				continue
			}
			if _, isProp := entDef.Properties[fc.Relation]; !isProp {
				continue
			}
			if cfg.HasPropertyFilterControl(list.EntityType, fc.Relation) {
				continue // an explicit property control resolves the routing
			}
			warnings = append(warnings, fmt.Sprintf(
				"list %q: filter_controls[%d] relation %q collides with a property of entity type %q; "+
					"the filter will match the PROPERTY, not the relation — rename or add a property "+
					"filter control to disambiguate",
				listID, i, fc.Relation, list.EntityType))
		}
	}
	return warnings
}

// conflictingRelationDirectionWarnings flags (entityType, relation) pairs where
// two or more lists of the same type configure the same relation filter with
// conflicting directions. RelationFilterDirection resolves to the lowest list
// ID, so any other list's declared direction is ignored at runtime.
func conflictingRelationDirectionWarnings(cfg *Config) []string {
	// Group the (listID, direction) each list declares for a given
	// (entityType, relation), in sorted-list-ID order so the winner and the
	// warning text are deterministic.
	type decl struct {
		listID    string
		direction Direction
	}
	byPair := map[string][]decl{}
	var pairOrder []string
	for _, listID := range sortedListIDs(cfg) {
		list := cfg.Lists[listID]
		for _, fc := range list.FilterControls {
			if fc.Relation == "" {
				continue
			}
			dir := DirectionOutgoing
			if fc.Direction.IsIncoming() {
				dir = DirectionIncoming
			}
			key := list.EntityType + "\x00" + fc.Relation
			if _, seen := byPair[key]; !seen {
				pairOrder = append(pairOrder, key)
			}
			byPair[key] = append(byPair[key], decl{listID: listID, direction: dir})
		}
	}

	var warnings []string
	for _, key := range pairOrder {
		decls := byPair[key]
		conflict := false
		for _, d := range decls[1:] {
			if d.direction != decls[0].direction {
				conflict = true
				break
			}
		}
		if !conflict {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		entityType, relation := parts[0], parts[1]
		winner := decls[0]
		var others []string
		for _, d := range decls {
			if d.listID == winner.listID {
				continue
			}
			others = append(others, fmt.Sprintf("%s=%s", d.listID, d.direction))
		}
		warnings = append(warnings, fmt.Sprintf(
			"entity type %q: relation %q filter is configured with conflicting directions across lists; "+
				"list %q (direction: %s) wins (lowest list ID), ignoring %s",
			entityType, relation, winner.listID, winner.direction, strings.Join(others, ", ")))
	}
	return warnings
}

// sortedListIDs returns the config's list IDs in deterministic order so
// warning output is stable across runs.
func sortedListIDs(cfg *Config) []string {
	ids := make([]string, 0, len(cfg.Lists))
	for id := range cfg.Lists {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// validateViews validates view definitions with their traversal rules and sections.
//
//nolint:gocognit,gocyclo,funlen // linear validation dispatcher: one independent config-vs-metamodel check per branch; splitting would scatter the rule set without lowering real complexity.
func validateViews(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	// At most one view may target a given entity type. The detail screen
	// looks up views by entity type, so duplicates would be ambiguous.
	byType := map[string][]string{}
	for viewID, view := range cfg.Views {
		if view.Entry.Type == "" {
			continue
		}
		byType[view.Entry.Type] = append(byType[view.Entry.Type], viewID)
	}
	for entityType, viewIDs := range byType {
		if len(viewIDs) <= 1 {
			continue
		}
		sort.Strings(viewIDs)
		errs = append(errs, fmt.Sprintf(
			"multiple views target entity type %q: %s — at most one view per entity type is allowed",
			entityType, strings.Join(viewIDs, ", ")))
	}

	for viewID, view := range cfg.Views {
		// Before the entry-type check, which continues on failure: a bad
		// export_render path is worth reporting even on a view whose entry
		// type is also wrong.
		errs = append(errs, validateExportRenderShape("view", viewID, view.ExportRender)...)

		// Validate entry type
		entDef, ok := meta.GetEntityDef(view.Entry.Type)
		if !ok {
			errs = append(errs, fmt.Sprintf(
				"view %q: entry type %q not in metamodel",
				viewID, view.Entry.Type))
			continue
		}

		// Build collection map: tracks what collections are available at each point
		// Start with "entry" which refers to the entry entity type
		collections := map[string]string{
			"entry": view.Entry.Type, // collection name -> entity type
		}

		// Validate traverse rules and track collections
		for i, t := range view.Traverse {
			// Validate from
			if t.From != "*" {
				if _, ok := collections[t.From]; !ok {
					validCollections := sortedMapKeys(collections)
					validCollections = append(validCollections, "*")
					errs = append(errs, fmt.Sprintf(
						"view %q: traverse[%d] references unknown collection %q in from (valid: %s)",
						viewID, i, t.From, strings.Join(validCollections, ", ")))
				}
			}

			// Validate follow/follow_incoming relation
			var relType string
			if t.Follow != "" {
				relType = t.Follow
				if _, ok := meta.GetRelationDef(t.Follow); !ok {
					if suggestion := suggestRelation(t.Follow, meta); suggestion != "" {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow (did you mean %q?)",
							viewID, i, t.Follow, suggestion))
					} else {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow",
							viewID, i, t.Follow))
					}
				}
			}
			if t.FollowIncoming != "" {
				relType = t.FollowIncoming
				if _, ok := meta.GetRelationDef(t.FollowIncoming); !ok {
					if suggestion := suggestRelation(t.FollowIncoming, meta); suggestion != "" {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow_incoming (did you mean %q?)",
							viewID, i, t.FollowIncoming, suggestion))
					} else {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow_incoming",
							viewID, i, t.FollowIncoming))
					}
				}
			}
			if t.Follow == "" && t.FollowIncoming == "" {
				errs = append(errs, fmt.Sprintf(
					"view %q: traverse[%d] must specify either follow or follow_incoming",
					viewID, i))
			}
			if t.Follow != "" && t.FollowIncoming != "" {
				errs = append(errs, fmt.Sprintf(
					"view %q: traverse[%d] cannot specify both follow and follow_incoming",
					viewID, i))
			}

			// Validate collect_as is specified
			if t.CollectAs == "" {
				errs = append(errs, fmt.Sprintf(
					"view %q: traverse[%d] must specify collect_as",
					viewID, i))
			} else {
				// Determine target entity type for this collection
				targetType := determineTargetType(t, relType, meta)
				collections[t.CollectAs] = targetType
			}
		}

		// Validate sections
		for i, s := range view.Sections {
			// Validate source
			sourceType := ""
			if s.Source != "" {
				if entityType, ok := collections[s.Source]; ok {
					sourceType = entityType
				} else {
					validSources := sortedMapKeys(collections)
					errs = append(errs, fmt.Sprintf(
						"view %q: section[%d] references unknown collection %q in source (valid: %s)",
						viewID, i, s.Source, strings.Join(validSources, ", ")))
				}
			}

			// Validate display mode
			if !validSectionDisplayModes[s.Display] {
				errs = append(errs, fmt.Sprintf(
					"view %q: section[%d] has invalid display mode %q (valid: %s)",
					viewID, i, s.Display, joinMapKeys(validSectionDisplayModes)))
			}

			// Spans are checked unconditionally — deliberately NOT inside the
			// `sourceType != ""` guard below, which only runs when the source
			// collection resolves. A bad span is wrong regardless of whether
			// the section's source is valid, and hiding it behind an unrelated
			// error would surface it only after the first one was fixed.
			for j, f := range s.Fields {
				errs = append(errs, validateSpan(f.Span,
					fmt.Sprintf("view %q: section[%d] field[%d]", viewID, i, j))...)
			}

			// Validate fields (if source type is known)
			if sourceType != "" { //nolint:nestif // nested guards each check a distinct optional field of the source config.
				if sourceDef, ok := meta.GetEntityDef(sourceType); ok {
					for j, f := range s.Fields {
						if f.Property != "" && f.Property != "title" && f.Property != "id" {
							if _, ok := sourceDef.Properties[f.Property]; !ok {
								errs = append(errs, fmt.Sprintf(
									"view %q: section[%d] field[%d] property %q not in entity %q",
									viewID, i, j, f.Property, sourceType))
							}
						}
					}

					// Validate columns
					for j, c := range s.Columns {
						if c.Property != "" && c.Property != "title" && c.Property != "id" {
							if _, ok := sourceDef.Properties[c.Property]; !ok {
								errs = append(errs, fmt.Sprintf(
									"view %q: section[%d] column[%d] property %q not in entity %q",
									viewID, i, j, c.Property, sourceType))
							}
						}
						if c.Relation != "" {
							if _, ok := meta.GetRelationDef(c.Relation); !ok {
								errs = append(errs, fmt.Sprintf(
									"view %q: section[%d] column[%d] references unknown relation %q",
									viewID, i, j, c.Relation))
							}
						}
					}

					// Validate group_by
					if s.GroupBy != "" {
						if _, ok := sourceDef.Properties[s.GroupBy]; !ok {
							errs = append(errs, fmt.Sprintf(
								"view %q: section[%d] group_by references unknown property %q",
								viewID, i, s.GroupBy))
						}
					}
				}
			} else if s.Source == "entry" {
				// Source is entry, use entry entity def
				for j, f := range s.Fields {
					if f.Property != "" && f.Property != "title" && f.Property != "id" {
						if _, ok := entDef.Properties[f.Property]; !ok {
							errs = append(errs, fmt.Sprintf(
								"view %q: section[%d] field[%d] property %q not in entity %q",
								viewID, i, j, f.Property, view.Entry.Type))
						}
					}
				}
			}
		}
	}

	return errs
}

// determineTargetType determines the entity type that a traverse rule collects.
func determineTargetType(t ViewTraverse, relType string, meta *metamodel.Metamodel) string {
	if relType == "" {
		return ""
	}
	relDef, ok := meta.GetRelationDef(relType)
	if !ok {
		return ""
	}

	// For outgoing (follow), target is relation's To types
	// For incoming (follow_incoming), target is relation's From types
	if t.Follow != "" {
		if len(relDef.To) == 1 {
			return relDef.To[0]
		}

		return "" // multiple possible types, can't determine statically
	}
	if t.FollowIncoming != "" {
		if len(relDef.From) == 1 {
			return relDef.From[0]
		}

		return "" // multiple possible types
	}

	return ""
}

// suggestRelation finds a similar relation name for typo suggestions.
func suggestRelation(name string, meta *metamodel.Metamodel) string {
	nameLower := strings.ToLower(name)
	for relName := range meta.Relations {
		if strings.EqualFold(relName, name) {
			return relName
		}
		// Simple similarity: check if one contains the other
		relNameLower := strings.ToLower(relName)
		if strings.Contains(relNameLower, nameLower) || strings.Contains(nameLower, relNameLower) {
			return relName
		}
	}

	return ""
}

// validateKanbans validates kanban board definitions.
//
//nolint:gocognit,gocyclo,funlen // linear validation dispatcher: one independent config-vs-metamodel check per branch; splitting would scatter the rule set without lowering real complexity.
func validateKanbans(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for kanbanID, kanban := range cfg.Kanbans {
		// Validate entity type
		entDef, ok := meta.GetEntityDef(kanban.EntityType)
		if !ok {
			errs = append(errs, fmt.Sprintf("kanban %q: unknown entity type %q", kanbanID, kanban.EntityType))
			continue
		}

		// Validate column_property exists and is enum type
		if kanban.ColumnProperty == "" { //nolint:nestif // nested guards each check a distinct optional field of the kanban config.
			errs = append(errs, fmt.Sprintf("kanban %q: column_property is required", kanbanID))
		} else {
			propDef, ok := entDef.Properties[kanban.ColumnProperty]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: column_property %q not in entity %q",
					kanbanID, kanban.ColumnProperty, kanban.EntityType))
			} else {
				// Check if it's an enum type
				validValues := GetValidEnumValues(propDef, meta)
				if len(validValues) == 0 {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: column_property %q must be an enum type",
						kanbanID, kanban.ColumnProperty))
				} else {
					// Validate column values if specified
					validSet := make(map[string]bool)
					for _, v := range validValues {
						validSet[v] = true
					}
					for i, col := range kanban.Columns {
						if !validSet[col.Value] {
							errs = append(errs, fmt.Sprintf(
								"kanban %q: columns[%d] value %q is not valid for %q (valid: %s)",
								kanbanID, i, col.Value, kanban.ColumnProperty, strings.Join(validValues, ", ")))
						}
					}
				}
			}
		}

		// Validate swimlane_property if specified
		if kanban.SwimlaneProperty != "" { //nolint:nestif // nested guards each check a distinct optional field of the kanban config.
			propDef, ok := entDef.Properties[kanban.SwimlaneProperty]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: swimlane_property %q not in entity %q",
					kanbanID, kanban.SwimlaneProperty, kanban.EntityType))
			} else {
				// Check if it's an enum type
				validValues := GetValidEnumValues(propDef, meta)
				if len(validValues) == 0 {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: swimlane_property %q must be an enum type",
						kanbanID, kanban.SwimlaneProperty))
				} else {
					// Validate swimlane values if specified
					validSet := make(map[string]bool)
					for _, v := range validValues {
						validSet[v] = true
					}
					for i, lane := range kanban.Swimlanes {
						if !validSet[lane.Value] {
							errs = append(errs, fmt.Sprintf(
								"kanban %q: swimlanes[%d] value %q is not valid for %q (valid: %s)",
								kanbanID, i, lane.Value, kanban.SwimlaneProperty, strings.Join(validValues, ", ")))
						}
					}
				}
			}
		}

		// Validate card title property
		if kanban.Card.Title != "" {
			if _, ok := entDef.Properties[kanban.Card.Title]; !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: card.title property %q not in entity %q",
					kanbanID, kanban.Card.Title, kanban.EntityType))
			}
		}

		// Validate card fields
		for i, f := range kanban.Card.Fields {
			if f.Relation != "" {
				if _, ok := meta.GetRelationDef(f.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: card.fields[%d] references unknown relation %q",
						kanbanID, i, f.Relation))
				}
				continue
			}
			if f.Property != "" && f.Property != "title" && f.Property != "id" {
				if _, ok := entDef.Properties[f.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: card.fields[%d] property %q not in entity %q",
						kanbanID, i, f.Property, kanban.EntityType))
				}
			}
		}

		// Validate filter properties
		for i, f := range kanban.Filters {
			if !validFilterOperators[f.Operator] {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: filters[%d] has invalid operator %q (valid: %s)",
					kanbanID, i, f.Operator, joinMapKeys(validFilterOperators)))
			}
			if f.HasProperty() && entity.IsEntityPropertyKey(f.Property) {
				if _, ok := entDef.Properties[f.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: filters[%d] references unknown property %q",
						kanbanID, i, f.Property))
				}
			}
		}

		// Validate filter_controls
		for i, fc := range kanban.FilterControls {
			if fc.Property == "" && fc.Relation == "" {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: filter_controls[%d] must specify either property or relation",
					kanbanID, i))
			}
			if fc.Property != "" {
				if _, ok := entDef.Properties[fc.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: filter_controls[%d] references unknown property %q",
						kanbanID, i, fc.Property))
				}
			}
			if fc.Relation != "" {
				if _, ok := meta.GetRelationDef(fc.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: filter_controls[%d] references unknown relation %q",
						kanbanID, i, fc.Relation))
				}
			}
		}

		// Validate form references
		if kanban.EditForm != "" {
			if _, ok := cfg.Forms[kanban.EditForm]; !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: references unknown form %q in edit_form",
					kanbanID, kanban.EditForm))
			}
		}
		if kanban.CreateForm != "" {
			if _, ok := cfg.Forms[kanban.CreateForm]; !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: references unknown form %q in create_form",
					kanbanID, kanban.CreateForm))
			}
		}
	}

	return errs
}

// validateDashboard validates dashboard configuration.
func validateDashboard(cfg *Config, _ *metamodel.Metamodel) []string {
	var errs []string

	if cfg.Dashboard == nil {
		return errs
	}

	for i, card := range cfg.Dashboard.Cards {
		// Validate display mode
		if !validDashboardDisplayModes[card.Display] {
			errs = append(errs, fmt.Sprintf(
				"dashboard: card[%d] %q has invalid display mode %q (valid: %s)",
				i, card.Title, card.Display, joinMapKeys(validDashboardDisplayModes)))
		}

		// Validate sort directions
		for j, s := range card.Sort {
			if !validSortDirections[s.Direction] {
				errs = append(errs, fmt.Sprintf(
					"dashboard: card[%d] %q sort[%d] has invalid direction %q (valid: asc, desc)",
					i, card.Title, j, s.Direction))
			}
		}

		// For breakdown display, group_by should be specified
		if card.Display == "breakdown" && card.GroupBy == "" {
			errs = append(errs, fmt.Sprintf(
				"dashboard: card[%d] %q uses breakdown display but has no group_by",
				i, card.Title))
		}

		// For table display, columns should be specified
		if card.Display == "table" && len(card.Columns) == 0 {
			errs = append(errs, fmt.Sprintf(
				"dashboard: card[%d] %q uses table display but has no columns",
				i, card.Title))
		}
	}

	return errs
}

// appIDRegex defines the allowed format for app IDs — same shape as action IDs.
// The ID is used as the URL path segment for GET /api/v1/_apps/{id}, so it must
// be filesystem- and URL-safe.
var appIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// ValidAppID reports whether id is a syntactically valid app ID. Apps are
// discovered by scanning apps/*.html; the id is the filename stem and is used as
// the URL path segment for GET /api/v1/_apps/{id}, so it must be filesystem- and
// URL-safe. Exported so the scanner and the request handler share one rule.
func ValidAppID(id string) bool { return appIDRegex.MatchString(id) }

// validateCommands validates command definitions.
// actionIDRegex defines the allowed format for action IDs.
// Lowercase letters, digits, hyphens, underscores, 1-64 characters.
var actionIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// reservedListKeys are single-character keys already used by useListKeyboard.ts.
// Action key bindings must not conflict with these.
var reservedListKeys = map[string]bool{
	"j": true, "k": true, // navigation
	"o": true, "e": true, "n": true, // open, edit, new
	"h": true, "l": true, // pagination
}

// actionKeyRegex allows a single lowercase letter or digit as a shortcut key.
var actionKeyRegex = regexp.MustCompile(`^[a-z0-9]$`)

// validateActions checks action definitions: ID format, set/script exclusivity,
// script path safety, and key shortcut validity.
//
//nolint:gocognit // linear validation dispatcher: one independent config-vs-metamodel check per branch; splitting would scatter the rule set without lowering real complexity.
func validateActions(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for id, action := range cfg.Actions {
		if !actionIDRegex.MatchString(id) {
			errs = append(errs, fmt.Sprintf(
				"actions: invalid action ID %q (must match ^[a-z0-9_-]{1,64}$)", id))
			continue
		}

		hasScript := action.Script != ""
		hasSet := len(action.Set) > 0

		if hasScript && hasSet {
			errs = append(errs, fmt.Sprintf(
				"actions: %q has both script and set (must have one, not both)", id))
			continue
		}
		if !hasScript && !hasSet {
			errs = append(errs, fmt.Sprintf(
				"actions: %q has neither script nor set (must have one)", id))
			continue
		}

		// Script validation
		if hasScript {
			if !strings.HasSuffix(action.Script, ".lua") {
				errs = append(errs, fmt.Sprintf(
					"actions: %q script %q must have .lua extension", id, action.Script))
			}
			if !filepath.IsLocal(action.Script) {
				errs = append(errs, fmt.Sprintf(
					"actions: %q script %q must be a local path (no '..' or absolute paths)",
					id, action.Script))
			}
		}

		// Key validation (optional — only required when referenced by a list)
		if action.Key != "" {
			if !actionKeyRegex.MatchString(action.Key) {
				errs = append(errs, fmt.Sprintf(
					"actions: %q key %q must be a single lowercase letter or digit", id, action.Key))
			} else if reservedListKeys[action.Key] {
				errs = append(errs, fmt.Sprintf(
					"actions: %q key %q conflicts with reserved list shortcut", id, action.Key))
			}
		}
	}

	// Validate list action references
	for listID, list := range cfg.Lists {
		seenKeys := map[string]string{} // key → action ID
		entDef, hasEntDef := meta.GetEntityDef(list.EntityType)

		for _, actionID := range list.Actions {
			action, ok := cfg.Actions[actionID]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"list %q: references unknown action %q", listID, actionID))
				continue
			}

			// Actions referenced by lists must have label and key
			if action.Label == "" {
				errs = append(errs, fmt.Sprintf(
					"list %q: action %q must have a label when used in a list", listID, actionID))
			}
			if action.Key == "" {
				errs = append(errs, fmt.Sprintf(
					"list %q: action %q must have a key when used in a list", listID, actionID))
			} else if prev, dup := seenKeys[action.Key]; dup {
				errs = append(errs, fmt.Sprintf(
					"list %q: actions %q and %q have duplicate key %q", listID, prev, actionID, action.Key))
			} else {
				seenKeys[action.Key] = actionID
			}

			// Validate set properties against the list's entity type
			if len(action.Set) > 0 && hasEntDef {
				for prop := range action.Set {
					if _, ok := entDef.Properties[prop]; !ok {
						errs = append(errs, fmt.Sprintf(
							"list %q: action %q sets unknown property %q for entity type %q",
							listID, actionID, prop, list.EntityType))
					}
				}
			}
		}
	}

	return errs
}

func validateCommands(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for cmdID, cmd := range cfg.Commands {
		if cmd.Label == "" {
			errs = append(errs, fmt.Sprintf("command %q: label is required", cmdID))
		}
		if cmd.Script == "" {
			errs = append(errs, fmt.Sprintf("command %q: script is required", cmdID))
		}
		if !validCommandContexts[cmd.Context] {
			errs = append(errs, fmt.Sprintf(
				"command %q: invalid context %q (valid: %s)",
				cmdID, cmd.Context, joinMapKeys(validCommandContexts)))
		}
		if cmd.AvailableOn != nil {
			for _, v := range cmd.AvailableOn.Views {
				if _, ok := cfg.Views[v]; !ok {
					errs = append(errs, fmt.Sprintf(
						"command %q: available_on references unknown view %q", cmdID, v))
				}
			}
			for _, l := range cmd.AvailableOn.Lists {
				if _, ok := cfg.Lists[l]; !ok {
					errs = append(errs, fmt.Sprintf(
						"command %q: available_on references unknown list %q", cmdID, l))
				}
			}
			for _, et := range cmd.AvailableOn.EntityTypes {
				if _, ok := meta.GetEntityDef(et); !ok {
					errs = append(errs, fmt.Sprintf(
						"command %q: available_on references unknown entity type %q", cmdID, et))
				}
			}
		}
	}

	return errs
}

// validateDocuments validates document configurations.
//
// validateApp validates app-level settings. PlantUMLServerURL, when set,
// must be an absolute http/https URL with a host: the SPA feeds it straight
// into an <img src>, so a malformed or non-http scheme (e.g. javascript:,
// data:, protocol-relative //host, or a bare host) is rejected at load time
// rather than reaching a user's browser. Empty is valid (PlantUML disabled).
func validateApp(cfg *Config) []string {
	var errs []string
	if raw := cfg.App.PlantUMLServerURL; raw != "" {
		u, err := url.Parse(raw)
		switch {
		case err != nil:
			errs = append(errs, fmt.Sprintf("app.plantuml_server_url: not a valid URL: %v", err))
		case u.Scheme != "http" && u.Scheme != "https":
			errs = append(errs, fmt.Sprintf(
				"app.plantuml_server_url: scheme must be http or https, got %q", u.Scheme))
		case u.Host == "":
			errs = append(errs, "app.plantuml_server_url: must include a host")
		}
	}
	return errs
}

// Invariant: every document must have entity_type set, and exactly one of
// {command, script} must be non-empty. entity_type is enforced at the HTTP
// handler layer to reject cross-type render requests; the mutual exclusion
// prevents ambiguous configs (which renderer runs when both are set?).
func validateDocuments(cfg *Config) []string {
	var errs []string

	for docID, doc := range cfg.Documents {
		// entity_type is NOT required: omitting it declares a standalone
		// document (DocumentConfig.IsStandalone). What a standalone document
		// cannot have is an edit: block — that button navigates to a form for
		// the document's entity, and there is no entity. Checked here rather
		// than left to render time so the author learns at config load.
		//
		// The nil-vs-{} caveat on Edit (see its godoc) does not matter here:
		// both shapes mean "no button", and only a non-nil Edit is an error.
		if doc.IsStandalone() && doc.Edit != nil {
			errs = append(errs, fmt.Sprintf(
				"document %q: edit is not supported without entity_type (a standalone document has no entity to edit)",
				docID))
		}

		hasCmd := len(doc.Command) > 0
		hasScript := doc.Script != ""
		switch {
		case hasCmd && hasScript:
			errs = append(errs, fmt.Sprintf(
				"document %q: command and script are mutually exclusive", docID))
		case !hasCmd && !hasScript:
			errs = append(errs, fmt.Sprintf(
				"document %q: one of command or script must be set", docID))
		}

		// {id} / {id_lower} were removed in TKT-QGHNVA: they spliced a
		// request-derived value into a shell string. Fail at config load with
		// the replacement named, rather than silently passing the literal
		// "{id}" through to the renderer — a silent pass would look like a
		// working config that produces a document about the wrong thing.
		for _, arg := range doc.Command {
			if strings.Contains(arg, "{id}") || strings.Contains(arg, "{id_lower}") {
				errs = append(errs, fmt.Sprintf(
					"document %q: {id}/{id_lower} are no longer supported; "+
						"use {in}, the entry entity's markdown file whose frontmatter carries `id:` "+
						"(commands now run without a shell, so the id never reaches the command line)",
					docID))
				break
			}
		}

		if doc.Edit != nil { //nolint:nestif // nested branches validate distinct doc.Edit sub-fields.
			if doc.Edit.Form == "" {
				errs = append(errs, fmt.Sprintf(
					"document %q: edit.form is required when edit is set", docID))
			} else if _, ok := cfg.Forms[doc.Edit.Form]; !ok {
				if suggestion := suggestForm(doc.Edit.Form, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"document %q: edit.form references unknown form %q (did you mean %q?)",
						docID, doc.Edit.Form, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"document %q: edit.form references unknown form %q",
						docID, doc.Edit.Form))
				}
			}
			if doc.Edit.Label == "" {
				errs = append(errs, fmt.Sprintf(
					"document %q: edit.label is required when edit is set", docID))
			}
		}
	}

	return errs
}

// validateStyles validates style definitions reference valid metamodel types.
func validateStyles(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for typeName := range cfg.Styles {
		// Check if it's a custom type in metamodel
		if _, ok := meta.Types[typeName]; ok {
			continue
		}
		// Check if it's used as a property type in any entity
		found := false
		for _, entDef := range meta.Entities {
			for _, propDef := range entDef.Properties {
				if propDef.Type == typeName {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf(
				"styles: type %q is not defined in metamodel", typeName))
		}
	}

	return errs
}

// validateCrossReferences validates that all cross-references between config sections are valid.
//
//nolint:gocognit // linear validation dispatcher: one independent config-vs-metamodel check per branch; splitting would scatter the rule set without lowering real complexity.
func validateCrossReferences(cfg *Config) []string {
	var errs []string

	// Validate list references to forms and views
	for listID, list := range cfg.Lists {
		if list.CreateForm != "" {
			if _, ok := cfg.Forms[list.CreateForm]; !ok {
				if suggestion := suggestForm(list.CreateForm, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in create_form (did you mean %q?)",
						listID, list.CreateForm, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in create_form",
						listID, list.CreateForm))
				}
			}
		}
		if list.EditForm != "" {
			if _, ok := cfg.Forms[list.EditForm]; !ok {
				if suggestion := suggestForm(list.EditForm, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in edit_form (did you mean %q?)",
						listID, list.EditForm, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in edit_form",
						listID, list.EditForm))
				}
			}
		}
		if list.DetailView != "" {
			if _, ok := cfg.Views[list.DetailView]; !ok {
				if suggestion := suggestView(list.DetailView, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown view %q in detail_view (did you mean %q?)",
						listID, list.DetailView, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown view %q in detail_view",
						listID, list.DetailView))
				}
			}
		}
	}

	return errs
}

// suggestForm finds a similar form name for typo suggestions.
func suggestForm(name string, cfg *Config) string {
	nameLower := strings.ToLower(name)
	for formName := range cfg.Forms {
		if strings.EqualFold(formName, name) {
			return formName
		}
		formNameLower := strings.ToLower(formName)
		if strings.Contains(formNameLower, nameLower) || strings.Contains(nameLower, formNameLower) {
			return formName
		}
	}

	return ""
}

// suggestView finds a similar view name for typo suggestions.
func suggestView(name string, cfg *Config) string {
	nameLower := strings.ToLower(name)
	for viewName := range cfg.Views {
		if strings.EqualFold(viewName, name) {
			return viewName
		}
		viewNameLower := strings.ToLower(viewName)
		if strings.Contains(viewNameLower, nameLower) || strings.Contains(nameLower, viewNameLower) {
			return viewName
		}
	}

	return ""
}

// Helper functions

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	natsort.Strings(keys)
	return keys
}

func joinMapKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != "" { // skip empty string (default)
			keys = append(keys, k)
		}
	}
	natsort.Strings(keys)
	return strings.Join(keys, ", ")
}
