package predicatefns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// Evaluator compiles predicate expressions against one metamodel and
// evaluates them against entities. It owns a compiled-Program cache keyed
// by (entityType, source) — scoped to THIS Evaluator instance, which is
// bound to a single metamodel, so the cache can never mix Programs across
// metamodels with different field layouts/date formats (RR-2Y851X). Build
// one Evaluator per metamodel and share it; it is safe for concurrent use.
//
// It fronts both entry points of the convergence:
//   - Compile(type, expr): a raw predicate expression (`--filter`).
//   - CompileFilter(type, filters): legacy filter strings (`--where`,
//     automation/validation), transpiled via FromFilter then compiled.
//
// Both return a *Program the caller evaluates per entity with Matches.
type Evaluator struct {
	meta *metamodel.Metamodel
	// now supplies the instant `today()` returns, evaluated per Matches
	// call. It is a func (not a captured time.Time) so a long-lived
	// Evaluator — the automation Engine and validation Service each hold
	// one for the process lifetime — sees the date advance rather than
	// freezing today() at construction (RR-FUD017).
	now func() time.Time

	mu    sync.Mutex
	cache map[string]*predicate.Program // key: type + "\x00" + source
}

// NewEvaluator returns an Evaluator bound to meta, with today() sourced
// from time.Now() per evaluation. Use NewEvaluatorWithClock to pin the
// clock (tests).
func NewEvaluator(meta *metamodel.Metamodel) *Evaluator {
	return NewEvaluatorWithClock(meta, time.Now)
}

// NewEvaluatorWithClock returns an Evaluator whose today() reads `now`
// on each Matches call. `now` must be non-nil.
func NewEvaluatorWithClock(meta *metamodel.Metamodel, now func() time.Time) *Evaluator {
	return &Evaluator{meta: meta, now: now, cache: map[string]*predicate.Program{}}
}

// Compile compiles a raw predicate expression for entityType, caching the
// result. The Env is entity-only (the `entity` record + the stdlib host
// funcs) — no current_user / has_role (deferred). A compile error is
// returned to the caller (surface it once, at load/flag-parse time).
func (e *Evaluator) Compile(entityType, source string) (*predicate.Program, error) {
	return e.compile(entityType, source, false)
}

// CompileWithCurrentUser is [Evaluator.Compile] for a REQUEST-SCOPED
// surface: the Env additionally carries current_user and its sugar (see
// [DeclareCurrentUser]), so `is_current_user(entity.assignee)` compiles.
//
// It is a separate entry point rather than a flag on Compile because the
// two profiles must not be confused at a call site. A program compiled
// here REQUIRES an identity to evaluate — [Evaluator.MatchesAs] returns
// [ErrNoCurrentUser] without one — so only a caller that can guarantee a
// resolved principal may use it. Everything compiled through Compile
// stays principal-free and evaluable with context.Background(), which is
// what internal/validation does.
func (e *Evaluator) CompileWithCurrentUser(entityType, source string) (*predicate.Program, error) {
	return e.compile(entityType, source, true)
}

func (e *Evaluator) compile(entityType, source string, withUser bool) (*predicate.Program, error) {
	// The profile is part of the cache key. Without it the two Envs would
	// collide: whichever spelling compiled first would be served to the
	// other, so a validation rule could silently receive a Program that
	// expects a current_user binding it will never get (or vice versa,
	// masking the load error that is the whole point of the split).
	profile := "base"
	if withUser {
		profile = "user"
	}
	key := profile + "\x00" + entityType + "\x00" + source
	e.mu.Lock()
	defer e.mu.Unlock()
	if prog, ok := e.cache[key]; ok {
		return prog, nil
	}
	def, ok := e.meta.GetEntityDef(entityType)
	if !ok {
		return nil, fmt.Errorf("predicatefns: unknown entity type %q", entityType)
	}
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", EntityRecordType(e.meta, def)); err != nil {
		return nil, err
	}
	if err := Declare(env); err != nil {
		return nil, err
	}
	if withUser {
		if err := DeclareCurrentUser(env); err != nil {
			return nil, err
		}
	}
	prog, err := predicate.Compile(env, source)
	if err != nil {
		return nil, err
	}
	e.cache[key] = prog
	return prog, nil
}

// CompileFilter transpiles legacy filter clauses (ANDed) for entityType
// via FromFilter, then compiles the combined predicate. Returns a
// transpile/compile error unmodified so the caller can surface it at
// load time.
func (e *Evaluator) CompileFilter(entityType string, filters []*filter.Filter) (*predicate.Program, error) {
	def, ok := e.meta.GetEntityDef(entityType)
	if !ok {
		return nil, fmt.Errorf("predicatefns: unknown entity type %q", entityType)
	}
	src, err := AndFilters(e.meta, def, filters)
	if err != nil {
		return nil, err
	}
	return e.Compile(entityType, src)
}

// Matches binds the entity's properties and evaluates prog. An Eval error
// (e.g. an off-type binding the type checker couldn't catch) is returned;
// callers on a filtering path typically treat a non-nil error as "does
// not match" but it is surfaced so a bug isn't swallowed.
func (e *Evaluator) Matches(
	ctx context.Context, prog *predicate.Program, entityType, id string, props map[string]any,
) (bool, error) {
	return e.matches(ctx, prog, entityType, id, props, false)
}

// MatchesAs evaluates a Program compiled by [Evaluator.CompileWithCurrentUser],
// binding the query identity carried by ctx (see [WithQueryIdentity]).
//
// Returns [ErrNoCurrentUser] if ctx carries no identity. That is a hard
// failure rather than a non-match: a caller that reached this point has
// already accepted a condition referencing the current user, and the
// honest outcomes are "the right rows" or "an error" — never "somebody
// else's rows".
func (e *Evaluator) MatchesAs(
	ctx context.Context, prog *predicate.Program, entityType, id string, props map[string]any,
) (bool, error) {
	return e.matches(ctx, prog, entityType, id, props, true)
}

func (e *Evaluator) matches(
	ctx context.Context, prog *predicate.Program, entityType, id string,
	props map[string]any, withUser bool,
) (bool, error) {
	def, ok := e.meta.GetEntityDef(entityType)
	if !ok {
		return false, fmt.Errorf("predicatefns: unknown entity type %q", entityType)
	}
	b := predicate.NewBindings()
	if err := b.SetVar("entity", EntityRecord(e.meta, def, id, entityType, props)); err != nil {
		return false, err
	}
	if err := Bind(b, e.now()); err != nil {
		return false, err
	}
	if withUser {
		if err := BindCurrentUser(ctx, b); err != nil {
			return false, err
		}
	}
	v, err := prog.Eval(ctx, b)
	if err != nil {
		return false, err
	}
	bv, ok := v.(predicate.Bool)
	if !ok {
		return false, errors.New("predicatefns: predicate did not return bool")
	}
	return bv.Bool(), nil
}

// AndFilters transpiles a slice of filter clauses to a single predicate
// source expression, ANDing them (matching filter.MatchAll semantics). An
// empty slice yields "true". Each clause goes through FromFilter, so an
// unsupported clause surfaces its transpile error.
func AndFilters(meta *metamodel.Metamodel, def *metamodel.EntityDef, filters []*filter.Filter) (string, error) {
	if len(filters) == 0 {
		return "true", nil
	}
	var b strings.Builder
	for i, f := range filters {
		src, err := FromFilter(meta, def, f)
		if err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteString(" and ")
		}
		b.WriteString("(")
		b.WriteString(src)
		b.WriteString(")")
	}
	return b.String(), nil
}
