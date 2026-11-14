package desktopnotify

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// MaxMatchesPerRule is the most entities a rule can match and still notify.
// A rule whose condition lowers reads at most this many rows plus one, so a
// poll costs the same however many entities match.
//
// A rule over the cap reports no matches and is listed in
// [Result.Truncated]. Reporting part of its matches would make the
// [Tracker] diff against an arbitrary window: an entity outside the window
// could not be told apart from one that stopped matching.
const MaxMatchesPerRule = 1000

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
	// Truncated lists the rules that matched more than [MaxMatchesPerRule]
	// entities. They contribute no Matches, and [Tracker.Update] stops
	// tracking them until they are back under the cap.
	Truncated []string
	// Badge is the number of entities matching the badge rule. It is zero
	// when desktop.yaml declares no badge; see [Config.HasBadge].
	Badge int
	// EvalErrors holds conditions that failed to evaluate for one entity,
	// for example because a stored value has the wrong type. Such an entity
	// counts as not matching; the error is kept so it can be logged. A
	// condition the store answers never adds one: its string equalities
	// cannot fail.
	EvalErrors []error
}

// Evaluate returns the current matches and badge count. A store error fails
// the whole evaluation.
//
// A condition that lowers exactly to store predicates (see
// Config.compileCondition) is answered by the store: its rule reads at most
// [MaxMatchesPerRule] plus one headers, and a badge is one count. Any other
// condition reads every header of its type that passes the condition's
// pushable equalities, and judges the rest in Go. Conditions that share a
// type and those equalities share that read.
//
// world is the world the entities are read in: one face per entity, the one
// the desktop shows. An unset world is rejected.
//
// The reader must be the store the desktop user sees. The desktop is a
// single-user app with no read ACL, so no visibility wrapper is applied here.
func (c *Config) Evaluate(ctx context.Context, entities store.GraphQueryer, world store.WorldScope) (Result, error) {
	if entities == nil {
		return Result{}, errors.New("desktopnotify: Evaluate requires an entity reader")
	}
	if !world.IsSet() {
		return Result{}, errors.New("desktopnotify: Evaluate requires a world")
	}
	ev := evaluation{cfg: c, entities: entities, world: world, scans: map[string][]store.EntityHeader{}}
	ev.res.Rules = c.RuleIDs()

	for _, r := range c.rules {
		hs, err := ev.ruleMatches(ctx, r)
		if err != nil {
			return Result{}, err
		}
		if len(hs) > MaxMatchesPerRule {
			ev.res.Truncated = append(ev.res.Truncated, r.id)
			continue
		}
		for _, h := range hs {
			f := fields{
				id:         h.ID,
				entityType: h.Type,
				title:      c.meta.DisplayTitle(h.ID, h.Type, h.Properties),
				props:      h.Properties,
			}
			ev.res.Matches = append(ev.res.Matches, Match{
				RuleID:     r.id,
				EntityID:   h.ID,
				EntityType: h.Type,
				Title:      plainText(r.title.render(f), maxTitleRunes, false),
				Body:       plainText(r.body.render(f), maxBodyRunes, true),
			})
		}
	}

	if c.badge != nil {
		n, err := ev.count(ctx, *c.badge)
		if err != nil {
			return Result{}, err
		}
		ev.res.Badge = n
	}
	return ev.res, nil
}

// evaluation is the state of one [Config.Evaluate] call.
type evaluation struct {
	cfg      *Config
	entities store.GraphQueryer
	world    store.WorldScope
	// scans holds the candidates of the conditions that do not lower,
	// keyed by [scanKey], so conditions that share a scan read once.
	scans map[string][]store.EntityHeader
	res   Result
}

// ruleMatches returns the headers matching r, ordered by id. A rule whose
// condition lowers gets at most [MaxMatchesPerRule] plus one of them, so the
// caller can tell a rule over the cap from one at it.
func (ev *evaluation) ruleMatches(ctx context.Context, r rule) ([]store.EntityHeader, error) {
	var hs []store.EntityHeader
	if r.exact {
		// Which rows the limit keeps does not matter: a rule over the cap
		// reports none of them.
		q := store.GraphQuery{
			EntityType: r.entityType,
			Props:      slices.Clone(r.pushed),
			Faces:      store.InWorld(ev.world),
			Limit:      MaxMatchesPerRule + 1,
		}
		for h, err := range store.GraphQueryHeaders(ctx, ev.entities, q) {
			if err != nil {
				return nil, fmt.Errorf("rule %q: query %s: %w", r.id, r.entityType, err)
			}
			hs = append(hs, h)
		}
	} else {
		candidates, err := ev.scan(ctx, r.condition)
		if err != nil {
			return nil, err
		}
		for _, h := range candidates {
			if ev.matches(ctx, r.condition, h, r.id) {
				hs = append(hs, h)
			}
		}
	}
	// The store's order is its own; the result promises id order.
	slices.SortFunc(hs, func(a, b store.EntityHeader) int { return strings.Compare(a.ID, b.ID) })
	return hs, nil
}

// count returns how many entities match cond.
func (ev *evaluation) count(ctx context.Context, cond condition) (int, error) {
	if cond.exact {
		n, err := store.CountMatched(ctx, ev.entities, store.GraphQuery{
			EntityType: cond.entityType,
			Props:      slices.Clone(cond.pushed),
			Faces:      store.InWorld(ev.world),
		})
		if err != nil {
			return 0, fmt.Errorf("badge: count %s: %w", cond.entityType, err)
		}
		return n, nil
	}
	candidates, err := ev.scan(ctx, cond)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, h := range candidates {
		if ev.matches(ctx, cond, h, "badge") {
			n++
		}
	}
	return n, nil
}

// scan reads the candidates of a condition that does not lower exactly:
// every header of its type that passes its pushed equalities. Its cost
// grows with those rows, not with the matches.
func (ev *evaluation) scan(ctx context.Context, cond condition) ([]store.EntityHeader, error) {
	key := scanKey(cond)
	if hs, ok := ev.scans[key]; ok {
		return hs, nil
	}
	var hs []store.EntityHeader
	q := store.GraphQuery{
		EntityType: cond.entityType,
		Props:      slices.Clone(cond.pushed),
		Faces:      store.InWorld(ev.world),
	}
	for h, err := range store.GraphQueryHeaders(ctx, ev.entities, q) {
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", cond.entityType, err)
		}
		hs = append(hs, h)
	}
	ev.scans[key] = hs
	return hs, nil
}

// scanKey identifies a scan by its type and pushed predicates. They are
// sorted by Config.lower, so equal sets give equal keys.
func scanKey(cond condition) string {
	var b strings.Builder
	b.WriteString(cond.entityType)
	for _, p := range cond.pushed {
		fmt.Fprintf(&b, "\x00%s\x00%d\x00%t\x00%s", p.Property, p.Op, p.Scalar, p.Value)
	}
	return b.String()
}

// matches evaluates cond for h. An evaluation error is recorded on the
// result and counts as no match.
func (ev *evaluation) matches(ctx context.Context, cond condition, h store.EntityHeader, label string) bool {
	ok, err := ev.cfg.eval.Matches(ctx, cond.prog, h.Type, h.ID, h.Properties)
	if err != nil {
		ev.res.EvalErrors = append(ev.res.EvalErrors, fmt.Errorf("%s: entity %s: %w", label, h.ID, err))
		return false
	}
	return ok
}
