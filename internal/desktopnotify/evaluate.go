package desktopnotify

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Match is one entity that satisfies one notification rule.
type Match struct {
	RuleID     string
	EntityID   string
	EntityType string
	// Title and Body are rendered plain text, capped at 120 and 240 runes.
	Title string
	Body  string
}

// Result is the outcome of one [Config.Evaluate].
type Result struct {
	// Rules lists every evaluated rule id, including rules with no match.
	// [Tracker.Update] needs it to tell "no matches" from "not evaluated".
	Rules []string
	// Matches holds the current matches, grouped by rule in declaration
	// order and ordered by entity id within a rule.
	Matches []Match
	// Badge is the number of entities matching the badge rule. It is zero
	// when desktop.yaml declares no badge; see [Config.HasBadge].
	Badge int
	// EvalErrors holds conditions that failed to evaluate for one entity,
	// for example because a stored value has the wrong type. Such an entity
	// counts as not matching; the error is kept so it can be logged.
	EvalErrors []error
}

// Evaluate reads the entities of every type a rule names and returns the
// current matches and badge count. It reads each entity type once, however
// many rules name it. A store error fails the whole evaluation.
//
// world is the world the entities are read in: one face per entity, the one
// the desktop shows. An unset world is rejected.
//
// The reader must be the store the desktop user sees. The desktop is a
// single-user app with no read ACL, so no visibility wrapper is applied here.
// It takes store.EntityReader, not a narrower interface, because that is what
// store.ListEntityHeaders accepts; the helper reads content-free headers when
// the backend can, which is all a condition and a template need.
func (c *Config) Evaluate(ctx context.Context, entities store.EntityReader, world store.WorldScope) (Result, error) {
	if entities == nil {
		return Result{}, errors.New("desktopnotify: Evaluate requires an entity reader")
	}
	if !world.IsSet() {
		return Result{}, errors.New("desktopnotify: Evaluate requires a world")
	}
	res := Result{Rules: c.RuleIDs()}

	headers := map[string][]store.EntityHeader{}
	read := func(entityType string) ([]store.EntityHeader, error) {
		if hs, ok := headers[entityType]; ok {
			return hs, nil
		}
		var hs []store.EntityHeader
		q := store.EntityQuery{Type: entityType, Faces: store.InWorld(world)}
		for h, err := range store.ListEntityHeaders(ctx, entities, q) {
			if err != nil {
				return nil, fmt.Errorf("list %s: %w", entityType, err)
			}
			hs = append(hs, h)
		}
		headers[entityType] = hs
		return hs, nil
	}

	for _, r := range c.rules {
		hs, err := read(r.entityType)
		if err != nil {
			return Result{}, err
		}
		for _, h := range hs {
			if !c.matches(ctx, r.condition, h, r.id, &res) {
				continue
			}
			f := fields{
				id:         h.ID,
				entityType: h.Type,
				title:      c.meta.DisplayTitle(h.ID, h.Type, h.Properties),
				props:      h.Properties,
			}
			res.Matches = append(res.Matches, Match{
				RuleID:     r.id,
				EntityID:   h.ID,
				EntityType: h.Type,
				Title:      plainText(r.title.render(f), maxTitleRunes, false),
				Body:       plainText(r.body.render(f), maxBodyRunes, true),
			})
		}
	}

	if c.badge != nil {
		hs, err := read(c.badge.entityType)
		if err != nil {
			return Result{}, err
		}
		for _, h := range hs {
			if c.matches(ctx, *c.badge, h, "badge", &res) {
				res.Badge++
			}
		}
	}
	return res, nil
}

// matches evaluates cond for h. An evaluation error is recorded on res and
// counts as no match.
func (c *Config) matches(
	ctx context.Context, cond condition, h store.EntityHeader, label string, res *Result,
) bool {
	ok, err := c.eval.Matches(ctx, cond.prog, h.Type, h.ID, h.Properties)
	if err != nil {
		res.EvalErrors = append(res.EvalErrors, fmt.Errorf("%s: entity %s: %w", label, h.ID, err))
		return false
	}
	return ok
}
