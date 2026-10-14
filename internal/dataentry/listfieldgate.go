package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// List filter and sort field gate (BUG-WJSYQT).
//
// A list filter or sort computes over the raw property values of every row
// the principal may read, before the served rows are redacted. On a property
// that `visible:` hides from the principal, the result would disclose the
// hidden value: `contains`, `lt` and `gt` recover it by binary search, and a
// sort orders rows by it. So a list request may filter or sort only on a
// property the principal sees on every row, and any other is refused with a
// 400 that names the property. Property names are configuration, not secrets,
// so naming one discloses nothing (root CLAUDE.md).
//
// A property shown on some rows only (a `when:` grant, or a local role that
// hides it on the rows it applies to) is refused too. Evaluating such a
// filter on redacted rows instead would mean judging the field verdict of
// every row of the type before a page is cut, keeping a redacted copy of each
// row in step with the raw row the page returns, and keeping the store
// pushdown away from those requests. The static refusal costs no row reads,
// is the same on the Go path and in the pushdown, and fails closed. A role
// that needs to filter on the field gets an unconditional grant.
//
// Two narrowings stay outside the gate on purpose: a list's `condition:` and
// its query scope (`query_scope:` or the `?query_scope=` a caller picks from
// the declared ones). Both are operator-authored config that may compare a
// property `visible:` hides from some reader. That is the accepted channel
// appbuild.QueryScopeVisibilityConflicts describes: the read gate runs first,
// so the narrowing can only drop rows the reader may already read, and the
// operator is warned at load for every declared scope. The caller picks a
// scope by name but cannot change what it compares, so there is no guess to
// confirm. The gate covers what the caller writes: `filter[]` and `sort`.
//
// The refusal must not depend on rows: the gate runs before the page scope,
// the free-text pass and the deny-all short cut, and its answer comes from
// the policy and the caller's own roles only. Otherwise the difference
// between a 400 and a 200 would itself say something about the data.

// errPropertyNotVisible marks a list filter or sort on a property the
// principal does not see on every row. Surfaced as HTTP 400
// property_not_visible by writeListPipelineError.
var errPropertyNotVisible = errors.New("property not visible")

// propertyNotVisibleError is the refusal gateListQuery returns. It says how
// the request used the property, so the 400 can tell a client whether it may
// drop that part of the request and read again.
type propertyNotVisibleError struct {
	use, property, entityType string
}

func (e *propertyNotVisibleError) Error() string {
	return fmt.Sprintf("%s: cannot %s on %q of %q, because it is hidden from you on some or all rows",
		errPropertyNotVisible, e.use, e.property, e.entityType)
}

// Is makes errors.Is(err, errPropertyNotVisible) hold for the refusal.
func (e *propertyNotVisibleError) Is(target error) bool { return target == errPropertyNotVisible }

// gatedListQuery is a list request's query parameters after gateListQuery
// checked every property they filter or sort on. The list pipeline's filter,
// sort, relation pass and store pushdown take this type rather than the raw
// map, so no list path can reach them with an unchecked query.
// TestGatedListQuery_BuiltOnlyByTheGate keeps the literal in this file.
type gatedListQuery struct {
	params map[string][]string
	// isRelationKey is the classification the gate used to tell relation
	// filter keys from property keys. The filter, relation pass and pushdown
	// route keys with this same function, so a key the gate skipped as a
	// relation is never evaluated as a property.
	//
	// Nil: accepted — every key is a property key.
	isRelationKey func(string) bool
}

// fieldVisibility is the one capability gateListQuery needs from the field
// verdict resolver.
type fieldVisibility interface {
	FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool
}

// gateListQuery checks every property query filters or sorts on against
// fields and returns the checked query, or an errPropertyNotVisible error
// naming the first property the principal does not see on every row.
//
// isRelationKey identifies the filter keys the relation pass handles; those
// name a relation, not a property of typeName, and are not checked here.
func gateListQuery(
	ctx context.Context, fields fieldVisibility, isRelationKey func(string) bool,
	typeName string, query map[string][]string,
) (gatedListQuery, error) {
	for _, ref := range listQueryProperties(query, isRelationKey) {
		if !fields.FieldVisibleOnEveryRow(ctx, typeName, ref.property) {
			return gatedListQuery{}, &propertyNotVisibleError{
				use: ref.use, property: ref.property, entityType: typeName,
			}
		}
	}
	return gatedListQuery{params: query, isRelationKey: isRelationKey}, nil
}

// gateAppListQuery is gateListQuery with the app's field resolver and the
// relation-filter classification the list pipeline uses for typeName.
//
// The config's FilterControls are authoritative for that classification
// (RR-0HWAS0 / RR-B0JPPL): a relation filter applies only when a list or
// kanban control of this type configures it. Name-based GetRelationDef is
// only a fallback and only ever routes AWAY from properties, never toward
// relations without a control.
func gateAppListQuery(
	ctx context.Context, a *App, state *Schema, typeName string, query map[string][]string,
) (gatedListQuery, error) {
	isRelationKey := relationFilterClassifier(state.Meta, state.Cfg, typeName)
	return gateListQuery(ctx, a.fieldResolver, isRelationKey, typeName, query)
}

// listQueryProperty is one property a list query reads, and whether it
// filters or sorts on it.
type listQueryProperty struct {
	property string
	use      string
}

// listQueryProperties returns every property query filters or sorts on, with
// the parsers the filter, the relation pass and the pushdown use
// (parseFilterKey) and the sort uses (parseSortParam).
//
// A malformed filter key is skipped: applyV1Filters refuses it with a 400 and
// the pushdown declines it, so it never reaches a row. A virtual sort key
// (filter.IsVirtualSortProperty) orders by the entity id or modification
// time, which no `visible:` grant governs, and the pushdown never orders by
// it.
func listQueryProperties(query map[string][]string, isRelationKey func(string) bool) []listQueryProperty {
	var out []listQueryProperty
	for key := range query {
		if !strings.HasPrefix(key, "filter[") {
			continue
		}
		fk, err := parseFilterKey(key)
		if err != nil || (isRelationKey != nil && isRelationKey(fk.property)) {
			continue
		}
		out = append(out, listQueryProperty{property: fk.property, use: "filter"})
	}
	for _, spec := range parseSortParam(query) {
		if filter.IsVirtualSortProperty(spec.Property) {
			continue
		}
		out = append(out, listQueryProperty{property: spec.Property, use: "sort"})
	}
	return out
}

// sidebarSkipLevel is the log level for a sidebar entry skipped because of
// err. A list refused for a hidden property is expected for some readers on
// every sidebar render, and the operator already got a load-time warning
// (warnListVisibilityConflicts), so it logs at DEBUG; any other cause stays a
// WARN.
func sidebarSkipLevel(err error) slog.Level {
	if errors.Is(err, errPropertyNotVisible) {
		return slog.LevelDebug
	}
	return slog.LevelWarn
}

// warnListVisibilityConflicts logs one warning per configured list that sorts,
// groups or filters on a property some role can hide with `visible:`. A
// reader whose roles do not show the property on every row is refused the
// list (see gateListQuery); a reader who also holds a role without a block
// that reads the type is not, because roles only add fields (BUG-Y5UF3K).
// The SPA reads a list again unsorted when only the sort is refused, but a
// refused filter or a sidebar count fails. The warning names the list, the
// property and the roles, so the operator finds the conflict at load rather
// than from a user report.
//
// Nil: accepted for cfg, meta and a non-declarative ACL — nothing to compare.
func warnListVisibilityConflicts(cfg *Config, meta *metamodel.Metamodel, aclImpl acl.ACL) {
	d, ok := aclImpl.(*acl.Declarative)
	if !ok || cfg == nil || meta == nil {
		return
	}
	for _, msg := range listVisibilityConflicts(cfg, meta, d.Policy()) {
		slog.Warn("data-entry config warning", "detail", msg)
	}
}

// configuredFieldUse is one place in the data-entry config that makes the SPA
// or the sidebar filter or sort a list read on a property.
type configuredFieldUse struct {
	surface    string // e.g. `list "tickets"`
	entityType string
	property   string
}

// listVisibilityConflicts reports, sorted, every configured filter or sort on
// a declared property that a reader may be refused because the property is
// hidden from them (BUG-WJSYQT). The surfaces are lists (sort, group_by,
// filters, filter controls), kanbans (filter controls), calendar sources
// (date window and `where:`) and sidebar `entities:` entries (sort). A
// property is reported when a role that reads the type has a `visible:`
// block that does not grant it without `when:`, or when a client baseline
// leaves it out of its `visible:` or names it in `redact:`. It is pessimistic
// like appbuild.QueryScopeVisibilityConflicts: there is no principal at load,
// so it asks whether ANY role or client could be refused.
func listVisibilityConflicts(cfg *Config, meta *metamodel.Metamodel, policy *acl.Policy) []string {
	if policy == nil {
		return nil
	}
	var out []string
	for _, use := range configuredFieldUses(cfg, meta) {
		def, ok := meta.GetEntityDef(use.entityType)
		if !ok {
			continue
		}
		if _, declared := def.Properties[use.property]; !declared {
			continue
		}
		if roles := rolesHidingField(policy, use.entityType, use.property); len(roles) > 0 {
			out = append(out, fmt.Sprintf(
				"%s sorts, groups or filters on %q, which role(s) %s can hide with `visible:`; "+
					"a reader who sees it only through those roles gets 400 property_not_visible there",
				use.surface, use.property, strings.Join(roles, ", ")))
		}
		if baselines := baselinesHidingField(policy, use.entityType, use.property); len(baselines) > 0 {
			out = append(out, fmt.Sprintf(
				"%s sorts, groups or filters on %q, which client baseline(s) %s hide; "+
					"a client under those baselines gets 400 property_not_visible there",
				use.surface, use.property, strings.Join(baselines, ", ")))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// configuredFieldUses returns every configured filter or sort on a property,
// on each surface that sends one to the list endpoint. Virtual sort keys and
// relation filter controls are left out: neither names a property.
func configuredFieldUses(cfg *Config, meta *metamodel.Metamodel) []configuredFieldUse {
	var uses []configuredFieldUse
	add := func(surface, entityType, property string) {
		if property != "" && !filter.IsVirtualSortProperty(property) {
			uses = append(uses, configuredFieldUse{surface: surface, entityType: entityType, property: property})
		}
	}
	for listID, list := range cfg.Lists {
		surface := fmt.Sprintf("list %q", listID)
		for _, property := range configuredListProperties(list) {
			add(surface, list.EntityType, property)
		}
	}
	for kanbanID, kanban := range cfg.Kanbans {
		for _, control := range kanban.FilterControls {
			if control.Relation == "" {
				add(fmt.Sprintf("kanban %q", kanbanID), kanban.EntityType, control.Property)
			}
		}
	}
	for calendarID, calendar := range cfg.Calendars {
		surface := fmt.Sprintf("calendar %q", calendarID)
		for _, source := range calendar.Sources {
			add(surface, source.EntityType, source.Date)
			for _, clause := range source.Where {
				if parsed, err := filter.Parse(clause); err == nil {
					add(surface, source.EntityType, parsed.Property)
				}
			}
		}
	}
	for _, nav := range dataentryconfig.AllNavEntitiesEntries(cfg) {
		for _, spec := range dataentryconfig.EffectiveNavSort(nav, meta) {
			add(fmt.Sprintf("sidebar entities %q", nav.Entities), nav.Entities, spec.Property)
		}
	}
	return uses
}

// configuredListProperties returns the distinct properties a list's config
// sorts, groups or filters on, including its property filter controls.
// Virtual sort keys are left out.
func configuredListProperties(list dataentryconfig.List) []string {
	var props []string
	add := func(p string) {
		if p != "" && !filter.IsVirtualSortProperty(p) && !slices.Contains(props, p) {
			props = append(props, p)
		}
	}
	for _, s := range list.Sort {
		add(s.Property)
	}
	if list.GroupBy != nil {
		add(list.GroupBy.Property)
	}
	for _, f := range list.Filters {
		add(f.Property)
	}
	for _, control := range list.FilterControls {
		if control.Relation == "" {
			add(control.Property)
		}
	}
	return props
}

// rolesHidingField returns, sorted, the roles that read entityType and whose
// `visible:` block for it does not grant property without `when:`. Such a
// role hides the property on at least the rows it applies to. A role that
// does not read the type shows no rows of it, so it hides nothing.
func rolesHidingField(policy *acl.Policy, entityType, property string) []string {
	var roles []string
	for name, role := range policy.Roles {
		grants, declared := role.Visible[entityType]
		if !declared {
			continue
		}
		if faces, all := acl.ReadFaces(role, entityType); !all && len(faces) == 0 {
			continue
		}
		if !slices.ContainsFunc(grants, func(g acl.FieldGrant) bool { return g.Field == property && g.When == "" }) {
			roles = append(roles, name)
		}
	}
	slices.Sort(roles)
	return roles
}

// baselinesHidingField returns, sorted, the client baselines whose ceiling
// hides property on entityType: their `visible:` for the type leaves it out,
// or their `redact:` for the type names it.
func baselinesHidingField(policy *acl.Policy, entityType, property string) []string {
	var names []string
	for name, baseline := range policy.ClientBaselines {
		allowed, restricted := baseline.Visible[entityType]
		redacted := slices.Contains(baseline.Redact[entityType], property)
		if (restricted && !slices.Contains(allowed, property)) || redacted {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
