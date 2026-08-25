package conditionlint

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// NextActionMatcher evaluates one source's compiled condition against a
// candidate. It implements the engine's consumer-side Matcher interface
// without the engine ever naming this package: the wiring site supplies it.
type NextActionMatcher struct {
	ev    *predicatefns.Evaluator
	progs NextActionPrograms
}

// Match reports whether e satisfies the condition.
//
// An entity whose type has no compiled program does NOT match. That is
// unreachable through the normal path — [CompileNextActions] compiles against
// every type the query names — but a query is filter syntax evaluated by the
// search layer, so a future widening there could yield a type this condition
// was never checked against. Refusing is the safe direction: showing a
// suggestion whose condition was never evaluated is worse than showing none.
func (m *NextActionMatcher) Match(ctx context.Context, e *entity.Entity) (bool, error) {
	if e == nil {
		return false, nil
	}
	prog, ok := m.progs[e.Type]
	if !ok {
		return false, nil
	}
	ok, err := m.ev.Matches(ctx, prog, e.Type, e.ID, e.Properties)
	if err != nil {
		// Surfaced, not swallowed. A missing date property is an eval error,
		// and treating it as "does not match" would make a broken condition
		// indistinguishable from a source that legitimately has nothing to
		// say — the silent-no-op this feature exists to remove.
		return false, fmt.Errorf("conditionlint: evaluating condition for %s: %w", e.ID, err)
	}
	return ok, nil
}

// NextActionMatchers compiles every source's condition and returns a lookup
// suitable for nextaction.WithMatchers, plus one message per problem.
//
// Compile errors are returned rather than tolerated: the caller surfaces them
// at config load, so an operator learns about a broken condition at startup
// rather than from a suggestion that never appears.
func NextActionMatchers(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (lookup func(sourceID string) (*NextActionMatcher, bool), problems []string) {
	compiled, errs := CompileNextActions(cfg, meta)
	if len(errs) > 0 {
		return nil, errs
	}
	if len(compiled) == 0 {
		return nil, nil
	}
	ev := predicatefns.NewEvaluator(meta)
	matchers := make(map[string]*NextActionMatcher, len(compiled))
	for id, progs := range compiled {
		matchers[id] = &NextActionMatcher{ev: ev, progs: progs}
	}
	return func(sourceID string) (*NextActionMatcher, bool) {
		m, ok := matchers[sourceID]
		return m, ok
	}, nil
}
