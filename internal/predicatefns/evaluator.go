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
	now  time.Time

	mu    sync.Mutex
	cache map[string]*predicate.Program // key: type + "\x00" + source
}

// NewEvaluator returns an Evaluator bound to meta. `now` is the instant
// `today()` returns during Eval (passed in for determinism; the engine
// never reads the wall clock at eval time).
func NewEvaluator(meta *metamodel.Metamodel, now time.Time) *Evaluator {
	return &Evaluator{meta: meta, now: now, cache: map[string]*predicate.Program{}}
}

// Compile compiles a raw predicate expression for entityType, caching the
// result. The Env is entity-only (the `entity` record + the stdlib host
// funcs) — no current_user / has_role (deferred). A compile error is
// returned to the caller (surface it once, at load/flag-parse time).
func (e *Evaluator) Compile(entityType, source string) (*predicate.Program, error) {
	key := entityType + "\x00" + source
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
	def, ok := e.meta.GetEntityDef(entityType)
	if !ok {
		return false, fmt.Errorf("predicatefns: unknown entity type %q", entityType)
	}
	b := predicate.NewBindings()
	if err := b.SetVar("entity", EntityRecord(e.meta, def, id, entityType, props)); err != nil {
		return false, err
	}
	if err := Bind(b, e.now); err != nil {
		return false, err
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
