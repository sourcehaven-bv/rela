package conditionlint

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
)

// NextActionPrograms holds one compiled program per entity type a source's
// `query:` names. A condition is checked against EVERY named type, so the
// engine can evaluate a candidate whatever its type turns out to be.
type NextActionPrograms map[string]*predicate.Program

// CompileNextActions compiles the `condition:` of every next-action source.
//
// Unlike [Lint], this is AUTHORITATIVE rather than a superset sanity check:
// these expressions are evaluated server-side by predicatefns with the same
// Env compiled here, so an expression that compiles will run and one that does
// not is a real error. Compiling at config load is what makes a bad condition
// fail at startup instead of silently suppressing a suggestion forever — the
// failure mode this whole feature exists to remove.
//
// Returns the programs keyed by source id, plus one message per problem. A
// source with no `condition:` is absent from the result, which the engine
// reads as "keep every candidate".
func CompileNextActions(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (programs map[string]NextActionPrograms, problems []string) {
	if cfg == nil || meta == nil {
		return nil, nil
	}
	ev := predicatefns.NewEvaluator(meta)
	programs = make(map[string]NextActionPrograms)

	for id, src := range cfg.NextActions {
		if src.Condition == "" {
			continue
		}
		progs, msgs := compileNextActionSource(id, src, ev)
		problems = append(problems, msgs...)
		if len(progs) > 0 {
			programs[id] = progs
		}
	}
	sort.Strings(problems)
	return programs, problems
}

// compileNextActionSource compiles one source's condition against each entity
// type its query names.
func compileNextActionSource(
	id string, src dataentryconfig.NextActionSource, ev *predicatefns.Evaluator,
) (programs NextActionPrograms, problems []string) {
	where := fmt.Sprintf("next_actions[%q]", id)

	types, err := conditionEntityTypes(src)
	if err != nil {
		return nil, []string{where + ": " + err.Error()}
	}

	programs = make(NextActionPrograms, len(types))
	for _, t := range types {
		// Compiled per type and required to succeed on ALL of them. A query
		// spanning task and bug yields candidates of either, and the engine
		// cannot know which until it has one in hand — so a condition that
		// only type-checks against one of them would silently drop the
		// other's candidates. Same rule as the pushdown's
		// stringComparableOnEveryType: valid everywhere, or refused.
		prog, err := ev.Compile(t, src.Condition)
		if err != nil {
			problems = append(problems, fmt.Sprintf(
				"%s: condition does not compile against entity type %q: %v", where, t, err))
			continue
		}
		programs[t] = prog
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return programs, nil
}

// conditionEntityTypes returns the entity types a condition must compile
// against, or an error explaining why the source cannot carry one.
func conditionEntityTypes(src dataentryconfig.NextActionSource) ([]string, error) {
	switch {
	case src.Context != "":
		// A context-aware source is already scoped to one declared type.
		return []string{src.Context}, nil

	case src.Count != "":
		// A count source has no entity to evaluate against: its candidate is
		// the absence of rows. Silently ignoring the condition would leave an
		// operator believing it applied.
		return nil, errors.New("condition is not supported on a count source (there is no entity to test)")

	case src.Query != "":
		sq := searchparser.ParseQuery(src.Query)
		if len(sq.EntityTypes) == 0 {
			// Every documented next-action query names exactly one type. A
			// condition referencing entity.<prop> against "whatever the free
			// text matched" has no type to check, so require it explicitly
			// rather than guessing.
			return nil, errors.New(
				"condition requires the query to name at least one entity type (e.g. \"type:task ...\")")
		}
		return sq.EntityTypes, nil
	}
	return nil, errors.New("condition requires a query or context source")
}
