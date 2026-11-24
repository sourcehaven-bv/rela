package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The child level and the counts of a generated group (TKT-MQSK037) follow
// one pattern. A relation query over the ids of entries already shown gives
// candidate row ids. The candidates only narrow the read: the rows come from
// configuredListRows, so they are row-gated, scoped, filtered and
// conditioned as the list is, before anything about them is shown or
// counted. A candidate the principal may not read is dropped there, so it
// never reaches a child entry or a count (gate before fold). The reads are a
// fixed number per group, whatever the number of entries.

// navRelatedRows returns, for each of ownerIDs, the rows of listID related
// to it through via, in the list's order. dir is from the row's side. A row
// related to several owners is listed under each.
func navRelatedRows(
	ctx context.Context, a *App, s *Schema, ownerIDs []string,
	listID, via string, dir dataentryconfig.Direction, query map[string][]string,
) (context.Context, map[string][]*entityPkg.Entity, error) {
	listCfg, ok := s.Cfg.Lists[listID]
	if !ok {
		return ctx, nil, fmt.Errorf("unknown list %q", listID)
	}
	if len(ownerIDs) == 0 {
		return ctx, nil, nil
	}
	resolved := dataentryconfig.NavItemsInitial{Relation: via, Direction: dir}.
		ResolvedDirection(listCfg.EntityType, s.Meta)
	// The query runs from the owners' side, so its direction is the
	// opposite of the row's.
	qdir := store.DirectionIncoming
	if resolved.IsIncoming() {
		qdir = store.DirectionOutgoing
	}
	owners := make(map[string]bool, len(ownerIDs))
	for _, id := range ownerIDs {
		owners[id] = true
	}
	ownersOf := make(map[string][]string)
	reach := make(map[string]bool)
	q := store.RelationQuery{EntityIDs: ownerIDs, Type: via, Direction: qdir}
	for r, err := range a.Services().Store.ListRelations(ctx, q) {
		if err != nil {
			return ctx, nil, err
		}
		rowID, ownerID := r.From, r.To
		if qdir == store.DirectionOutgoing {
			rowID, ownerID = r.To, r.From
		}
		if !owners[ownerID] {
			continue
		}
		ownersOf[rowID] = append(ownersOf[rowID], ownerID)
		reach[rowID] = true
	}
	// No candidates means no rows. A nil reach would mean no narrowing and
	// read the whole list, so the read is skipped instead.
	if len(reach) == 0 {
		return ctx, nil, nil
	}
	ctx, rows, err := configuredListRows(ctx, a, s, listID, listCfg, query, reach)
	if err != nil {
		return ctx, nil, err
	}
	out := make(map[string][]*entityPkg.Entity)
	for _, e := range rows {
		for _, owner := range ownersOf[e.ID] {
			out[owner] = append(out[owner], e)
		}
	}
	return ctx, out, nil
}

// navItemStatuses returns the count of each of ids that has a nonzero one.
func navItemStatuses(
	ctx context.Context, a *App, s *Schema, st *dataentryconfig.NavItemsStatus, ids []string,
) (map[string]*v1.NavStatus, error) {
	out := make(map[string]*v1.NavStatus)
	if st == nil {
		return out, nil
	}
	listCfg, ok := s.Cfg.Lists[st.List]
	if !ok {
		return nil, fmt.Errorf("unknown list %q", st.List)
	}
	_, related, err := navRelatedRows(ctx, a, s, ids, st.List, st.Via, st.Direction, navStatusListQuery(listCfg))
	if err != nil {
		return nil, err
	}
	for id, rows := range related {
		if n := len(rows); n > 0 {
			out[id] = &v1.NavStatus{
				Tone:  st.Tone,
				Label: strings.ReplaceAll(st.Label, navStatusCountVar, strconv.Itoa(n)),
				Count: n,
			}
		}
	}
	return out, nil
}

// navItemChildren fills the children of entries, at most the limit per
// entry. Truncation is judged on the gated rows, so a hidden child cannot
// mark an entry as truncated. The children's counts are read for the shown
// children only.
func navItemChildren(
	ctx context.Context, a *App, s *Schema, c *dataentryconfig.NavItemsChildren, entries []v1.NavItem,
) error {
	if c == nil || len(entries) == 0 {
		return nil
	}
	listCfg, ok := s.Cfg.Lists[c.List]
	if !ok {
		return fmt.Errorf("unknown list %q", c.List)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	ctx, related, err := navRelatedRows(ctx, a, s, ids, c.List, c.Via, c.Direction,
		navItemsListQuery(c.List, listCfg))
	if err != nil {
		return err
	}
	limit := c.EffectiveLimit()
	truncated := make(map[string]bool)
	var shown []*entityPkg.Entity
	for _, id := range ids {
		if rows := related[id]; len(rows) > limit {
			related[id] = rows[:limit]
			truncated[id] = true
		}
		shown = append(shown, related[id]...)
	}
	if len(shown) == 0 {
		return nil
	}
	// Redacted once and primed for the whole group, as the parents are.
	ctx = primeVerdicts(ctx, a.fieldResolver, shown)
	red := appRedactor(a)
	childIDs := make([]string, 0, len(shown))
	for _, e := range shown {
		childIDs = append(childIDs, e.ID)
	}
	statuses, err := navItemStatuses(ctx, a, s, c.Status, childIDs)
	if err != nil {
		// The children stand without their counts, as the entries do.
		slog.Log(ctx, sidebarSkipLevel(err), "nav items: child counts skipped", "list", c.List, "error", err)
	}
	for i := range entries {
		id := entries[i].ID
		for _, e := range related[id] {
			re := visibility.Redact(ctx, red, e)
			entries[i].Children = append(entries[i].Children, v1.NavItem{
				ID:     re.ID,
				Type:   re.Type,
				Label:  s.Meta.DisplayTitle(re.ID, re.Type, re.Properties),
				Status: statuses[re.ID],
			})
		}
		entries[i].ChildrenTruncated = truncated[id]
	}
	return nil
}
