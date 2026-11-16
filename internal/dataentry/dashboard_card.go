package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const (
	// defaultDashboardTableLimit is a table card's row count when it sets no
	// limit; maxDashboardTableLimit caps an explicit one.
	defaultDashboardTableLimit = 10
	maxDashboardTableLimit     = 100
)

// dashboardCardValue computes one `list:` card. Config load has checked the
// list, the properties and the scope, so a miss here is a config that skipped
// validation and answers an error.
//
// anchor is the entity an entity dashboard counts for, already read through
// the gate (readPageAnchor), and nil on any other dashboard. With an anchor,
// the card keeps only the list rows the anchor reaches over the card's scope
// relation. The order is the one an aggregate needs: the rows pass the read
// gate first, then the narrowing, and only then does anything count or fold
// them. The value is computed per request and never cached, since it differs
// per principal.
//
// Nil anchor: accepted, means an unscoped card.
func dashboardCardValue(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard, anchor *entityPkg.Entity,
) (v1.DashboardCardValue, error) {
	out := v1.DashboardCardValue{Title: card.Title, Display: card.Display, List: card.List}
	listCfg, ok := s.Cfg.Lists[card.List]
	if !ok {
		return out, fmt.Errorf("%w: unknown list %q", errListLoad, card.List)
	}
	def, ok := s.Meta.GetEntityDef(listCfg.EntityType)
	if !ok {
		return out, fmt.Errorf("%w: unknown entity type %q", errListLoad, listCfg.EntityType)
	}
	reach, err := cardReach(ctx, a, s, card, anchor)
	if err != nil {
		return out, err
	}

	if card.Display == "table" {
		return dashboardTable(ctx, a, s, card, listCfg, reach, out)
	}

	// The reach narrows the gated read itself, so the rows are the ones the
	// reader may read AND the anchor reaches, before anything folds.
	ctx, raw, err := listRowsFor(ctx, a, s, card.List, listCfg, "", reach)
	if err != nil {
		return out, err
	}
	out.Total = len(raw)

	typeName := listCfg.EntityType
	switch card.Display {
	case "sum", "avg", "min", "max":
		return dashboardSummary(ctx, a, s, card, def, typeName, raw, out), nil
	case "breakdown":
		rows := cardRows(ctx, a, typeName, raw, card.GroupBy)
		out.Groups, out.Partial = groupCounts(rows, newAxisProperty(s.Meta, def, card.GroupBy), nil)
	case "matrix":
		rows := cardRows(ctx, a, typeName, raw, card.Rows, card.MatrixColumns, card.ToneBy)
		in := matrixInput{
			rows:    newMatrixAxis(s.Meta, def, card.Rows, card.RowsLabel, card.RowsOrder),
			columns: newMatrixAxis(s.Meta, def, card.MatrixColumns, card.ColumnsLabel, card.ColumnsOrder),
		}
		if card.ToneBy != "" {
			tone := newAxisProperty(s.Meta, def, card.ToneBy)
			in.toneBy = &tone
			in.colorFor = matrixColorFor(card, s.Cfg, s.StyleMap[def.Properties[card.ToneBy].Type])
		}
		var m v1.DashboardMatrix
		m, out.Partial = foldMatrix(rows, in)
		out.Matrix = &m
	case "ratio":
		matched, err := dashboardMatches(ctx, a, s, listCfg.EntityType, card.Match, raw)
		if err != nil {
			return out, err
		}
		for _, e := range raw {
			if matched[e.ID] {
				out.Matched++
			}
		}
		if card.GroupBy != "" {
			// Only a grouping reads property values, so only it redacts.
			rows := cardRows(ctx, a, typeName, raw, card.GroupBy)
			out.Groups, out.Partial = groupCounts(rows, newAxisProperty(s.Meta, def, card.GroupBy), matched)
		}
	}
	return out, nil
}

// cardReach returns the ids the anchor reaches over the card's scope
// relation, or nil for a card without an anchor. A card that has an anchor
// but no scope is a config that skipped validation; it answers an error
// rather than counting the whole list under the anchor.
func cardReach(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard, anchor *entityPkg.Entity,
) (map[string]bool, error) {
	if anchor == nil {
		return nil, nil //nolint:nilnil // a nil reach means the card is not narrowed
	}
	if card.Scope == nil || card.Scope.Root || card.Scope.Relation == "" {
		return nil, fmt.Errorf("%w: card %q of an entity dashboard has no relation scope", errListLoad, card.Title)
	}
	_, ids, err := anchorReach(ctx, a, s.Meta, anchor, *card.Scope)
	return ids, err
}

// redactRows primes the redactor for the batch and redacts each row once.
func redactRows(ctx context.Context, a *App, raw []*entityPkg.Entity) []*entityPkg.Entity {
	red := appRedactor(a)
	ctx = visibility.PrimeTraversals(ctx, red, raw)
	rows := make([]*entityPkg.Entity, len(raw))
	for i, e := range raw {
		rows[i] = visibility.Redact(ctx, red, e)
	}
	return rows
}

// dashboardMatches answers which rows a ratio card's `match:` scope admits,
// in one batch over the list's rows.
func dashboardMatches(
	ctx context.Context, a *App, s *Schema, typeName, scopeName string, raw []*entityPkg.Entity,
) (map[string]bool, error) {
	if a.queryScopes == nil && scopeName != metamodel.AllQueryScopeName {
		// viewQueryScope answers "unscoped" without a resolver, which for a
		// ratio would claim every row matches.
		return nil, fmt.Errorf("%w: match %q: no query scope resolver is wired", errListLoad, scopeName)
	}
	scope, err := viewQueryScope(a.queryScopes, s.Cfg, s.Meta, typeName, scopeName)
	if err != nil {
		return nil, err
	}
	if ctx, err = scope.bind(ctx); err != nil {
		return nil, err
	}
	headers := make([]store.EntityHeader, len(raw))
	for i, e := range raw {
		headers[i] = store.EntityHeader{ID: e.ID, Type: e.Type, Face: e.Face, Properties: e.Properties}
	}
	// The same faces as the read that produced the rows, so a related()
	// scope cannot match on a face this reader may not read.
	faces := readGateFromContext(ctx).ReadQuery(ctx, typeName).Faces
	kept, err := applyScope(ctx, a.Services(), headers, scopeRequest{
		Type: typeName, Faces: faces, Scope: scope.Scope, ScopeProps: scope.Props, ScopeFilter: scope.Filter,
	})
	if err != nil {
		return nil, err
	}
	matched := make(map[string]bool, len(kept))
	for _, h := range kept {
		matched[h.ID] = true
	}
	return matched, nil
}

// dashboardTable reads a table card's first rows through the list endpoint's
// own pipeline, as navItemsFor does, sorted by the card's sort when it has
// one and by the list's otherwise.
//
// With a reach (an entity dashboard card), the read is narrowed to the reach
// and sorted, and only then counted and cut to the limit, so the total
// describes the narrowed set.
func dashboardTable(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard,
	listCfg dataentryconfig.List, reach map[string]bool, out v1.DashboardCardValue,
) (v1.DashboardCardValue, error) {
	limit := card.Limit
	if limit <= 0 {
		limit = defaultDashboardTableLimit
	}
	// Config load rejects a larger limit; the clamp covers a config that
	// skipped validation.
	limit = min(limit, maxDashboardTableLimit)
	sort := tableSort(card, listCfg)
	read := func(sorted bool) ([]*entityPkg.Entity, int, error) {
		if reach != nil {
			by := ""
			if sorted {
				by = sort
			}
			_, rows, err := listRowsFor(ctx, a, s, card.List, listCfg, by, reach)
			if err != nil {
				return nil, 0, err
			}
			return rows[:min(limit, len(rows))], len(rows), nil
		}
		query := navItemsListQuery(card.List, listCfg)
		delete(query, "sort")
		if sorted && sort != "" {
			query["sort"] = []string{sort}
		}
		rows, total, _, err := a.listPage(ctx, listCfg.EntityType, query, 1, limit)
		return rows, total, err
	}
	raw, total, err := read(true)
	if refusal, ok := errors.AsType[*propertyNotVisibleError](err); ok && refusal.use == "sort" {
		// The sort property is hidden from this reader on some row. The rows
		// are still the list's, so drop the order and read again, as
		// navItemsFor does. A refused filter stands: dropping it would widen
		// the card.
		raw, total, err = read(false)
	}
	if err != nil {
		return out, err
	}
	out.Total = total
	wanted := map[string]bool{}
	var columns []string
	for _, c := range card.Columns {
		if c.Property != "" {
			wanted[c.Property] = true
			columns = append(columns, c.Property)
		}
	}
	for _, e := range cardRows(ctx, a, listCfg.EntityType, raw, columns...) {
		row := v1.DashboardRow{
			ID:         e.ID,
			Type:       e.Type,
			Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
			Properties: map[string]any{},
		}
		for p := range wanted {
			if e.IsRedacted(p) {
				row.Redacted = append(row.Redacted, p)
				out.Partial = true
				continue
			}
			if v, ok := e.Properties[p]; ok {
				row.Properties[p] = v
			}
		}
		slices.Sort(row.Redacted)
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

// tableSort is the sort a table card reads its rows in, as the list
// endpoint's `sort` parameter: the card's own sort when it has one, else the
// list's (navItemsListQuery). Empty when neither sorts.
func tableSort(card dataentryconfig.DashboardCard, listCfg dataentryconfig.List) string {
	if len(card.Sort) == 0 {
		if v := navItemsListQuery(card.List, listCfg)["sort"]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	sort := make([]string, 0, len(card.Sort))
	for _, spec := range card.Sort {
		if spec.IsDescending() {
			sort = append(sort, "-"+spec.Property)
		} else {
			sort = append(sort, spec.Property)
		}
	}
	return strings.Join(sort, ",")
}
