package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// candidates adapts the app's ACL-gated read paths into the
// [nextaction.CandidateFunc] the engine takes, so the engine never sees an
// entity the principal cannot read and depends on no store or ACL type.
// Query sources run through App.executeQuery, count sources through
// countCandidates, flow_tasks through FlowService.Tasks, and context sources
// through contextCandidates. A source's store-evaluable condition conjuncts
// are also pushed into the query; this is sound because the store only drops
// rows the engine's authoritative condition pass would drop. meta and lookup
// are the request's snapshot, captured once by the caller.
func (h *nextActionHandler) candidates(
	meta *metamodel.Metamodel, lookup nextaction.MatcherFunc, page nextActionPage,
) nextaction.CandidateFunc {
	return func(
		ctx context.Context, id string, src dataentryconfig.NextActionSource, want nextaction.Want,
	) ([]nextaction.Candidate, error) {
		if src.IsFlowTasks() {
			return h.flowTasks(ctx, meta, src, want)
		}
		ctx, ok, err := nextActionSourceWorld(ctx, h.worlds(), h.defaultWorld(), src)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		switch {
		case src.Count != "":
			return h.countCandidates(ctx, src.Count, src.CountUngated)
		case src.Query != "":
			return h.queryCandidates(ctx, src.Query, nextActionPrefilters(ctx, meta, lookup, id, src), want)
		case src.Context != "":
			return h.contextCandidates(ctx, src, page.entityID)
		default:
			return nil, nil
		}
	}
}

// ConditionPrefilterer is the OPTIONAL capability of a next-action
// [nextaction.Matcher] whose compiled `condition:` can be partly lowered to
// store predicates.
//
// Consumer-side (see docs/architecture/consumer-side-interfaces.md): dataentry
// declares the one method it needs and the composition root's matcher
// (appbuild.NextActionMatchers) supplies it. The predicate engine that knows
// how to do the lowering sits above this package, which is why the method
// takes the request ctx — the identity `current_user` resolves to is derived
// from the principal on it by the implementation, and the SAME derivation
// serves the matcher's Match, so the pre-filter and the authoritative pass
// can never compare against two different identities.
//
// Returned predicates are ANDed into the candidate query. The contract is the
// one every pushdown in this package carries: they may only remove rows the
// matcher's Match would also reject. A matcher without this capability simply
// gets an unpushed query, with the same result. The matcher knows which entity
// types its condition was compiled against, so it needs no query here.
//
// One asymmetry is load-bearing for that contract: the pre-filter compares RAW
// store values, while Match sees the candidate AFTER field redaction
// (redactedForSuggestion). That is sound only because redaction REMOVES a
// hidden property — it binds Nil, and every current-user form is false on Nil —
// so the Go pass is strictly narrower than the pre-filter, never wider. A
// change that made a hidden property bind to something truthy, or that ran the
// condition before redaction, would break this; pinned by
// TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse.
type ConditionPrefilterer interface {
	Prefilters(ctx context.Context, meta *metamodel.Metamodel) []store.PropPredicate
}

// nextActionPrefilters lowers a source's condition, or returns nil when there
// is nothing to push: no condition, no matcher wired, or a matcher without the
// capability.
//
// A free function rather than an App method: App is at its plimsoll cap.
func nextActionPrefilters(
	ctx context.Context, meta *metamodel.Metamodel, lookup nextaction.MatcherFunc,
	id string, src dataentryconfig.NextActionSource,
) []store.PropPredicate {
	if src.Condition == "" || lookup == nil || meta == nil {
		return nil
	}
	m, ok := lookup(id)
	if !ok || m == nil {
		return nil
	}
	p, ok := m.(ConditionPrefilterer)
	if !ok {
		return nil
	}
	return p.Prefilters(ctx, meta)
}

// nextActionSourceWorld rebinds ctx to the world a source's query runs in,
// reporting ok=false when this principal may not read that world.
// The operator-declared source world replaces the request's `?world=`, which
// governs display only, so a caller cannot aim an operator's rule at another
// world. An absent source world means the default world, returned unchanged.
// A missing read grant skips the source; a gate failure is not a denial and
// propagates as an error.
func nextActionSourceWorld(
	ctx context.Context, lookup WorldLookup, defaultName string, src dataentryconfig.NextActionSource,
) (context.Context, bool, error) {
	name := src.SourceWorld
	if name == "" {
		name = defaultName
	}
	if lookup == nil {
		if name == defaultName {
			return withWorld(ctx, worldHandle{name: name, scope: defaultWorldHandle().scope}), true, nil
		}
		return ctx, false, nil
	}
	scope, known := lookup.Lookup(name)
	if !known {
		return ctx, false, nil
	}
	if name == defaultName {
		return withWorld(ctx, worldHandle{name: name, scope: scope}), true, nil
	}
	permitted, err := readGateFromContext(ctx).PermitsWorld(ctx, name)
	if err != nil {
		return ctx, false, fmt.Errorf("next-action source world %q: %w", name, err)
	}
	if !permitted {
		return ctx, false, nil
	}
	return withWorld(ctx, worldHandle{name: name, scope: scope}), true, nil
}

// nextActionOptions resolves a pick_one option list through the SAME
// ACL-gated path as candidates, so an option can never name an entity the
// caller may not read.
//
// Labels come from the entity's display title, which the read gate has
// already approved — this is why the resolution belongs here and not in the
// SPA: a client-side fetch would be a second read surface to gate.
// Each option is redacted before its display title is taken, so a hidden title
// never becomes a label.
func (h *nextActionHandler) options(meta *metamodel.Metamodel) nextaction.OptionFunc {
	return func(
		ctx context.Context, src dataentryconfig.NextActionSource, query string, limit int,
	) ([]nextaction.PickOption, error) {
		ctx, ok, err := nextActionSourceWorld(ctx, h.worlds(), h.defaultWorld(), src)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		entities, err := h.queries().executeQueryWith(ctx, query, queryOptions{firstN: limit, limit: limit})
		if err != nil {
			return nil, fmt.Errorf("next-action pick_one query %q: %w", query, err)
		}
		if len(entities) > limit {
			entities = entities[:limit]
		}
		out := make([]nextaction.PickOption, 0, len(entities))
		ctx = primeVerdicts(ctx, h.fields(), entities)
		aff := h.affordances()
		for _, e := range entities {
			red := redactedForSuggestion(ctx, aff, e)
			out = append(out, nextaction.PickOption{
				EntityID: e.ID,
				Label:    safeDisplayTitle(meta, red),
			})
		}
		return out, nil
	}
}

// queryCandidates runs a source's query through the ACL-gated pipeline.
// Candidates are field-redacted here, not only row-gated, because a
// suggestion interpolates `{property}` into text and never passes the
// serialization redaction. prefilters are the condition's store-evaluable
// conjuncts; nil pushes only the query's own filters. want bounds the read to
// the leading rows the engine keeps; when that prefix misses want.Keep, the
// source is read whole.
func (h *nextActionHandler) queryCandidates(
	ctx context.Context, query string, prefilters []store.PropPredicate, want nextaction.Want,
) ([]nextaction.Candidate, error) {
	queries := h.queries()
	entities, cut, err := queries.executeQueryCut(ctx, query,
		queryOptions{prefilters: prefilters, firstN: want.First})
	if err == nil && cut && want.Keep != "" &&
		!slices.ContainsFunc(entities, func(e *entityPkg.Entity) bool { return e.ID == want.Keep }) {

		entities, err = queries.executeQueryWith(ctx, query, queryOptions{prefilters: prefilters})
	}
	if err != nil {
		return nil, fmt.Errorf("next-action query %q: %w", query, err)
	}
	out := make([]nextaction.Candidate, 0, len(entities))
	ctx = primeVerdicts(ctx, h.fields(), entities)
	aff := h.affordances()
	for _, e := range entities {
		out = append(out, nextaction.Candidate{Entity: redactedForSuggestion(ctx, aff, e)})
	}
	return out, nil
}

// redactedForSuggestion returns a copy of e carrying only the properties this
// principal may see.
//
// A COPY, never a mutation: the entity comes from the store's iterator and may
// be shared with a cache, so stripping in place would redact it for everyone.
//
// A package function taking the one collaborator it needs. affordanceService
// is at its plimsoll method cap, and the callers are not all next-action code
// (view conditions, export and status pages use it through
// [suggestionRedactor]).
func redactedForSuggestion(ctx context.Context, aff affordanceService, e *entityPkg.Entity) *entityPkg.Entity {
	clone := *e
	clone.Properties = aff.copyVisibleProperties(ctx, e)
	return &clone
}

// suggestionRedactor binds [redactedForSuggestion] to aff, in the shape
// [applyViewCondition] takes. aff is copied when this is called, so a caller
// that must see a later reassignment of App.affordances calls this per use.
func suggestionRedactor(aff affordanceService) func(context.Context, *entityPkg.Entity) *entityPkg.Entity {
	return func(ctx context.Context, e *entityPkg.Entity) *entityPkg.Entity {
		return redactedForSuggestion(ctx, aff, e)
	}
}

// countCandidates answers a `<entity_type> == 0` source: it yields exactly one
// entity-less candidate when the type is empty, and none otherwise.
// The count is gated by default: it counts what this principal can see, since
// an ungated count would disclose that hidden entities exist. `ungated`
// (config: count_ungated) opts out explicitly and is never inferred.
func (h *nextActionHandler) countCandidates(
	ctx context.Context, count string, ungated bool,
) ([]nextaction.Candidate, error) {
	entityType, ok := parseCountZero(count)
	if !ok {
		return nil, nil
	}

	empty, err := h.countIsZero(ctx, entityType, ungated)
	if err != nil {
		return nil, err
	}
	if !empty {
		return nil, nil
	}
	return []nextaction.Candidate{{}}, nil
}

// countIsZero reports whether the caller sees no entities of entityType.
// Unless ungated, it reads through the list's read gate, so a hidden entity does
// not count.
func (h *nextActionHandler) countIsZero(ctx context.Context, entityType string, ungated bool) (bool, error) {
	if ungated {
		_, total, err := h.store().GraphCount(ctx, store.GraphQuery{
			EntityType: entityType, Faces: store.AllFaces(),
		})
		if err != nil {
			return false, fmt.Errorf("next-action count for %q: %w", entityType, err)
		}
		return total == 0, nil
	}

	entities, err := h.scoped(ctx, entityType, nil)
	if err != nil {
		return false, fmt.Errorf("next-action count for %q: %w", entityType, err)
	}
	return len(entities) == 0, nil
}

// parseCountZero extracts the entity type from the only supported aggregate
// form, "<entity_type> == 0". Mirrors validateNextActionCount so the engine
// and the validator agree on what is accepted.
func parseCountZero(count string) (string, bool) {
	fields := strings.Fields(count)
	if len(fields) != 3 || fields[1] != "==" || fields[2] != "0" {
		return "", false
	}
	return fields[0], true
}

// SetNextActions installs the next-action per-user state backend and the
// predicate compiler backing a source's `condition:`. Until it is called the
// endpoints report "not configured". The state is disposable, so an in-memory
// backend suits a single process.
// Nil: rejected for both. A missing store would ignore snoozes and mutes, and
// a missing compiler would show suggestions the operator excluded.
func (a *App) SetNextActions(state userstate.Store, matchers NextActionMatcherFunc) error {
	if state == nil || matchers == nil {
		return errors.New("dataentry.SetNextActions: state store and matcher func must be non-nil")
	}
	if err := a.nextActions.setUserState(state); err != nil {
		return err
	}
	return a.nextActions.setMatchers(matchers)
}

// setUserState replaces the per-user state backend. Nil: rejected — see
// [App.SetNextActions].
func (h *nextActionHandler) setUserState(s userstate.Store) error {
	if s == nil {
		return errors.New("dataentry.SetNextActions: state store must be non-nil")
	}
	h.userState = s
	return nil
}

// setMatchers replaces the condition compiler. Nil: rejected — see
// [App.SetNextActions].
func (h *nextActionHandler) setMatchers(fn NextActionMatcherFunc) error {
	if fn == nil {
		return errors.New("dataentry.SetNextActions: matcher func must be non-nil")
	}
	h.matchers = fn
	return nil
}

// NextActionMatcherFunc compiles the `condition:` of every configured source
// against the current metamodel, returning a per-source lookup, the
// per-request scope binder the matchers evaluate under, and one message per
// problem.
//
// The consumer-side seam for the predicate compiler: this package must not
// import it (arch-lint keeps the condition/policy engine above the data-entry
// app), so the composition root supplies an implementation.
//
// The scope result is spelled as the bare func type (not [NextActionRequestScope])
// so a composition root that cannot import this package still satisfies the
// seam; the named type is the same signature under a name for the handler.
type NextActionMatcherFunc func(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (func(sourceID string) (nextaction.Matcher, bool), func(context.Context) (context.Context, error), []string)

// NextActionRequestScope binds request-scoped evaluation state — today the
// identity `current_user` resolves to — onto ctx ONCE per resolve, before the
// engine runs. Every matcher and pre-filter then reads what it stamped, so a
// resolve over N candidates derives the identity once, not N times, and the
// pre-filter and the authoritative pass cannot see two different identities.
//
// Nil: accepted — a compiler with no request-scoped state returns nil and the
// handler resolves on the request ctx as is. An error is a refusal of the
// whole request (the binder found the request's identity inconsistent), and
// the handler reports it rather than resolving without a scope.
type NextActionRequestScope func(ctx context.Context) (context.Context, error)

// nextActionPageMatcher is the capability of a compiled next-action matcher
// that judges a whole source at once, answering `related(...)` with the gate
// and store query it is handed (TKT-205V2N).
type nextActionPageMatcher interface {
	MatchAllWith(
		ctx context.Context, es []*entityPkg.Entity, gate relresolve.Gate, match relresolve.Match,
	) ([]bool, error)
}

// gatedNextActionMatchers hands the engine matchers that answer a condition's
// `related(...)` under the request's own read gate and world, like a view
// condition, so a suggestion never rests on an entity the reader cannot see.
func gatedNextActionMatchers(lookup nextaction.MatcherFunc, st store.GraphQueryer) nextaction.MatcherFunc {
	return func(id string) (nextaction.Matcher, bool) {
		m, ok := lookup(id)
		if !ok {
			return nil, false
		}
		pm, ok := m.(nextActionPageMatcher)
		if !ok {
			return m, true
		}
		return gatedNextActionMatcher{Matcher: m, page: pm, st: st}, true
	}
}

type gatedNextActionMatcher struct {
	nextaction.Matcher
	page nextActionPageMatcher
	st   store.GraphQueryer
}

// Prefilters forwards [ConditionPrefilterer], which the wrapper would
// otherwise hide from the candidate query.
func (g gatedNextActionMatcher) Prefilters(ctx context.Context, meta *metamodel.Metamodel) []store.PropPredicate {
	if p, ok := g.Matcher.(ConditionPrefilterer); ok {
		return p.Prefilters(ctx, meta)
	}
	return nil
}

// MatchAll implements [nextaction.BatchMatcher].
func (g gatedNextActionMatcher) MatchAll(ctx context.Context, es []*entityPkg.Entity) ([]bool, error) {
	return g.page.MatchAllWith(ctx, es, traversalGateFromContext(ctx).GateTraversal, pageMatch(g.st))
}

// nextActionValueFormatter renders message values the way the schema
// describes them: an enum value as its label, a date in its property's
// `format:` layout. The label rule is the one the metamodel API serves
// ([toV1PropertyDef]): a custom type's labels win over inline ones.
func nextActionValueFormatter(meta *metamodel.Metamodel) nextaction.ValueFormatter {
	return func(entityType, property string, v any) (string, bool) {
		if meta == nil {
			return "", false
		}
		def, ok := meta.GetEntityDef(entityType)
		if !ok {
			return "", false
		}
		pd, ok := def.Properties[property]
		if !ok {
			return "", false
		}
		if t, ok := v.(time.Time); ok && pd.Format != "" {
			return t.Format(pd.Format), true
		}
		labels := pd.Labels
		if ct, ok := meta.Types[pd.Type]; ok {
			labels = ct.Labels
		}
		if len(labels) == 0 {
			return "", false
		}
		switch t := v.(type) {
		case string:
			if l, ok := labels[t]; ok {
				return l, true
			}
		case []any:
			parts := make([]string, 0, len(t))
			for _, item := range t {
				s := nextaction.FormatValue(item)
				if l, ok := labels[s]; ok {
					s = l
				}
				parts = append(parts, s)
			}
			return strings.Join(parts, ", "), true
		}
		return "", false
	}
}
