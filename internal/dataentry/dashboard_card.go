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
func dashboardCardValue(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard,
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

	if card.Display == "table" {
		return dashboardTable(ctx, a, s, card, listCfg, out)
	}

	ctx, raw, err := navStatusRows(ctx, a, s, card.List, listCfg)
	if err != nil {
		return out, err
	}
	out.Total = len(raw)

	switch card.Display {
	case "breakdown":
		rows := redactRows(ctx, a, raw)
		out.Groups, out.Partial = groupCounts(rows, newAxisProperty(s.Meta, def, card.GroupBy), nil)
	case "matrix":
		rows := redactRows(ctx, a, raw)
		in := matrixInput{
			rows:    newAxisProperty(s.Meta, def, card.Rows),
			columns: newAxisProperty(s.Meta, def, card.MatrixColumns),
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
			rows := redactRows(ctx, a, raw)
			out.Groups, out.Partial = groupCounts(rows, newAxisProperty(s.Meta, def, card.GroupBy), matched)
		}
	}
	return out, nil
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
func dashboardTable(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard,
	listCfg dataentryconfig.List, out v1.DashboardCardValue,
) (v1.DashboardCardValue, error) {
	limit := card.Limit
	if limit <= 0 {
		limit = defaultDashboardTableLimit
	}
	// Config load rejects a larger limit; the clamp covers a config that
	// skipped validation.
	limit = min(limit, maxDashboardTableLimit)
	query := navItemsListQuery(card.List, listCfg)
	if len(card.Sort) > 0 {
		var sort []string
		for _, spec := range card.Sort {
			if spec.IsDescending() {
				sort = append(sort, "-"+spec.Property)
			} else {
				sort = append(sort, spec.Property)
			}
		}
		query["sort"] = []string{strings.Join(sort, ",")}
	}
	raw, total, _, err := a.listPage(ctx, listCfg.EntityType, query, 1, limit)
	if refusal, ok := errors.AsType[*propertyNotVisibleError](err); ok && refusal.use == "sort" {
		// The sort property is hidden from this reader on some row. The rows
		// are still the list's, so drop the order and read again, as
		// navItemsFor does. A refused filter stands: dropping it would widen
		// the card.
		delete(query, "sort")
		raw, total, _, err = a.listPage(ctx, listCfg.EntityType, query, 1, limit)
	}
	if err != nil {
		return out, err
	}
	out.Total = total
	wanted := map[string]bool{}
	for _, c := range card.Columns {
		if c.Property != "" {
			wanted[c.Property] = true
		}
	}
	for _, e := range redactRows(ctx, a, raw) {
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
