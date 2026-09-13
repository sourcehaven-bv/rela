package conditionlint

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// ViewConditionKind names the config surface a compiled condition came from,
// so a diagnostic can say `lists["open_tickets"]` rather than just the id —
// two surfaces may legitimately share one.
type ViewConditionKind string

const (
	ViewConditionList   ViewConditionKind = "lists"
	ViewConditionKanban ViewConditionKind = "kanbans"
)

// ViewConditionKey identifies one compiled view condition.
type ViewConditionKey struct {
	Kind ViewConditionKind
	ID   string
}

// String renders the key the way the config reads, for diagnostics.
func (k ViewConditionKey) String() string { return fmt.Sprintf("%s[%q]", k.Kind, k.ID) }

// CompileViewConditions compiles the `condition:` of every list and kanban.
//
// Like [CompileNextActions] and unlike [Lint], this is AUTHORITATIVE: the
// programs returned here are the ones evaluated on the read path, so an
// expression that compiles will run and one that does not is a real error.
// Compiling at config load is what makes a bad condition fail at startup
// instead of silently emptying a view — the failure mode BUG-WHEREWIDE and
// BUG-F1LTV0 both record, in opposite directions.
//
// A view declares exactly one `entity_type`, so unlike a next-action source
// there is no multi-type question: the condition is compiled against that
// type and must succeed.
//
// The Env is the request-scoped profile (CompileWithCurrentUser), matching
// next actions: a view is rendered for one signed-in principal, so a
// condition may name `current_user` and the is_current_user /
// has_current_user sugar.
//
// Returns the programs keyed by surface, plus one message per problem. A view
// with no `condition:` is absent from the result, which the read path reads as
// "no constraint".
func CompileViewConditions(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (programs map[ViewConditionKey]*predicate.Program, problems []string) {
	if cfg == nil || meta == nil {
		return nil, nil
	}
	ev := predicatefns.NewEvaluator(meta)
	programs = make(map[ViewConditionKey]*predicate.Program)

	for id, v := range cfg.Lists {
		compileOne(ev, ViewConditionKey{ViewConditionList, id},
			v.EntityType, v.Condition, programs, &problems)
	}
	for id, v := range cfg.Kanbans {
		compileOne(ev, ViewConditionKey{ViewConditionKanban, id},
			v.EntityType, v.Condition, programs, &problems)
	}

	sort.Strings(problems)
	if len(programs) == 0 {
		programs = nil
	}
	return programs, problems
}

// compileOne compiles a single surface's condition, appending a diagnostic
// rather than returning early so one bad view does not hide the next.
func compileOne(
	ev *predicatefns.Evaluator, key ViewConditionKey, entityType, condition string,
	programs map[ViewConditionKey]*predicate.Program, problems *[]string,
) {
	if condition == "" {
		return
	}
	if entityType == "" {
		// Structural validation reports the missing entity_type separately;
		// say why the condition specifically cannot be checked rather than
		// compiling against nothing and reporting a confusing attribute error.
		*problems = append(*problems, fmt.Sprintf(
			"%s: condition requires entity_type to be set", key))
		return
	}
	prog, err := ev.CompileWithCurrentUser(entityType, condition)
	if err != nil {
		*problems = append(*problems, fmt.Sprintf(
			"%s: condition does not compile against entity type %q: %v", key, entityType, err))
		return
	}
	programs[key] = prog
}
