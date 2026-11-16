package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The request parameters that narrow a collection read to one tab of an
// entity page (`pages.<p>.entity_type`), for the anchor entity the page shows.
//
// Like [listIDParam], the parameters carry ids, never an expression. The
// relation and direction come from the tab's `scope:` in config, so a caller
// can only choose among narrowings the operator declared. And a narrowing
// only ever removes rows: the scope is an intersection with the ACL-scoped
// set, never a second source of rows.
const (
	scopePageParam   = "scope_page"
	scopeTabParam    = "scope_tab"
	scopeAnchorParam = "anchor"
)

// errPageAnchorNotFound reports an anchor the principal cannot read, or that
// does not exist, or that is not of the page's entity type. The three are one
// error on purpose: they answer with the same 404 an entity GET gives, so the
// parameter is not an existence oracle.
var errPageAnchorNotFound = errors.New("page anchor not found")

// hasPageScope reports whether the request names a page scope at all. The
// list pushdown declines on this before resolution, since a scoped request
// narrows the population the store would otherwise page and count.
func hasPageScope(query map[string][]string) bool {
	return len(query[scopePageParam]) > 0 || len(query[scopeTabParam]) > 0 || len(query[scopeAnchorParam]) > 0
}

// resolvePageScope returns the ids of the rows a page tab keeps for its
// anchor: the entities the anchor reaches over the tab's scope relation.
// active is false when the request names no page scope.
//
// ordering is non-nil when the rows are to be shown in relation order: the
// tab reaches its rows over edges of a relation orderable on the tab's
// side, and the request names no sort of its own. The edge values come from
// the relation query the scope runs anyway.
//
// Errors: a malformed or unknown page, tab or type is errBadFilter (400); an
// anchor the principal cannot see is errPageAnchorNotFound (the uniform 404);
// a read-gate failure is wrapped in errACLListQuery; a store failure in
// errListLoad.
//
// Cost: one gated anchor read and ONE relation query, whatever the number of
// rows; an incoming scope adds one batched edge gate (anchorReach). An
// incoming ordering adds one batch of source reads for its movable check,
// also independent of the number of rows.
func resolvePageScope(
	ctx context.Context, a *App, query map[string][]string, typeName string,
) (ids map[string]bool, ordering *relationOrdering, active bool, err error) {
	if !hasPageScope(query) {
		return nil, nil, false, nil
	}
	pageID, tabID, anchor, err := pageScopeParams(query)
	if err != nil {
		return nil, nil, false, err
	}
	state := a.State()
	page, ok := state.Cfg.Pages[pageID]
	if !ok || !page.IsEntityPage() {
		return nil, nil, false, fmt.Errorf("%w: %s %q is not an entity page", errBadFilter, scopePageParam, pageID)
	}
	tab, ok := page.Tab(tabID)
	if !ok || tab.Scope == nil || tab.Scope.Root {
		return nil, nil, false, fmt.Errorf("%w: page %q has no tab %q scoped by a relation",
			errBadFilter, pageID, tabID)
	}
	if rowType := pageTabRowType(state.Cfg, tab); rowType != typeName {
		return nil, nil, false, fmt.Errorf("%w: tab %q of page %q shows %q, not %q",
			errBadFilter, tabID, pageID, rowType, typeName)
	}

	e, err := readPageAnchor(ctx, a, page.EntityType, anchor)
	if err != nil {
		return nil, nil, false, err
	}
	dir := tab.Scope.ResolvedDirection(page.EntityType, state.Meta)
	rels, ids, err := anchorReach(ctx, a, state.Meta, e, *tab.Scope)
	if err != nil {
		return nil, nil, false, err
	}
	if len(parseSortParam(query)) == 0 && orderPropertyOf(state.Meta, tab.Scope.Relation, dir.IsIncoming()) != "" {
		// anchorReach gated incoming edges before the places fold into the
		// order; see newRelationOrdering.
		ordering, err = newRelationOrdering(ctx, a.affordances, state.Meta, e, tab.Scope.Relation,
			dir.IsIncoming(), rels)
		if err != nil {
			return nil, nil, false, fmt.Errorf("%w: %w", errACLListQuery, err)
		}
	}
	return ids, ordering, true, nil
}

// readPageAnchor reads the anchor of an entity page tab or an entity
// dashboard. It goes through the same gated read an entity GET does, so a
// hidden anchor, a missing one and one of another type are all
// errPageAnchorNotFound, the uniform 404.
func readPageAnchor(ctx context.Context, a *App, anchorType, anchor string) (*entityPkg.Entity, error) {
	e, visible, err := a.visibleReader.address(ctx, anchorType, anchor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errACLListQuery, err)
	}
	if !visible || e.Type != anchorType {
		return nil, errPageAnchorNotFound
	}
	return e, nil
}

// anchorReach returns the edges of the scope's relation at the anchor and
// the ids of the entities at their other end: the rows a relation-scoped
// list tab or dashboard card keeps. An outgoing scope keeps only the edges
// the anchor's face owns, as on the entity page. An incoming scope keeps
// only the edges whose source the reader may read at the edge's tail face
// (Relation.FromFace), the rule visibility.Resolver.EndpointsReadable
// applies: a row is never reached through a face the reader may not read.
//
// The ids are a narrowing, not a source of rows: a caller intersects them
// with rows that already passed the read gate. Cost: ONE relation query; an
// incoming scope adds one batched endpoint gate, independent of the number
// of edges.
func anchorReach(
	ctx context.Context, a *App, meta *metamodel.Metamodel, e *entityPkg.Entity, sc dataentryconfig.PageTabScope,
) ([]*entityPkg.Relation, map[string]bool, error) {
	dir := sc.ResolvedDirection(e.Type, meta)
	q := store.RelationQuery{EntityIDs: []string{e.ID}, Type: sc.Relation, Direction: relationDirection(dir)}
	var rels []*entityPkg.Relation
	for r, lerr := range a.store.ListRelations(ctx, q) {
		if lerr != nil {
			return nil, nil, fmt.Errorf("%w: page scope: %w", errListLoad, lerr)
		}
		rels = append(rels, r)
	}
	if dir.IsIncoming() {
		var err error
		if rels, err = a.visibleReader.readableRelations(ctx, rels); err != nil {
			return nil, nil, fmt.Errorf("%w: page scope: %w", errACLListQuery, err)
		}
	} else {
		rels = edgesOwnedBy(meta, rels, e.Face)
	}
	ids := make(map[string]bool, len(rels))
	for _, r := range rels {
		if dir.IsIncoming() {
			ids[r.From] = true
		} else {
			ids[r.To] = true
		}
	}
	return rels, ids, nil
}

// pageScopeParams reads the three parameters. All three are required once
// any is given, and none may repeat: a half-named scope would otherwise read
// as no scope and widen the list to the whole type.
func pageScopeParams(query map[string][]string) (page, tab, anchor string, err error) {
	values := make([]string, 0, 3)
	for _, key := range []string{scopePageParam, scopeTabParam, scopeAnchorParam} {
		v := query[key]
		if len(v) != 1 || v[0] == "" {
			return "", "", "", fmt.Errorf("%w: %s, %s and %s must each be given exactly once",
				errBadFilter, scopePageParam, scopeTabParam, scopeAnchorParam)
		}
		values = append(values, v[0])
	}
	return values[0], values[1], values[2], nil
}

// pageTabLinks returns the relations a row created on a tab of an entity
// page is linked to the anchor over, one per type the tab can show. A list or
// kanban tab gives its scope relation for its row type. A `scope: root` gantt
// tab gives each hierarchy relation that starts at the anchor type, for each
// child type the gantt draws. Other tabs give none.
//
// The SPA links a row created from the space's Create menu with these, so the
// row appears on the page it was created from. They are a convenience, not a
// gate: the link is an ordinary relation write under the normal ACL.
func pageTabLinks(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel, anchorType string, tab dataentryconfig.PageTab,
) []v1.SidebarPageLink {
	switch sc := tab.Scope; {
	case sc == nil:
		return nil
	case sc.Root:
		return ganttRootLinks(cfg, meta, anchorType, tab.Gantt)
	default:
		rowType := pageTabRowType(cfg, tab)
		if rowType == "" {
			return nil
		}
		return []v1.SidebarPageLink{{
			Type: rowType, Relation: sc.Relation, Direction: string(sc.ResolvedDirection(anchorType, meta)),
		}}
	}
}

// ganttRootLinks returns the links of a gantt tab rooted at the anchor: each
// hierarchy relation that starts at the anchor type, for each child type the
// gantt draws. Hierarchy edges run parent to child, so the link is outgoing.
func ganttRootLinks(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel, anchorType, gantt string,
) []v1.SidebarPageLink {
	g, ok := cfg.Gantts[gantt]
	if !ok || meta == nil {
		return nil
	}
	var out []v1.SidebarPageLink
	for _, rel := range g.Hierarchy {
		def, ok := meta.GetRelationDef(rel)
		if !ok || !slices.Contains(def.From, anchorType) {
			continue
		}
		for _, child := range def.To {
			if _, drawn := g.Sources[child]; drawn {
				out = append(out, v1.SidebarPageLink{
					Type: child, Relation: rel, Direction: string(dataentryconfig.DirectionOutgoing),
				})
			}
		}
	}
	return out
}

// pageTabRowType is the entity type a list or kanban tab shows, or "".
func pageTabRowType(cfg *dataentryconfig.Config, tab dataentryconfig.PageTab) string {
	switch {
	case tab.List != "":
		return cfg.Lists[tab.List].EntityType
	case tab.Kanban != "":
		return cfg.Kanbans[tab.Kanban].EntityType
	default:
		return ""
	}
}
