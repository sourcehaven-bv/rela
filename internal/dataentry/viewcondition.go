package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ViewConditionMatcher evaluates one view's compiled `condition:` against a
// candidate row.
//
// Consumer-side (see docs/architecture/consumer-side-interfaces.md): dataentry
// declares the one method it needs and the composition root supplies the
// implementation, because the predicate engine that compiles and evaluates the
// expression sits above this package — arch-lint forbids dataentry importing
// `conditionlint` or `predicate`, the same reason next actions reach their
// compiled conditions through [ConditionPrefilterer].
//
// The method takes the request ctx because a condition may name
// `current_user`; the identity is derived from the principal on it by the
// implementation, so one derivation serves every row of the page.
//
// An evaluation error is RETURNED, never swallowed as "no match". A row that
// cannot be judged is not the same as a row judged false: silently dropping it
// would be the BUG-WHEREWIDE failure inverted, narrowing a view with no
// diagnostic. The caller decides what an error means for the request.
//
// MatchPage judges a whole page at once, so a condition using `related(...)`
// costs one store query per traversal rather than one per row. gate
// authorizes each traversal and match answers it; the verdicts come back in
// row order.
type ViewConditionMatcher interface {
	MatchPage(
		ctx context.Context, rows []*entityPkg.Entity, gate relresolve.Gate, match relresolve.Match,
	) ([]bool, error)
}

// ViewConditionLookup returns the matcher for a configured view, or ok=false
// when that view declares no condition.
//
// Keyed by the same (kind, id) pair the config uses, because a list and a
// kanban may legitimately share an id. `kind` is "lists" or "kanbans",
// matching conditionlint.ViewConditionKind — spelled as plain strings here so
// the seam carries no import from the compiling package.
type ViewConditionLookup func(kind, id string) (ViewConditionMatcher, bool)

// ViewConditionFunc compiles every configured view `condition:` against the
// current config and metamodel, returning the lookup plus one message per
// problem. The lookup holds every condition that compiled, even when others
// did not; a caller fails a surface closed when its declared condition is
// absent.
//
// The consumer-side seam for the predicate compiler, mirroring
// [NextActionMatcherFunc]: arch-lint keeps the condition engine above this
// package, so the composition root supplies an implementation.
//
// Takes cfg + meta at call time rather than returning a prebuilt lookup
// because both reload at runtime — a lookup captured at boot would keep
// evaluating a condition the operator has since edited.
type ViewConditionFunc func(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (ViewConditionLookup, []string)

// View kinds and the request parameter that names a list.
//
// The parameter carries an ID, never an expression: the condition itself is
// operator config the server already holds, so a caller can only select among
// views it was already free to name. Nothing about the request can introduce
// a predicate the config did not declare.
const (
	viewKindList   = "lists"
	viewKindKanban = "kanbans"
	// viewKindAction is a detail action's `when:` (TKT-VVS16W), keyed by
	// dataentryconfig.ActionConditionID.
	viewKindAction = "actions"
	// viewKindLLMCandidates is an llm action's relate `candidates:`, keyed
	// by dataentryconfig.LLMCandidateConditionID.
	viewKindLLMCandidates = "llm_candidates"

	// listIDParam names which configured list a generic collection read is
	// serving, so the server can apply that list's condition. Absent means
	// "no view named" — see [viewConditionFor].
	listIDParam = "list_id"
	// kanbanIDParam is listIDParam for a board: it names the kanban whose
	// condition the read applies. A request names at most one of the two.
	kanbanIDParam = "kanban_id"
)

// errBadViewID reports a request that names both a list and a kanban. The
// two would each pick a condition, and taking either one silently would show
// the caller a view it did not ask for.
var errBadViewID = errors.New("invalid view parameter: give list_id or kanban_id, not both")

// errViewConditionNotCompiled reports a view whose config declares a
// `condition:` that did not compile. The view fails closed: serving its rows
// unfiltered would show rows the operator's view excludes. The compile
// problem, which names the expression and its error, is logged once per config
// load (logViewConditionProblems). Each refused request is still logged by the
// caller's error path, without that detail.
var errViewConditionNotCompiled = errors.New("view condition does not compile")

// requestViewCondition resolves the condition of the list or kanban a
// collection read names, or nil when it names neither.
func requestViewCondition(
	fn ViewConditionFunc, s *Schema, query map[string][]string,
) (ViewConditionMatcher, error) {
	listID, kanbanID := queryGet(query, listIDParam), queryGet(query, kanbanIDParam)
	switch {
	case listID != "" && kanbanID != "":
		return nil, errBadViewID
	case kanbanID != "":
		return viewCondition(fn, s, viewKindKanban, kanbanID)
	default:
		return viewCondition(fn, s, viewKindList, listID)
	}
}

// viewConditionFor resolves the matcher for a list, or nil when there is none:
// no id supplied, no lookup wired, or the view declares no condition.
//
// A nil result means "no constraint", which is the deliberate answer for a
// request that names no view. `/api/v1/{plural}` is the generic entity
// endpoint and stays generic: a condition is view PRESENTATION, not
// authorization, so omitting it returns the ACL-scoped superset of the view
// rather than anything the principal may not see. Making the bare endpoint
// refuse instead would break every non-SPA caller (MCP, CLI, integrations)
// that legitimately reads a type directly.
func viewConditionFor(lookup ViewConditionLookup, kind, id string) ViewConditionMatcher {
	if lookup == nil || id == "" {
		return nil
	}
	m, ok := lookup(kind, id)
	if !ok {
		return nil
	}
	return m
}

// AdaptViewConditions converts a compiler that returns a STRUCTURALLY
// identical matcher under its own name into a [ViewConditionFunc].
//
// The composition root cannot name this package's types — dataentry's own
// tests import appbuild, so doing so would close an import cycle — and while
// Go assigns the two INTERFACES freely, it will not assign the function types
// wrapping them. This adapter is that one conversion, kept here (where both
// halves are already visible) rather than inlined at every wiring site.
//
// The type parameter is what makes the drift visible: a supplier whose
// matcher stops satisfying [ViewConditionMatcher] fails to compile at the
// call, not at some later evaluation.
func AdaptViewConditions[M ViewConditionMatcher](
	fn func(*dataentryconfig.Config, *metamodel.Metamodel) (func(kind, id string) (M, bool), []string),
) ViewConditionFunc {
	if fn == nil {
		return nil
	}
	return func(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) (ViewConditionLookup, []string) {
		lookup, problems := fn(cfg, meta)
		if lookup == nil {
			return nil, problems
		}
		return func(kind, id string) (ViewConditionMatcher, bool) {
			m, ok := lookup(kind, id)
			if !ok {
				return nil, false
			}
			return m, true
		}, problems
	}
}

// SetViewConditions injects the predicate compiler backing a list's or
// kanban's `condition:`.
//
// Separate from NewApp for the same reason as [App.SetNextActions]:
// the compiler lives above this package, so the composition root supplies it.
// Rejects nil for the same reason too — a silently absent compiler would
// leave every condition unevaluated, showing rows the operator's view
// excludes. A condition that reads roll-ups is judged with them
// ([withConditionRollups]).
func (a *App) SetViewConditions(fn ViewConditionFunc) error {
	if fn == nil {
		return errors.New("dataentry.SetViewConditions: func must be non-nil")
	}
	a.viewConditions = withConditionRollups(fn, newRollupSource(a))
	logViewConditionProblems(fn, a.State())
	return nil
}

// viewCondition resolves the matcher for one view against the CURRENT config
// and metamodel. It compiles per call, because both reload at runtime; the
// Evaluator memoizes programs, so this is cheap next to the store read.
//
// Nil with no error means the view declares no condition. A view whose
// declared condition did not compile returns [errViewConditionNotCompiled],
// so a broken condition fails its own view closed and leaves the others
// filtering.
func viewCondition(fn ViewConditionFunc, s *Schema, kind, id string) (ViewConditionMatcher, error) {
	if id == "" || s == nil {
		return nil, nil //nolint:nilnil // nil is the documented "no condition"
	}
	var lookup ViewConditionLookup
	if fn != nil {
		lookup, _ = fn(s.Cfg, s.Meta)
	}
	m := viewConditionFor(lookup, kind, id)
	if m == nil && declaresViewCondition(s.Cfg, kind, id) {
		return nil, fmt.Errorf("%s[%q]: %w", kind, id, errViewConditionNotCompiled)
	}
	return m, nil
}

// declaresViewCondition reports whether cfg gives the (kind, id) surface a
// condition. It reads the config the compiler compiled and covers the kinds
// [viewCondition] serves.
func declaresViewCondition(cfg *dataentryconfig.Config, kind, id string) bool {
	if cfg == nil {
		return false
	}
	switch kind {
	case viewKindList:
		return cfg.Lists[id].Condition != ""
	case viewKindKanban:
		return cfg.Kanbans[id].Condition != ""
	case viewKindNavStatus:
		for _, ns := range dataentryconfig.NavStatusEntries(cfg) {
			for i, rule := range ns.Entry.Status {
				if rule.Condition != "" && dataentryconfig.NavStatusConditionID(ns.Key, i) == id {
					return true
				}
			}
		}
	case viewKindPageDone:
		for _, pd := range dataentryconfig.PageDoneEntries(cfg) {
			if dataentryconfig.PageDoneConditionID(pd.Page, pd.Tab.ID) == id {
				return true
			}
		}
	}
	return false
}

// logViewConditionProblems logs every condition in s's config that does not
// compile. Called when the compiler is installed and on each config reload,
// never per request.
func logViewConditionProblems(fn ViewConditionFunc, s *Schema) {
	if fn == nil || s == nil {
		return
	}
	_, problems := fn(s.Cfg, s.Meta)
	for _, p := range problems {
		slog.Error("dataentry: condition does not compile; its view fails closed", "problem", p)
	}
}

// applyViewCondition filters rows by a view's condition, preserving order.
//
// Returns rows unchanged when there is no matcher, so a view without a
// condition costs nothing. An evaluation error aborts the page rather than
// yielding a partial one — see [ViewConditionMatcher].
// The condition reads the redacted entity, so a hidden value cannot decide
// whether a row appears. The kept rows are the originals. A caller that has a
// field policy must pass redact; nil means no redaction.
func applyViewCondition(
	ctx context.Context, rows []*entityPkg.Entity, m ViewConditionMatcher,
	redact func(context.Context, *entityPkg.Entity) *entityPkg.Entity, st store.GraphQueryer,
) ([]*entityPkg.Entity, error) {
	if m == nil || len(rows) == 0 {
		return rows, nil
	}
	verdicts, err := matchPage(ctx, redactAll(ctx, rows, redact), m, st)
	if err != nil {
		return nil, err
	}
	kept := make([]*entityPkg.Entity, 0, len(rows))
	for i, e := range rows {
		if verdicts[i] {
			kept = append(kept, e)
		}
	}
	return kept, nil
}

// redactAll returns the rows as the condition must judge them: redacted
// copies when redact is set, the rows themselves otherwise. See
// [applyViewCondition] for why a condition judges redacted values only.
func redactAll(
	ctx context.Context, rows []*entityPkg.Entity, redact func(context.Context, *entityPkg.Entity) *entityPkg.Entity,
) []*entityPkg.Entity {
	if redact == nil {
		return rows
	}
	out := make([]*entityPkg.Entity, len(rows))
	for i, e := range rows {
		out[i] = redact(ctx, e)
	}
	return out
}

// matchPage judges already-redacted rows in one batch and returns one verdict
// per row.
func matchPage(
	ctx context.Context, candidates []*entityPkg.Entity, m ViewConditionMatcher, st store.GraphQueryer,
) ([]bool, error) {
	verdicts, err := m.MatchPage(ctx, candidates, traversalGateFromContext(ctx).GateTraversal, pageMatch(st))
	if err != nil {
		return nil, err
	}
	if len(verdicts) != len(candidates) {
		// coverage-ignore: invariant: MatchPage returns one verdict per row
		return nil, fmt.Errorf("view condition: %d verdicts for %d rows", len(verdicts), len(candidates))
	}
	return verdicts, nil
}

// pageMatch answers a traversal for rows the page already read. It keeps the
// candidate selection acl.TraversalQuery chose: the rows are already the
// page's faces, and stamping the request world over them would drop a row
// read at a face that world does not serve. The request world reaches the
// endpoints through the gate, so a condition still cannot see further than
// the list it filters.
func pageMatch(st store.GraphQueryer) relresolve.Match {
	return func(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
		if st == nil {
			return nil, errors.New("view condition: no store to answer related()")
		}
		return store.MatchingIDs(ctx, st, q, ids)
	}
}
