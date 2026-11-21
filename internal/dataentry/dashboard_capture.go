package dataentry

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// errCardNotCapturable marks a card a capture task cannot store a figure of.
var errCardNotCapturable = errors.New("card cannot be captured")

// CardFigureSource computes dashboard card figures for capture tasks
// (TKT-6OYQO79). It is the scheduler's card evaluator: the scheduler cannot
// import this package, so the wiring site hands it one of these.
type CardFigureSource struct{ app *App }

// CardFigures returns the card figure source over a. It is a package
// function rather than an App method because App is at its plimsoll load
// line.
//
// Nil: rejected, since a source without an App could compute nothing.
func CardFigures(a *App) (CardFigureSource, error) {
	if a == nil {
		return CardFigureSource{}, errors.New("dataentry: card figures need an app")
	}
	return CardFigureSource{app: a}, nil
}

// CardFigure computes the figure of the card titled card on dashboard
// ("default" or empty for the default dashboard) as the principal bound on
// ctx, exactly:
//
//   - count: the rows of the card's list this principal reads;
//   - ratio: matched rows as a percentage of those rows;
//   - sum, avg, min, max: the folded value of a numeric property.
//
// The rows pass the read gate before anything folds, exactly as for
// /api/v1/_dashboard/card. Only `list:` cards on a dashboard without
// entity_type have a figure. A card's `permission:` is not consulted: it is a
// UX filter on the dashboard, not an access control, and the rows are gated
// either way.
//
// A sum, avg, min or max over a property hidden on some row is refused rather
// than stored as a figure that covers fewer rows than it claims. So is a
// ratio over no rows, and a fold with no value.
func (c CardFigureSource) CardFigure(ctx context.Context, dashboard, card string) (*big.Rat, error) {
	a := c.app
	s := a.State()
	dash, ok := s.Cfg.DashboardByName(dashboard)
	if !ok || dash == nil {
		if dashboard == "" || dashboard == dataentryconfig.DefaultDashboardName {
			return nil, fmt.Errorf("%w: data-entry.yaml has no dashboard: block", errCardNotCapturable)
		}
		return nil, fmt.Errorf("%w: no dashboard named %s", errCardNotCapturable, strconv.Quote(dashboard))
	}
	if dash.IsEntityDashboard() {
		return nil, fmt.Errorf("%w: dashboard %s is an entity dashboard; its cards count for one entity",
			errCardNotCapturable, strconv.Quote(dashboard))
	}
	cfg, err := captureCard(dash, card)
	if err != nil {
		return nil, err
	}
	ctx, err = captureEdge(ctx, a)
	if err != nil {
		return nil, err
	}
	switch cfg.Display {
	case "count", "ratio":
		value, err := dashboardCardValue(ctx, a, s, cfg, nil)
		if err != nil {
			return nil, err
		}
		if cfg.Display == "count" {
			return new(big.Rat).SetInt64(int64(value.Total)), nil
		}
		if value.Total == 0 {
			return nil, fmt.Errorf("%w: the ratio has no rows", errCardNotCapturable)
		}
		return big.NewRat(int64(value.Matched)*100, int64(value.Total)), nil
	default:
		return summaryFigure(ctx, a, s, cfg)
	}
}

// captureCard returns the card titled title on dash, if a capture can store
// its figure.
func captureCard(dash *dataentryconfig.DashboardConfig, title string) (dataentryconfig.DashboardCard, error) {
	var found []dataentryconfig.DashboardCard
	for _, card := range dash.Cards {
		if card.Title == title {
			found = append(found, card)
		}
	}
	switch {
	case len(found) == 0:
		return dataentryconfig.DashboardCard{}, fmt.Errorf("%w: no card titled %s",
			errCardNotCapturable, strconv.Quote(title))
	case len(found) > 1:
		return dataentryconfig.DashboardCard{}, fmt.Errorf("%w: %d cards are titled %s; give the card a unique title",
			errCardNotCapturable, len(found), strconv.Quote(title))
	}
	card := found[0]
	if card.List == "" {
		return card, fmt.Errorf("%w: only a card with list: has a computed value", errCardNotCapturable)
	}
	switch card.Display {
	case "count", "ratio", "sum", "avg", "min", "max":
		return card, nil
	}
	return card, fmt.Errorf("%w: display %s has no single figure; use count, ratio, sum, avg, min or max",
		errCardNotCapturable, strconv.Quote(card.Display))
}

// captureEdge prepares ctx as the API middleware prepares a request: the
// read gate of the Request bound on ctx, and the default world. A capture
// runs on a scheduler worker, outside any HTTP request.
//
// Under a declarative policy a missing Request is an error, and a read that
// somehow finds no gate is denied rather than allowed (withACLEnforced).
// A default world the lookup does not know is an error rather than the
// trivial scope, under which a faced type would look empty and the figure
// would come out low.
func captureEdge(ctx context.Context, a *App) (context.Context, error) {
	if d, ok := a.acl.(*acl.Declarative); ok && d != nil {
		bound, err := attachBoundReadGate(withACLEnforced(ctx), d)
		if err != nil {
			return ctx, err
		}
		ctx = bound
	}
	name := effectiveDefaultWorld(a)
	if a.worlds == nil {
		return withWorld(ctx, worldHandle{name: name, scope: defaultWorldHandle().scope}), nil
	}
	scope, ok := a.worlds.Lookup(name)
	if !ok {
		return ctx, fmt.Errorf("default world %s is not known", strconv.Quote(name))
	}
	return withWorld(ctx, worldHandle{name: name, scope: scope}), nil
}

// summaryFigure folds a sum, avg, min or max card exactly. The rows are the
// card's list rows as the dashboard reads them, redacted once, with their
// roll-ups; only then do they fold.
func summaryFigure(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard,
) (*big.Rat, error) {
	listCfg, ok := s.Cfg.Lists[card.List]
	if !ok {
		return nil, fmt.Errorf("%w: unknown list %q", errListLoad, card.List)
	}
	def, ok := s.Meta.GetEntityDef(listCfg.EntityType)
	if !ok {
		return nil, fmt.Errorf("%w: unknown entity type %q", errListLoad, listCfg.EntityType)
	}
	pd := def.Properties[card.Property]
	if pd.Type != metamodel.PropertyTypeInteger && pd.Type != metamodel.PropertyTypeDecimal {
		return nil, fmt.Errorf("%w: property %s is a %s, not a number", errCardNotCapturable,
			strconv.Quote(card.Property), pd.Type)
	}
	ctx, raw, err := listRowsFor(ctx, a, s, card.List, listCfg, "", nil)
	if err != nil {
		return nil, err
	}
	rows := cardRows(ctx, a, listCfg.EntityType, raw, card.Property)
	return foldFigure(card.Display, card.Property, rows)
}

// foldFigure folds prop over rows exactly. A row whose prop is redacted, or
// holds a value that is not a number, refuses the figure: either way the
// figure would cover fewer rows than the card. A row without a value is left
// out, as the card leaves it out.
func foldFigure(display, prop string, rows []*entityPkg.Entity) (*big.Rat, error) {
	for _, e := range rows {
		if e.IsRedacted(prop) {
			return nil, fmt.Errorf("%w: property %s is hidden on some rows, so the %s would cover fewer rows "+
				"than the card", errCardNotCapturable, strconv.Quote(prop), display)
		}
		if v, set := e.Properties[prop]; set && v != nil && v != "" {
			if _, _, err := metamodel.ParseDecimal(v); err != nil {
				// Neither the row nor the value is named: the error is logged.
				return nil, fmt.Errorf("%w: property %s holds a value that is not a number on some rows",
					errCardNotCapturable, strconv.Quote(prop))
			}
		}
	}
	switch display {
	case "sum", "avg":
		sum, _, n := sumDecimals(prop, rows)
		if n == 0 {
			return nil, fmt.Errorf("%w: no row has a value for %s", errCardNotCapturable, strconv.Quote(prop))
		}
		if display == "avg" {
			sum.Quo(sum, new(big.Rat).SetInt64(int64(n)))
		}
		return sum, nil
	default:
		var pick *big.Rat
		for _, e := range rows {
			r, _, err := metamodel.ParseDecimal(e.Properties[prop])
			if err != nil {
				continue
			}
			if pick == nil || (display == "min" && r.Cmp(pick) < 0) || (display == "max" && r.Cmp(pick) > 0) {
				pick = r
			}
		}
		if pick == nil {
			return nil, fmt.Errorf("%w: no row has a value for %s", errCardNotCapturable, strconv.Quote(prop))
		}
		return pick, nil
	}
}
