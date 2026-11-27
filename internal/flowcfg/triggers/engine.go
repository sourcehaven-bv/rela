package triggers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TraversalBinder answers the `related(...)` traversals of a condition for
// the triggering entity (TKT-205V2N). The wiring site supplies one bound to
// the raw store: an automation is system policy, so what it sees must not
// depend on who made the write.
type TraversalBinder interface {
	Bind(ctx context.Context, entityType string, ids []string, progs ...*predicate.Program) (
		func(rowID string) predicate.TraversalFunc, error)
}

// Option configures an engine built by [New].
type Option func(*Engine)

// WithTraversals lets conditions use `related(...)`. Without it such a
// condition is a load error.
func WithTraversals(b TraversalBinder) Option {
	return func(e *Engine) { e.traversals = b }
}

// WithClock sets the clock behind `{{now}}` and `{{today}}` in templates and
// `today()` in conditions. Without it the engine reads the wall clock.
//
// Nil: ignored — the wall clock.
func WithClock(now func() time.Time) Option {
	return func(e *Engine) {
		if now == nil {
			return
		}
		e.vars.Now = now
		e.now = now
	}
}

// Engine evaluates automations against entity events.
type Engine struct {
	automations []Rule
	// triggers are the automations compiled onto flow triggers, in
	// declaration order; byEvent indexes them by the event types that may
	// fire them. Both are built by the constructor and only read after.
	triggers []trigger
	byEvent  map[EventType][]int
	vars     TemplateVars
	// meta is the active metamodel, used to evaluate `when:` and
	// `validate:` comparisons against the property's declared type
	// (so `count>9` compares numerically, not lexicographically).
	// Optional: nil falls back to string-only matching, preserving the
	// pre-metamodel behavior for engines constructed via [NewEngine].
	meta *metamodel.Metamodel

	// ev is the predicate evaluator for typed `when:`/`validate:`
	// comparisons — the condition engine (TKT-J4IR1G). Built lazily on
	// first typed match against the current meta; rebuilt if meta changes.
	// Guarded by evMu.
	evMu sync.Mutex
	ev   *predicatefns.Evaluator

	// traversals answers `related(...)` in conditions; nil refuses them.
	traversals TraversalBinder

	// now is the clock behind today() in conditions; see [WithClock].
	//
	// Nil: accepted — the wall clock.
	now func() time.Time
}

// evaluator returns the predicate Evaluator bound to the current
// metamodel, building it once. Returns nil when there is no metamodel
// (the string-only fallback path handles that).
func (e *Engine) evaluator() *predicatefns.Evaluator {
	if e.meta == nil {
		return nil
	}
	e.evMu.Lock()
	defer e.evMu.Unlock()
	if e.ev == nil {
		if e.now != nil {
			e.ev = predicatefns.NewEvaluatorWithClock(e.meta, e.now)
		} else {
			e.ev = predicatefns.NewEvaluator(e.meta)
		}
	}
	return e.ev
}

// NewEngine creates an engine for rules without a metamodel, so `where:`
// clauses and checks fall back to string matching, and a rule with a
// `when:` predicate never fires. Use [New] for type-aware comparison.
func NewEngine(automations []Rule) *Engine {
	e := newEngine(automations)
	e.compile()
	return e
}

// newEngine returns an engine whose trigger table is not compiled yet.
//
// The engine keeps its own copy of the rules. The wiring site parses the
// rules once and builds an engine per tenant from them, and the load writes
// resolved notify paths into the rules. Shared rules would be a data race.
func newEngine(automations []Rule) *Engine {
	return &Engine{
		automations: cloneRules(automations),
		vars:        DefaultTemplateVars(),
	}
}

// cloneRules returns a copy of rules that shares nothing the engine may
// write. Each rule gets its own slices, and each action its own copy of the
// structs it points to. The filters under `where:` are shared, because
// nothing writes to a parsed filter.
func cloneRules(rules []Rule) []Rule {
	if rules == nil {
		return nil
	}
	out := make([]Rule, len(rules))
	for i, r := range rules {
		r.On.Entity = slices.Clone(r.On.Entity)
		r.On.Faces = slices.Clone(r.On.Faces)
		r.On.Copy = slices.Clone(r.On.Copy)
		r.On.When = slices.Clone(r.On.When)
		r.Validate = slices.Clone(r.Validate)
		r.Do = slices.Clone(r.Do)
		for j := range r.Do {
			unshareAction(&r.Do[j])
		}
		out[i] = r
	}
	return out
}

// unshareAction replaces what a points to with copies, so a shares nothing
// with the action it was copied from.
func unshareAction(a *Action) {
	if a.CreateRelation != nil {
		cr := *a.CreateRelation
		a.CreateRelation = &cr
	}
	if a.CreateEntity != nil {
		ce := *a.CreateEntity
		ce.Properties = maps.Clone(ce.Properties)
		a.CreateEntity = &ce
	}
	if a.AddToPile != nil {
		p := *a.AddToPile
		a.AddToPile = &p
	}
	if a.Notify != nil {
		n := *a.Notify
		n.Via = slices.Clone(n.Via)
		n.hops = nil
		a.Notify = &n
	}
	a.Capabilities.Secrets = slices.Clone(a.Capabilities.Secrets)
	a.Capabilities.Tokens = slices.Clone(a.Capabilities.Tokens)
}

// New creates an engine for rules decoded by [Parse], wiring the metamodel
// so `where:` clauses and checks compare against each property's declared
// type.
//
// It returns an error when a rule cannot be honored as written: a `when:`
// predicate that does not compile, a write to a file property, or a notify
// path that does not resolve. Failing the load is the safe direction,
// because a dropped constraint fires the rule on more entities than the
// operator asked for.
//
// Nil: meta is rejected — conditions and notify paths need the schema.
func New(meta *metamodel.Metamodel, rules []Rule, opts ...Option) (*Engine, error) {
	if meta == nil {
		return nil, errors.New("triggers: New requires a metamodel")
	}
	e := newEngine(rules)
	e.meta = meta
	for _, opt := range opts {
		opt(e)
	}
	if err := e.compileConditions(); err != nil {
		return nil, err
	}
	if err := rejectFilePropertyWrites(meta, e.automations); err != nil {
		return nil, err
	}
	if err := resolveNotifyPaths(meta, e.automations); err != nil {
		return nil, err
	}
	e.compile()
	return e, nil
}

// rejectFilePropertyWrites refuses an automation whose `set:` or
// `create_entity` writes a `file` property. A file value is the capability
// to download the bytes it names, and only the attachment paths may change
// one (BUG-CTUW2N). The manager checks caller-authored values, but applies
// automation values after that check, so an automation writing one would
// bypass the rule; it is refused here, at load. A `set:` on an automation
// with no `entity:` is checked against every type, since it may fire on
// any of them.
func rejectFilePropertyWrites(meta *metamodel.Metamodel, automations []Rule) error {
	if meta == nil {
		return nil
	}
	for _, auto := range automations {
		types := auto.On.Entity
		if len(types) == 0 {
			types = slices.Sorted(maps.Keys(meta.Entities))
		}
		for _, act := range auto.Do {
			if act.Set != "" {
				for _, t := range types {
					if slices.Contains(metamodel.FileProperties(meta, t), act.Set) {
						return fmt.Errorf(
							"trigger %q: `set` writes %q, a file property of %q; file values change only through the attachments API",
							auto.Name, act.Set, t)
					}
				}
			}
			if ce := act.CreateEntity; ce != nil {
				for _, prop := range metamodel.FileProperties(meta, ce.Type) {
					if _, ok := ce.Properties[prop]; ok {
						return fmt.Errorf(
							"trigger %q: `create` sets %q, a file property of %q; file values change only through the attachments API",
							auto.Name, prop, ce.Type)
					}
				}
			}
		}
	}
	return nil
}

// compileConditions compiles every `condition:` expression up front, so a
// syntax error or an unknown property surfaces at load rather than on the
// first matching write. Compiled programs are cached inside the
// Evaluator, keyed by (entityType, source), so this also warms it.
//
// A condition needs a metamodel to build its typed env. An engine
// constructed without one (NewEngine, used by tests and the memory
// backend) therefore cannot honor a condition at all — that is an
// error, not a silent no-op, for the same widening reason.
func (e *Engine) compileConditions() error {
	for _, auto := range e.automations {
		src := auto.On.Condition
		if src == "" {
			continue
		}
		if e.meta == nil {
			return fmt.Errorf(
				"trigger %q: `when` %q requires a metamodel", auto.Name, src)
		}
		types := auto.On.Entity
		if len(types) == 0 {
			// A condition references entity properties, so it must be
			// compiled against a concrete type's env. Without `entity:`
			// there is nothing to compile against.
			return fmt.Errorf(
				"trigger %q: `when` requires `on.entity` naming the type(s) it applies to",
				auto.Name)
		}
		ev := e.evaluator()
		for _, t := range types {
			prog, err := ev.Compile(t, src)
			if err != nil {
				return fmt.Errorf("trigger %q: `when` %q on %q: %w",
					auto.Name, src, t, err)
			}
			if auto.On.Deleted && len(prog.Traversals()) > 0 {
				// The delete removed the entity's relations with it.
				return fmt.Errorf("trigger %q: `when` %q: %s(...) cannot be used with `on.deleted`; "+
					"the entity's relations are deleted with it", auto.Name, src, predicate.FuncRelated)
			}
			if auto.On.Created && len(prog.Traversals()) > 0 {
				// The entity is persisted before any of its relations, so on
				// create `related(...)` is always false and `not related` is
				// always true: a condition that looks meaningful and is not.
				return fmt.Errorf("trigger %q: `when` %q: %s(...) cannot be used with `on.created`; "+
					"the entity's relations are written after it", auto.Name, src, predicate.FuncRelated)
			}
			if err := e.checkTraversals(t, prog); err != nil {
				return fmt.Errorf("trigger %q: `when` %q on %q: %w", auto.Name, src, t, err)
			}
		}
	}
	return nil
}

// checkTraversals refuses a `related(...)` the engine cannot answer: one
// that does not resolve against the metamodel, or any at all when no binder
// is wired.
func (e *Engine) checkTraversals(entityType string, prog *predicate.Program) error {
	if len(prog.Traversals()) == 0 {
		return nil
	}
	if e.traversals == nil {
		return fmt.Errorf("%s(...) is not available: no store is wired to answer it", predicate.FuncRelated)
	}
	return predicatefns.ValidateTraversals(e.meta, entityType, prog)
}

// SetTemplateVars sets the template variables for interpolation.
func (e *Engine) SetTemplateVars(vars TemplateVars) {
	e.vars = vars
}

// Process evaluates all automations against an event and returns the result.
//
// The event selects its triggers through the event index, the scope
// (managed name, `entity:`, `faces:`, `copy:`) and the event keys; the
// selected triggers fire through [flow.Fire], and each fired trigger
// renders its outputs in declaration order. A guard that fails to evaluate
// adds a warning in its place and fires nothing.
func (e *Engine) Process(ctx context.Context, event Event) *Result {
	event.currentUserID = currentUserEntityID(ctx)
	result := &Result{
		PropertiesSet:     make(map[string]string),
		RelationsToCreate: make([]RelationToCreate, 0),
		EntitiesToCreate:  make([]EntityToCreate, 0),
		LuaToExecute:      make([]LuaToExecute, 0),
		PilesToPush:       make([]PileToPush, 0),
		Notifications:     make([]NotificationToSend, 0),
		Warnings:          make([]string, 0),
		Errors:            make([]string, 0),
	}

	var selected []*trigger
	for _, i := range e.byEvent[event.Type] {
		t := &e.triggers[i]
		if e.admits(t.on, event) && e.selects(t.on, event) {
			selected = append(selected, t)
		}
	}
	fired := flow.Fire(selected, func(t *trigger) (bool, error) { return e.holds(ctx, t, event.Entity) })
	for _, f := range fired {
		if f.Err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("trigger %q: could not be evaluated: %v", f.Trigger.name, f.Err))
			continue
		}
		e.render(ctx, f.Trigger, event, result)
	}
	return result
}

// WatchesRelation reports whether a trigger may fire on event ev for a
// relation of type relType. A write that raises no event a trigger watches
// can skip the work of raising it. A managed relation type is never watched.
func (e *Engine) WatchesRelation(ev EventType, relType string) bool {
	if e == nil || entity.IsManagedName(relType) {
		return false
	}
	for _, i := range e.byEvent[ev] {
		on := e.triggers[i].on
		if (ev == EventRelationCreated && on.RelationCreated == relType) ||
			(ev == EventRelationRemoved && on.RelationRemoved == relType) {

			return true
		}
	}
	return false
}

// admits applies a trigger's scope to the event.
func (e *Engine) admits(trigger Trigger, event Event) bool {
	// A rela-managed record (a flow run, its `_carries` edge) is written by
	// the flow engine only; no operator trigger fires on it, not even a
	// type-less one (TKT-BC5G8RP). [Check] refuses a trigger that names one,
	// so only the type-less case reaches here.
	if (event.Entity != nil && entity.IsManagedName(event.Entity.Type)) ||
		(event.Relation != nil && entity.IsManagedName(event.Relation.Type)) {

		return false
	}
	// The type scope is an exact match on the type as written.
	if len(trigger.Entity) > 0 && event.Entity != nil && !slices.Contains(trigger.Entity, event.Entity.Type) {
		return false
	}
	// A trigger scoped away from this row does not apply, which is a
	// different thing from a guard that does not hold.
	if !e.matchesFace(trigger, event.Entity) {
		return false
	}
	return e.copyScopeAdmits(trigger, event)
}

// selects reports whether the event's keys fire the trigger: the event
// type, and for an update the changed property or body, for a relation
// event its type.
func (e *Engine) selects(trigger Trigger, event Event) bool {
	if len(trigger.Copy) > 0 && event.Type != EventEntityRenamed && copyOnly(trigger) {
		// Alone, `on.copy:` fires on every run of the copy, changed or not.
		return event.Type == EventEntityCreated || event.Type == EventEntityUpdated
	}

	switch event.Type {
	case EventEntityCreated:
		return trigger.Created

	case EventEntityUpdated:
		if trigger.Updated {
			return entityChanged(event)
		}
		if trigger.Property == "" {
			return false
		}
		return e.matchesPropertyChange(trigger, event)

	case EventEntityRenamed:
		// A pending job names the old id and finds nothing, so a rename
		// re-runs every entity trigger that could have scheduled one.
		return trigger.Created || trigger.Updated || trigger.Property != "" || len(trigger.Copy) > 0

	case EventEntityDeleted:
		return trigger.Deleted

	case EventRelationCreated:
		if trigger.RelationCreated == "" {
			return false
		}
		return event.Relation != nil && event.Relation.Type == trigger.RelationCreated

	case EventRelationRemoved:
		if trigger.RelationRemoved == "" {
			return false
		}
		return event.Relation != nil && event.Relation.Type == trigger.RelationRemoved
	}

	return false
}

// copyScopeAdmits applies the trigger's `on.copy:` scope: an entity event
// must come from one of the named copies. A rename carries no copy, and
// re-runs a copy trigger only on a face one of its copies writes, so the
// re-run cannot reach a face the copy's guard never admitted. Without a
// metamodel the targets are unknown, so a rename re-runs no copy trigger.
func (e *Engine) copyScopeAdmits(trigger Trigger, event Event) bool {
	if len(trigger.Copy) == 0 {
		return true
	}
	if event.Type != EventEntityRenamed {
		return slices.Contains(trigger.Copy, event.Copy)
	}
	if e.meta == nil || event.Entity == nil {
		return false
	}
	for _, name := range trigger.Copy {
		to, err := metamodel.ParseCopyTarget(e.meta.Copies[name].To)
		if err == nil && to.Type == event.Entity.Type && to.Face == string(event.Entity.Face) {
			return true
		}
	}
	return false
}

// copyOnly reports whether `on.copy:` is the trigger's only event key.
func copyOnly(trigger Trigger) bool {
	return !trigger.Created && !trigger.Updated && trigger.Property == "" &&
		trigger.RelationCreated == "" && trigger.RelationRemoved == ""
}

// matchesPropertyChange checks if a property change event matches the trigger.
// entityChanged reports whether an update changed a property or the body.
// An update without the previous state counts as a change.
func entityChanged(event Event) bool {
	if event.Entity == nil {
		return false
	}
	if event.OldEntity == nil {
		return true
	}
	// The canonical hash normalizes value types, so a script writing back a
	// float64 where the store held an int is not a change.
	return canonical.HashEntity(*event.Entity) != canonical.HashEntity(*event.OldEntity)
}

func (e *Engine) matchesPropertyChange(trigger Trigger, event Event) bool {
	if event.Entity == nil {
		return false
	}

	newValue := event.Entity.GetString(trigger.Property)
	oldValue := ""
	if event.OldEntity != nil {
		oldValue = event.OldEntity.GetString(trigger.Property)
	}

	// No change occurred
	if newValue == oldValue {
		return false
	}

	// Check "from" constraint
	if trigger.From != "" && oldValue != trigger.From {
		return false
	}

	// Check "becomes" constraint
	if trigger.Becomes != "" && newValue != trigger.Becomes {
		return false
	}

	return true
}

// planPilePush appends an add_to_pile action to result.PilesToPush with its
// pile and owner interpolated.
//
// An owner template that interpolates to nothing ({{new.assignee}} on an
// unassigned entity) skips the push. An empty owner means the acting user,
// so pushing anyway would put the entity on the pile of whoever edited it,
// not on nobody's. A pile name that interpolates to nothing is a warning.
func (e *Engine) planPilePush(action AddToPileAction, event Event, result *Result, automationName string) {
	owner := strings.TrimSpace(e.interpolate(action.Owner, event))
	if action.Owner != "" && owner == "" {
		return
	}
	pile := strings.TrimSpace(e.interpolate(action.Pile, event))
	if pile == "" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("trigger %q: add_to_pile: the pile name is empty", automationName))
		return
	}
	result.PilesToPush = append(result.PilesToPush, PileToPush{
		Pile:           pile,
		Owner:          owner,
		Create:         action.Create,
		AutomationName: automationName,
	})
}

// matchSimple does simple value matching without metamodel context.
// Handles the most common automation validation cases.
func matchSimple(val any, f *filter.Filter) bool {
	// Handle nil/missing/empty values
	if val == nil || val == "" {
		// Only match if explicitly comparing to empty with = operator
		if f.Operator == filter.OpEqual && f.Value == "" {
			return true
		}
		// For "!=" with empty value, missing/nil means "is empty", so it should NOT match "is not empty"
		return false
	}

	// Use the filter package's MatchValue for the actual comparison
	return filter.MatchValue(val, f)
}

// interpolate replaces template variables in a string.
func (e *Engine) interpolate(template string, event Event) string {
	vars := e.vars
	vars.CurrentUserID = event.currentUserID
	vars.currentUser = true
	return Interpolate(template, vars, event.Entity, event.OldEntity)
}

// lacksCurrentUser reports whether template reads {{current_user.id}} while
// the event has no resolved user. Such a `set` or `create.properties` value
// is left out: writing "" would erase a stored value on every write from
// the CLI, a job or a flow.
func lacksCurrentUser(template string, event Event) bool {
	return event.currentUserID == "" && strings.Contains(template, "{{"+PlaceholderCurrentUserID+"}}")
}

// PlaceholderCurrentUserID names the acting user's entity in a template,
// written as {{current_user.id}}.
const PlaceholderCurrentUserID = "current_user.id"

// currentUserEntityID returns the user entity the principal on ctx resolved
// to, for {{current_user.id}}. The principal counts only when the ACL's
// principal_property lookup substituted its User with an entity id, which
// RawUser records (the rule appbuild.queryIdentityFor follows). It reads the
// principal, the identity the audit records, and not a stamped query
// identity, so the two cannot disagree. A raw identity (the CLI, a job, a
// flow's run_as, an unmatched principal) is not an entity id, so it yields
// "" rather than pointing at an entity whose id happens to match.
func currentUserEntityID(ctx context.Context) string {
	p, ok := principal.Stamped(ctx)
	if !ok || p.RawUser == "" || p.RawUser == p.User {
		return ""
	}
	return p.User
}

// isValidTemplateName validates that a template name contains only safe identifier characters.
// Uses an allowlist approach: only alphanumeric, hyphen, and underscore are allowed.
// Empty template names are valid (means use default template).
func isValidTemplateName(name string) bool {
	if name == "" {
		return true
	}
	// Allowlist approach: only allow identifier-like characters.
	// This is safer than blocklisting dangerous patterns (path separators, .., null bytes, etc.)
	for _, ch := range name {
		isLower := ch >= 'a' && ch <= 'z'
		isUpper := ch >= 'A' && ch <= 'Z'
		isDigit := ch >= '0' && ch <= '9'
		isAllowed := isLower || isUpper || isDigit || ch == '-' || ch == '_'
		if !isAllowed {
			return false
		}
	}
	return true
}

// matchesFace reports whether a trigger's `faces:` scope covers the row the
// event is about.
//
// An empty scope means every state — see Trigger.Faces for why that is the
// default. The comparison is against the DECLARED face name, because the bare
// face is stored as the empty coordinate and an operator writes `faces: [en]`,
// not `faces: [""]`.
func (e *Engine) matchesFace(trigger Trigger, ent *entity.Entity) bool {
	if len(trigger.Faces) == 0 {
		return true
	}
	if ent == nil {
		// A relation event with no entity in hand cannot be face-scoped; a
		// scoped trigger simply does not apply to it.
		return false
	}
	return slices.Contains(trigger.Faces, ent.Face.String())
}
