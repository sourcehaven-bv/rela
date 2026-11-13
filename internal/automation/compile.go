package automation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// trigger is one automation compiled onto a flow trigger (TKT-GS7THPA): the
// event keys and scope that select it, the guard [flow.Fire] evaluates, and
// the outputs a firing renders into a [Result]. The table of triggers is
// built once by the constructor and only read afterwards, so Process takes
// no lock to read it.
type trigger struct {
	name string
	// on holds the event keys and scope as written.
	on Trigger
	// when is the `when:` clauses, compiled per entity type.
	when clauseSet
	// condition is the `condition:` program per entity type named in
	// `entity:`; nil without a condition or a metamodel. conditionErr holds
	// the compile error of a type whose program is missing.
	condition    map[string]*predicate.Program
	conditionErr map[string]error
	outputs      []output
	checks       []check
}

// clauseKind is how one filter clause is judged. The three kinds are the
// verdicts the engine has always used: a predicate program for a declared
// property, the legacy filter.Match verdict for a clause the predicate
// engine cannot transpile (RR-G9KT8H), and string matching for an
// undeclared property or a type the metamodel does not know.
type clauseKind int

const (
	clauseString clauseKind = iota
	clausePredicate
	clauseLegacy
)

// clause is one compiled filter clause.
type clause struct {
	kind   clauseKind
	filter *filter.Filter
	prog   *predicate.Program
	// prop is the declared property, for a legacy clause.
	prop metamodel.PropertyDef
}

// clauseSet is a list of filter clauses compiled per entity type. A type
// with no entry is judged with string matching.
type clauseSet struct {
	filters []*filter.Filter
	byType  map[string][]clause
}

// outputKind is what an output writes. set, create and relate are the flow
// output kinds (see flowcfg.OutputKind); pile, notify and script are named
// exceptions with no flow counterpart yet.
type outputKind int

const (
	outputSet outputKind = iota
	outputRelate
	outputPile
	outputNotify
	outputCreate
	outputScript
	outputScriptFile
)

// output is one output of a trigger. action is the index of its `do:` entry:
// a create that fails skips the rest of the entry, as it always has.
type output struct {
	kind   outputKind
	action int
	act    *Action
}

// check is one compiled `validate:` entry. An unparseable check stays a
// runtime warning, as before; it is not a load error.
type check struct {
	Validation
	invalid bool
	clauses clauseSet
}

// compile builds the trigger table from e.automations. Each constructor
// calls it once, after the metamodel and options are set.
func (e *Engine) compile() {
	e.triggers = make([]trigger, len(e.automations))
	e.byEvent = make(map[EventType][]int)
	for i := range e.automations {
		auto := &e.automations[i]
		types := e.compileTypes(auto.On)
		t := trigger{
			name: auto.Name,
			on:   auto.On,
			when: e.compileClauses(types, auto.On.When),
		}
		if src := auto.On.Condition; src != "" && e.meta != nil {
			ev := e.evaluator()
			t.condition = make(map[string]*predicate.Program, len(auto.On.Entity))
			t.conditionErr = make(map[string]error)
			for _, typ := range auto.On.Entity {
				// compileConditions compiled every one of these at load and
				// fails the load on an error, so this is a cache hit. An error
				// is kept anyway, so the runtime warning names its cause.
				prog, err := ev.Compile(typ, src)
				if err != nil {
					t.conditionErr[typ] = err
					continue
				}
				t.condition[typ] = prog
			}
		}
		for ai := range auto.Do {
			t.outputs = append(t.outputs, compileOutputs(ai, &auto.Do[ai])...)
		}
		for _, v := range auto.Validate {
			c := check{Validation: v}
			f, err := filter.Parse(v.Check)
			if err != nil {
				c.invalid = true
			} else {
				c.clauses = e.compileClauses(types, []*filter.Filter{f})
			}
			t.checks = append(t.checks, c)
		}
		e.triggers[i] = t
		for _, et := range eventTypes {
			if mayFire(auto.On, et) {
				e.byEvent[et] = append(e.byEvent[et], i)
			}
		}
	}
}

// eventTypes are the events an engine dispatches.
var eventTypes = []EventType{
	EventEntityCreated, EventEntityUpdated, EventRelationCreated, EventRelationRemoved,
	EventEntityRenamed, EventEntityDeleted,
}

// mayFire reports whether an event of type et can fire a trigger with keys
// on. It is a superset prefilter for the event index; Engine.selects is
// the authority, and TestMayFireCoversSelects pins the superset.
func mayFire(on Trigger, et EventType) bool {
	copyKey := len(on.Copy) > 0
	switch et {
	case EventEntityCreated:
		return on.Created || copyKey
	case EventEntityUpdated:
		return on.Updated || on.Property != "" || copyKey
	case EventEntityRenamed:
		return on.Created || on.Updated || on.Property != "" || copyKey
	case EventEntityDeleted:
		return on.Deleted
	case EventRelationCreated:
		return on.RelationCreated != ""
	case EventRelationRemoved:
		return on.RelationRemoved != ""
	}
	return false
}

// compileTypes returns the entity types a trigger's clauses compile for:
// the types `entity:` names, or every declared type for a trigger without
// one. Without a metamodel there is nothing to compile against.
//
// A type-less trigger therefore compiles its clauses for every declared
// type at load, rather than for the types that are written. The cost is
// automations x types x clauses programs, once, in place of a compile on the
// first write of each type.
func (e *Engine) compileTypes(on Trigger) []string {
	if e.meta == nil {
		return nil
	}
	if len(on.Entity) > 0 {
		return on.Entity
	}
	return slices.Sorted(maps.Keys(e.meta.Entities))
}

// compileClauses compiles filters for each of types. A type the metamodel
// does not know gets no entry and is judged with string matching.
func (e *Engine) compileClauses(types []string, filters []*filter.Filter) clauseSet {
	cs := clauseSet{filters: filters}
	if len(filters) == 0 || len(types) == 0 {
		return cs
	}
	ev := e.evaluator()
	cs.byType = make(map[string][]clause, len(types))
	for _, typ := range types {
		def, ok := e.meta.GetEntityDef(typ)
		if !ok {
			continue
		}
		clauses := make([]clause, len(filters))
		for i, f := range filters {
			prop, declared := def.Properties[f.Property]
			switch {
			case !declared:
				clauses[i] = clause{kind: clauseString, filter: f}
			default:
				prog, err := ev.CompileFilter(typ, []*filter.Filter{f})
				if err != nil {
					// Untranspilable (fuzzy with a wildcard, or an operator
					// and type FromFilter rejects): keep filter.Match's
					// verdict rather than the string matcher, which would
					// accept operators filter.Match rejects (RR-G9KT8H).
					clauses[i] = clause{kind: clauseLegacy, filter: f, prop: prop}
				} else {
					clauses[i] = clause{kind: clausePredicate, filter: f, prog: prog}
				}
			}
		}
		cs.byType[typ] = clauses
	}
	return cs
}

// compileOutputs compiles one `do:` entry. An entry may carry several keys;
// they render in the order the engine has always applied them.
func compileOutputs(ai int, act *Action) []output {
	var out []output
	add := func(k outputKind) { out = append(out, output{kind: k, action: ai, act: act}) }
	if act.Set != "" {
		add(outputSet)
	}
	if act.CreateRelation != nil {
		add(outputRelate)
	}
	if act.AddToPile != nil {
		add(outputPile)
	}
	if act.Notify != nil {
		add(outputNotify)
	}
	if act.CreateEntity != nil {
		add(outputCreate)
	}
	if act.Lua != "" {
		add(outputScript)
	}
	if act.LuaFile != "" {
		add(outputScriptFile)
	}
	return out
}

// clausesFor returns the compiled clauses for an entity type. The lookup
// tries the type as written, then its canonical name, so an alias uses the
// clauses of the type it names. The program still sees the type as written,
// as before. nil means string matching.
func (e *Engine) clausesFor(cs clauseSet, typ string) []clause {
	if cs.byType == nil {
		return nil
	}
	if c, ok := cs.byType[typ]; ok {
		return c
	}
	return cs.byType[e.meta.ResolveAlias(typ)]
}

// matchClauses reports whether ent satisfies every clause of cs, stopping at
// the first that fails.
func (e *Engine) matchClauses(ctx context.Context, cs clauseSet, ent *entity.Entity) bool {
	compiled := e.clausesFor(cs, ent.Type)
	for i, f := range cs.filters {
		c := clause{kind: clauseString, filter: f}
		if compiled != nil {
			c = compiled[i]
		}
		if !e.matchClause(ctx, c, ent) {
			return false
		}
	}
	return true
}

// matchClause judges one clause against ent. An evaluation error is no
// match.
func (e *Engine) matchClause(ctx context.Context, c clause, ent *entity.Entity) bool {
	switch c.kind {
	case clausePredicate:
		m, err := e.evaluator().Matches(ctx, c.prog, ent.Type, ent.ID, ent.Properties)
		return err == nil && m
	case clauseLegacy:
		rec := filter.Record{ID: ent.ID, Type: ent.Type, Properties: ent.Properties}
		m, err := filter.Match(rec, c.filter, &c.prop, e.meta)
		return err == nil && m
	default:
		return matchSimple(ent.Properties[c.filter.Property], c.filter)
	}
}

// holds evaluates a trigger's guard: its `when:` clauses first, stopping at
// the first that fails, then its `condition:`. The order matters: the
// condition may bind traversals in the store, and its evaluation error is
// reported as a warning, so it runs only when every clause holds.
func (e *Engine) holds(ctx context.Context, t *trigger, ent *entity.Entity) (bool, error) {
	if len(t.on.When) == 0 && t.on.Condition == "" {
		return true, nil
	}
	if ent == nil {
		return false, nil
	}
	if !e.matchClauses(ctx, t.when, ent) {
		return false, nil
	}
	return e.matchesCondition(ctx, t, ent)
}

// matchesCondition evaluates the trigger's `condition:`.
//
// The program was compiled at load, so a failure here is an EVAL error (a
// missing property binding, the step budget) rather than a syntax mistake.
// The automation does NOT fire, since it must not act on a condition that
// could not be shown to hold, but the error is returned rather than
// swallowed, so [Engine.Process] reports it as a warning. A condition that
// silently never matches is the exact failure this key exists to remove.
func (e *Engine) matchesCondition(ctx context.Context, t *trigger, ent *entity.Entity) (bool, error) {
	src := t.on.Condition
	if src == "" {
		return true, nil
	}
	prog := t.condition[ent.Type]
	if err := t.conditionErr[ent.Type]; err != nil {
		return false, fmt.Errorf("condition %q: %w", src, err)
	}
	if prog == nil {
		// NewEngineFromMetamodel compiles every condition or fails; reaching
		// this means an engine was built without a metamodel. Fail closed,
		// and say so.
		return false, fmt.Errorf("condition %q: no metamodel", src)
	}
	ev := e.evaluator()
	var traversal predicate.TraversalFunc
	if len(prog.Traversals()) > 0 {
		if e.traversals == nil {
			return false, fmt.Errorf("condition %q: related(...) has no store to answer it", src)
		}
		if ent.Face != "" {
			// The store answers from the default face's edges; a named face
			// has its own, so the answer would be about another entity.
			return false, fmt.Errorf("condition %q: related(...) cannot be answered on face %q", src, ent.Face)
		}
		bound, err := e.traversals.Bind(ctx, ent.Type, []string{ent.ID}, prog)
		if err != nil {
			return false, fmt.Errorf("condition %q: %w", src, err)
		}
		traversal = bound(ent.ID)
	}
	matched, err := ev.MatchesWithTraversals(ctx, prog, ent.Type, ent.ID, ent.Properties, traversal)
	if err != nil {
		return false, fmt.Errorf("condition %q: %w", src, err)
	}
	return matched, nil
}

// render writes a fired trigger's outputs and checks into result.
func (e *Engine) render(ctx context.Context, t *trigger, event Event, result *Result) {
	skip := -1
	for _, o := range t.outputs {
		if o.action == skip {
			continue
		}
		if !e.renderOutput(o, t.name, event, result) {
			skip = o.action
		}
	}
	if event.Entity == nil {
		return
	}
	for _, c := range t.checks {
		e.renderCheck(ctx, c, event, result)
	}
}

// renderOutput renders one output. It returns false when a create failed,
// so the rest of its `do:` entry is skipped.
func (e *Engine) renderOutput(o output, name string, event Event, result *Result) bool {
	act := o.act
	switch o.kind {
	case outputSet:
		result.PropertiesSet[act.Set] = e.interpolate(act.Value, event)
	case outputRelate:
		targetID := e.interpolate(act.CreateRelation.To, event)
		if targetID != "" && event.Entity != nil {
			rel := entity.NewRelation(event.Entity.ID, act.CreateRelation.Relation, targetID)
			result.RelationsToCreate = append(result.RelationsToCreate,
				RelationToCreate{Relation: rel, AutomationName: name})
		}
	case outputPile:
		if event.Entity != nil {
			e.planPilePush(*act.AddToPile, event, result, name)
		}
	case outputNotify:
		if event.Entity != nil {
			e.planNotify(*act.Notify, event, result, name)
		}
	case outputCreate:
		return e.renderCreate(*act.CreateEntity, name, event, result)
	case outputScript:
		// Only safe values ({{today}}, {{user.name}}, ...) are interpolated
		// into code. Entity properties reach the script as Lua globals,
		// never as source.
		result.LuaToExecute = append(result.LuaToExecute, LuaToExecute{
			Code:           InterpolateSafeOnly(act.Lua, e.vars),
			AutomationName: name,
			AllowACLBypass: act.AllowACLBypass,
			Capabilities:   act.Capabilities,
		})
	case outputScriptFile:
		// The script package validates the path when it runs, as for every
		// other script path.
		result.LuaToExecute = append(result.LuaToExecute, LuaToExecute{
			FilePath:       act.LuaFile,
			AutomationName: name,
			AllowACLBypass: act.AllowACLBypass,
			Capabilities:   act.Capabilities,
			Background:     act.Background,
		})
	}
	return true
}

// renderCreate renders a create output and its relate.
func (e *Engine) renderCreate(ce CreateEntityAction, name string, event Event, result *Result) bool {
	if ce.Type == "" {
		result.Errors = append(result.Errors, "create_entity action requires 'type' field")
		return false
	}
	// The template name may interpolate entity values ({{new.kind}}), so the
	// result is checked against an allowlist before it names a file.
	template := e.interpolate(ce.Template, event)
	if !isValidTemplateName(template) {
		result.Errors = append(result.Errors,
			"create_entity template name contains invalid characters (only alphanumeric, hyphen, underscore allowed)")
		return false
	}
	props := make(map[string]any, len(ce.Properties))
	for k, v := range ce.Properties {
		props[k] = e.interpolate(v, event)
	}
	ifExists := ce.IfExists
	if ifExists == "" {
		ifExists = IfExistsSkip
	}
	result.EntitiesToCreate = append(result.EntitiesToCreate, EntityToCreate{
		Type:                ce.Type,
		Template:            template,
		Properties:          props,
		RelationFromTrigger: ce.Relation,
		IfExists:            ifExists,
		AutomationName:      name,
	})
	return true
}

// renderCheck evaluates a `validate:` check and adds its message when the
// entity fails it.
func (e *Engine) renderCheck(ctx context.Context, c check, event Event, result *Result) {
	if c.invalid {
		result.Warnings = append(result.Warnings, "Invalid validation check: "+c.Check)
		return
	}
	if e.matchClauses(ctx, c.clauses, event.Entity) {
		return
	}
	msg := e.interpolate(c.Message, event)
	if c.GetSeverity() == "error" {
		result.Errors = append(result.Errors, msg)
	} else {
		result.Warnings = append(result.Warnings, msg)
	}
}
