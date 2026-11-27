package conditionlint

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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
	// ViewConditionAction is an action's `when:` (TKT-VVS16W). Its key ID is
	// [dataentryconfig.ActionConditionID]: one program per listed type.
	ViewConditionAction ViewConditionKind = "actions"

	// ViewConditionNavStatus is a rule of a navigation entry's `status:`,
	// keyed by dataentryconfig.NavStatusConditionID. It is compiled against
	// the entity type of the list the entry opens, since the rule counts that
	// list's rows.
	ViewConditionNavStatus ViewConditionKind = "nav_status"

	// ViewConditionPageDone is the `done_when:` of a tab on a steps page
	// (TKT-MWZF0UO), keyed by dataentryconfig.PageDoneConditionID. It is
	// compiled against the entity type of the tab's list, since it judges
	// that list's rows.
	ViewConditionPageDone ViewConditionKind = "page_done"

	// ViewConditionTraverse is the `condition:` of a view's traverse step,
	// keyed by dataentryconfig.TraverseConditionID: one program per entity
	// type the step can collect.
	ViewConditionTraverse ViewConditionKind = "traverse"

	// ViewConditionLLMCandidate is the `candidates:` of an llm action's relate
	// output (TKT-4USH1HO), keyed by dataentryconfig.LLMCandidateConditionID:
	// one program per target type of the relation.
	ViewConditionLLMCandidate ViewConditionKind = "llm_candidates"

	// ViewConditionListGroup is the `condition:` of one computed group of a
	// list's `group_by` (TKT-QS024L0), keyed by
	// dataentryconfig.ListGroupConditionID. It is compiled against the list's
	// entity type.
	ViewConditionListGroup ViewConditionKind = "list_groups"
)

// ViewConditionKey identifies one compiled view condition.
type ViewConditionKey struct {
	Kind ViewConditionKind
	ID   string
}

// String renders the key the way the config reads, for diagnostics.
func (k ViewConditionKey) String() string { return fmt.Sprintf("%s[%q]", k.Kind, k.ID) }

// CompileViewConditions compiles the `condition:` of every list, kanban and
// view traverse step, the `when:` of every detail-page action, and every rule
// of a navigation entry's `status:` and the `done_when:` of every page tab.
//
// Like [CompileNextActions] and unlike [Lint], this is AUTHORITATIVE: the
// programs returned here are the ones evaluated on the read path, so an
// expression that compiles will run and one that does not is a real error.
// Compiling at config load is what makes a bad condition fail at startup
// instead of silently emptying a view — the failure mode BUG-WHEREWIDE and
// BUG-F1LTV0 both record, in opposite directions.
//
// A list or kanban declares exactly one `entity_type`, so the condition is
// compiled against that type and must succeed. A traverse step can collect
// several types and compiles once per type (see compileTraverseConditions).
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
		key := ViewConditionKey{ViewConditionList, id}
		compileOne(ev, meta, key, key.String(), v.EntityType, v.Condition, programs, &problems)
	}
	for id, v := range cfg.Lists {
		if !v.GroupBy.Computed() {
			continue
		}
		for _, g := range v.GroupBy.Groups {
			key := ViewConditionKey{ViewConditionListGroup, dataentryconfig.ListGroupConditionID(id, g.ID)}
			name := fmt.Sprintf("lists[%q].group_by.groups[%s]", id, g.ID)
			compileOne(ev, meta, key, name, v.EntityType, g.Condition, programs, &problems)
		}
	}
	for id, v := range cfg.Kanbans {
		key := ViewConditionKey{ViewConditionKanban, id}
		compileOne(ev, meta, key, key.String(), v.EntityType, v.Condition, programs, &problems)
	}
	for _, ns := range dataentryconfig.NavStatusEntries(cfg) {
		entityType := cfg.Lists[ns.List].EntityType
		for i, rule := range ns.Entry.Status {
			key := ViewConditionKey{ViewConditionNavStatus, dataentryconfig.NavStatusConditionID(ns.Key, i)}
			name := fmt.Sprintf("navigation[%q].status[%d]", ns.Entry.Label, i)
			if ns.Space != "" {
				name = fmt.Sprintf("spaces[%s].%s", ns.Space, name)
			}
			compileOne(ev, meta, key, name, entityType, rule.Condition, programs, &problems)
		}
	}
	for _, pd := range dataentryconfig.PageDoneEntries(cfg) {
		key := ViewConditionKey{ViewConditionPageDone, dataentryconfig.PageDoneConditionID(pd.Page, pd.Tab.ID)}
		name := fmt.Sprintf("pages[%s].tabs[%s].done_when", pd.Page, pd.Tab.ID)
		compileOne(ev, meta, key, name, pd.EntityType, pd.Tab.DoneWhen, programs, &problems)
	}
	compileActionConditions(ev, meta, cfg, programs, &problems)
	compileLLMCandidateConditions(ev, meta, cfg, programs, &problems)
	compileTraverseConditions(ev, meta, cfg, programs, &problems)
	problems = append(problems, assignExprProblems(cfg, meta)...)

	slices.Sort(problems)
	if len(programs) == 0 {
		programs = nil
	}
	return programs, problems
}

// compileActionConditions compiles each detail-page action's `when:` once per
// type in its available_on.
//
// `related(...)` is refused. A detail action's when decides one button on one
// entity, and keeping it a pure check of that entity's own properties keeps
// the affordance and the POST gate a single cheap evaluation. A traversal
// would also need the page-level batching list conditions use, for a page of
// one row.
func compileActionConditions(
	ev *predicatefns.Evaluator, meta *metamodel.Metamodel, cfg *dataentryconfig.Config,
	programs map[ViewConditionKey]*predicate.Program, problems *[]string,
) {
	for id, a := range cfg.Actions {
		if a.When == "" || a.AvailableOn == nil {
			// A when without available_on is a structural error that
			// validateActions reports; there is no type to compile against.
			continue
		}
		for _, et := range a.AvailableOn.EntityTypes {
			if _, ok := meta.GetEntityDef(et); !ok {
				continue // reported by validateActions
			}
			key := ViewConditionKey{ViewConditionAction, dataentryconfig.ActionConditionID(id, et)}
			prog, err := ev.CompileWithCurrentUser(et, a.When)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf(
					"actions[%q]: when does not compile against entity type %q: %v", id, et, err))
				continue
			}
			if len(prog.Traversals()) > 0 {
				*problems = append(*problems, fmt.Sprintf(
					"actions[%q]: %s(...) is not available in an action's when; it may only "+
						"test the entity's own properties", id, predicate.FuncRelated))
				continue
			}
			programs[key] = prog
		}
	}
}

// compileLLMCandidateConditions compiles the `candidates:` of each llm
// action's relate output once per target type of its relation.
//
// `related(...)` is refused, as for an action's when: the candidate list is
// read once per run, and a traversal per candidate would multiply its cost.
// Every target type must compile, so a candidate of any type is judged.
func compileLLMCandidateConditions(
	ev *predicatefns.Evaluator, meta *metamodel.Metamodel, cfg *dataentryconfig.Config,
	programs map[ViewConditionKey]*predicate.Program, problems *[]string,
) {
	for id, a := range cfg.Actions {
		if a.LLM == nil {
			continue
		}
		for i, r := range a.LLM.Outputs.Relate {
			if r.Candidates == "" {
				continue
			}
			rd, ok := meta.GetRelationDef(r.Relation)
			if !ok {
				continue // reported by validateActions
			}
			for _, et := range rd.To {
				if _, ok := meta.GetEntityDef(et); !ok {
					continue
				}
				key := ViewConditionKey{ViewConditionLLMCandidate, dataentryconfig.LLMCandidateConditionID(id, i, et)}
				prog, err := ev.CompileWithCurrentUser(et, r.Candidates)
				if err != nil {
					*problems = append(*problems, fmt.Sprintf(
						"actions[%q]: llm.outputs.relate[%d].candidates does not compile against entity type %q: %v",
						id, i, et, err))
					continue
				}
				if len(prog.Traversals()) > 0 {
					*problems = append(*problems, fmt.Sprintf(
						"actions[%q]: %s(...) is not available in llm.outputs.relate[%d].candidates; it may only "+
							"test the candidate's own properties", id, predicate.FuncRelated, i))
					continue
				}
				programs[key] = prog
			}
		}
	}
}

// compileTraverseConditions compiles the `condition:` of every view traverse
// step once per entity type the step can collect, narrowed by a `where:` on
// `type` (dataentryconfig.TraverseConditionTypes).
//
// Every type must compile, as for a next-action source spanning several
// types: the step learns an entity's type only when it holds one, so a
// condition valid for some types would silently drop the others. The Env
// makes that rule usable: `entity.type` is readable, and a property another
// collected type declares is nil-typed (CompileForCollection).
//
// `related(...)` is refused. The step runs inside the view's fixpoint loop,
// up to ten passes per rule, and a store query per pass per type is a cost
// the view should not hide.
//
// A step with an unknown relation has no type set; validateViews reports the
// relation, so it is skipped here.
func compileTraverseConditions(
	ev *predicatefns.Evaluator, meta *metamodel.Metamodel, cfg *dataentryconfig.Config,
	programs map[ViewConditionKey]*predicate.Program, problems *[]string,
) {
	for viewID, view := range cfg.Views {
		for i, t := range view.Traverse {
			if t.Condition == "" {
				continue
			}
			name := fmt.Sprintf("views[%q].traverse[%d]", viewID, i)
			types := dataentryconfig.TraverseConditionTypes(t, meta)
			for _, et := range types {
				if _, ok := meta.GetEntityDef(et); !ok {
					continue // an undeclared relation endpoint is the metamodel's to report
				}
				prog, err := ev.CompileForCollection(et, types, t.Condition)
				if err != nil {
					*problems = append(*problems, fmt.Sprintf(
						"%s: condition does not compile against entity type %q: %v", name, et, err))
					continue
				}
				if len(prog.Traversals()) > 0 {
					*problems = append(*problems, fmt.Sprintf(
						"%s: %s(...) is not available in a traverse condition", name, predicate.FuncRelated))
					continue
				}
				id := dataentryconfig.TraverseConditionID(et, types, t.Condition)
				programs[ViewConditionKey{ViewConditionTraverse, id}] = prog
			}
		}
	}
}

// ViewConditionMatcher evaluates one view's compiled condition against a row.
//
// The entity type is fixed at compile time (a view declares exactly one), so
// unlike [NextActionMatcher] there is no per-type program map and no
// unreachable-type case to refuse.
type ViewConditionMatcher struct {
	ev         *predicatefns.Evaluator
	prog       *predicate.Program
	entityType string
}

// Matches evaluates the condition against the identity on ctx.
//
// The matcher evaluates with [predicatefns.Evaluator.LoweringSemantics], so
// an integer or string ordered comparison on an unset value is false, as it
// is in the store query the condition may be lowered into (TKT-8HY05WB).
// Any other evaluation error, a missing date property among them, is
// SURFACED, not swallowed as a non-match: treating it as "does not match"
// would silently narrow the view with no diagnostic (the inverse of
// BUG-WHEREWIDE).
func (m *ViewConditionMatcher) Matches(ctx context.Context, e *entity.Entity) (bool, error) {
	if m == nil || e == nil {
		return false, nil
	}
	ok, err := m.ev.MatchesAs(ctx, m.prog, m.entityType, e.ID, e.Properties)
	if err != nil {
		return false, fmt.Errorf("conditionlint: evaluating view condition for %s: %w", e.ID, err)
	}
	return ok, nil
}

// MatchesWith is [ViewConditionMatcher.Matches] for a condition using
// `related(...)`: traversal answers each one for e. Nil is fine for a
// condition without one.
func (m *ViewConditionMatcher) MatchesWith(
	ctx context.Context, e *entity.Entity, traversal predicate.TraversalFunc,
) (bool, error) {
	if m == nil || e == nil {
		return false, nil
	}
	ok, err := m.ev.MatchesWithTraversals(ctx, m.prog, m.entityType, e.ID, e.Properties, traversal)
	if err != nil {
		return false, fmt.Errorf("conditionlint: evaluating view condition for %s: %w", e.ID, err)
	}
	return ok, nil
}

// Program returns the compiled condition, so a caller can answer its
// traversals for a batch of rows before evaluating them.
func (m *ViewConditionMatcher) Program() *predicate.Program { return m.prog }

// EntityAttributes returns the entity fields the condition reads. The
// compiler rejects dynamic attribute access, so the set is complete.
func (m *ViewConditionMatcher) EntityAttributes() []string { return m.prog.Attributes("entity") }

// EntityType returns the entity type the condition was compiled against.
func (m *ViewConditionMatcher) EntityType() string { return m.entityType }

// ViewConditionMatchers compiles every view condition and returns a lookup
// keyed by (kind, id) — the pair the config uses, since a list and a kanban
// may share an id.
//
// The lookup holds every condition that compiled, even when others did not,
// and the problems name the ones that did not. A surface whose condition
// failed is absent, so a caller must not read absence as "no constraint"
// for a surface its config gives a condition: it fails closed instead. One
// broken condition then disables only its own surface. Nil when nothing
// compiled.
func ViewConditionMatchers(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (lookup func(kind, id string) (*ViewConditionMatcher, bool), problems []string) {
	programs, problems := CompileViewConditions(cfg, meta)
	if len(programs) == 0 {
		return nil, problems
	}
	ev := predicatefns.NewEvaluator(meta).LoweringSemantics()
	matchers := make(map[ViewConditionKey]*ViewConditionMatcher, len(programs))
	for key, prog := range programs {
		matchers[key] = &ViewConditionMatcher{ev: ev, prog: prog, entityType: entityTypeFor(cfg, key)}
	}
	return func(kind, id string) (*ViewConditionMatcher, bool) {
		m, ok := matchers[ViewConditionKey{ViewConditionKind(kind), id}]
		return m, ok
	}, problems
}

// entityTypeFor reads back the entity type a key's view declares. Compilation
// already proved it non-empty and known, so this cannot fail for a key that
// reached the matcher map.
func entityTypeFor(cfg *dataentryconfig.Config, key ViewConditionKey) string {
	switch key.Kind {
	case ViewConditionList:
		return cfg.Lists[key.ID].EntityType
	case ViewConditionKanban:
		return cfg.Kanbans[key.ID].EntityType
	case ViewConditionListGroup:
		return cfg.Lists[dataentryconfig.ListGroupConditionList(key.ID)].EntityType
	case ViewConditionAction:
		_, et, _ := dataentryconfig.SplitActionConditionID(key.ID)
		return et
	case ViewConditionTraverse:
		et, _ := dataentryconfig.SplitTraverseConditionID(key.ID)
		return et
	case ViewConditionLLMCandidate:
		return dataentryconfig.LLMCandidateConditionType(key.ID)
	case ViewConditionPageDone:
		for _, pd := range dataentryconfig.PageDoneEntries(cfg) {
			if dataentryconfig.PageDoneConditionID(pd.Page, pd.Tab.ID) == key.ID {
				return pd.EntityType
			}
		}
	case ViewConditionNavStatus:
		for _, ns := range dataentryconfig.NavStatusEntries(cfg) {
			for i := range ns.Entry.Status {
				if dataentryconfig.NavStatusConditionID(ns.Key, i) == key.ID {
					return cfg.Lists[ns.List].EntityType
				}
			}
		}
	}
	return ""
}

// compileOne compiles a single surface's condition, appending a diagnostic
// rather than returning early so one bad view does not hide the next.
//
// name is how a diagnostic refers to the surface: the key itself for a view,
// the entry label and rule index for a navigation status rule.
func compileOne(
	ev *predicatefns.Evaluator, meta *metamodel.Metamodel, key ViewConditionKey, name, entityType, condition string,
	programs map[ViewConditionKey]*predicate.Program, problems *[]string,
) {
	if condition == "" {
		return
	}
	if entityType == "" {
		// Structural validation reports the missing entity_type separately;
		// say why the condition specifically cannot be checked rather than
		// compiling against nothing and reporting a confusing attribute error.
		*problems = append(*problems, name+": condition requires entity_type to be set")
		return
	}
	prog, err := ev.CompileWithCurrentUser(entityType, condition)
	if err != nil {
		*problems = append(*problems, fmt.Sprintf(
			"%s: condition does not compile against entity type %q: %v", name, entityType, err))
		return
	}
	if err := predicatefns.ValidateTraversals(meta, entityType, prog); err != nil {
		*problems = append(*problems, fmt.Sprintf("%s: %v", name, err))
		return
	}
	programs[key] = prog
}

// compileAssignExpr compiles an `assign:` value expression (TKT-FGE5G3I) for
// one property. It takes no entity: the value is the same for every item, so
// `entity` and `related(...)` are unavailable.
func compileAssignExpr(meta *metamodel.Metamodel, def *metamodel.PropertyDef, expr string) (*predicate.Program, error) {
	typ, ok := predicatefns.ScalarTypeForProp(meta, def)
	if !ok {
		return nil, fmt.Errorf("property type %q is not supported by expressions", def.Type)
	}
	env := predicate.NewEnv()
	if err := predicatefns.Declare(env); err != nil {
		return nil, err
	}
	prog, err := predicate.CompileValue(env, expr, predicate.ValueProfile(typ))
	if err != nil {
		return nil, err
	}
	if len(prog.Traversals()) > 0 {
		return nil, fmt.Errorf("%s(...) is not available in an assign expression", predicate.FuncRelated)
	}
	return prog, nil
}

// assignExprProblems compiles every expression of every list and page
// `assign:`, so one that does not compile fails the load.
func assignExprProblems(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) []string {
	var problems []string
	check := func(at, itemType string, a *dataentryconfig.Assign) {
		if a == nil {
			return
		}
		def, ok := meta.GetEntityDef(itemType)
		if !ok {
			return // reported by dataentryconfig
		}
		for _, prop := range slices.Sorted(maps.Keys(a.Properties)) {
			v := a.Properties[prop]
			pd, ok := def.Properties[prop]
			if v.Expr == "" || !ok {
				continue
			}
			if _, err := compileAssignExpr(meta, &pd, v.Expr); err != nil {
				problems = append(problems, fmt.Sprintf("%s: property %q: expr does not compile: %v", at, prop, err))
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Lists)) {
		l := cfg.Lists[id]
		check(fmt.Sprintf("list %q: assign", id), l.EntityType, l.Assign)
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Pages)) {
		p := cfg.Pages[id]
		if p.Assign != nil {
			check(fmt.Sprintf("pages[%s]: assign", id), p.Assign.EntityType, p.Assign)
		}
	}
	return problems
}

// EvalAssignExpr evaluates an `assign:` value expression with the clock now,
// the one list conditions use, so an assigned today() and a list's today()
// agree. The result is in the property's wire form; a date is formatted in
// the property's date format, and nil means unset.
func EvalAssignExpr(
	ctx context.Context, meta *metamodel.Metamodel, def *metamodel.PropertyDef, expr string, now time.Time,
) (any, error) {
	prog, err := compileAssignExpr(meta, def, expr)
	if err != nil {
		return nil, err
	}
	b := predicate.NewBindings()
	if bindErr := predicatefns.Bind(b, now); bindErr != nil {
		return nil, bindErr
	}
	v, err := prog.Eval(ctx, b)
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case predicate.Nil:
		return nil, nil //nolint:nilnil // nil is the value "unset", not a missing result
	case predicate.String:
		return x.String(), nil
	case predicate.Int:
		return x.Int64(), nil
	case predicate.Bool:
		return x.Bool(), nil
	case predicate.Date:
		return x.Time().Format(def.GetDateFormat()), nil
	case predicate.Decimal:
		text, ok := x.Text()
		if !ok {
			return nil, errors.New("decimal result has no finite form")
		}
		return text, nil
	default:
		return nil, fmt.Errorf("unsupported result %T", v)
	}
}
