// Package writerule compiles the validation rules declared `enforce: write`
// and checks an entity against them on the write path (TKT-5CESLHD).
//
// A write-enforced rule reads only the entity being written. Its conditions
// evaluate through internal/predicate, the same engine `rela validate` uses
// for the rule, so the write path and analyze agree on what the rule means.
// Lua never runs here, and nothing reads the graph: the metamodel loader
// refuses `lua:`, `lua_file:`, `relations:` and `content:` on an enforced
// rule, and [NewSet] refuses `related(...)`. A check is therefore a pure
// function of the entity, so the write path may run it on the exact state it
// is about to persist.
package writerule

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// Set is the compiled write-enforced rules of one metamodel. The zero value
// and nil both hold no rules, and [Set.Check] on them finds nothing.
type Set struct {
	ev     *predicatefns.Evaluator
	byType map[string][]compiledRule
}

// compiledRule is one rule compiled for one entity type. Each slice is
// ANDed; an empty when selects every entity of the type.
type compiledRule struct {
	name        string
	description string
	faces       []string
	when        []*predicate.Program
	then        []*predicate.Program
}

// Read is one rule's dependency on the properties of one entity type.
type Read struct {
	Type       string
	Rule       string
	Properties []string // sorted
}

// Option configures [NewSet].
type Option func(*setOptions)

type setOptions struct {
	now func() time.Time
}

// WithClock sets the clock behind today() in the rules' conditions. Without
// it the rules read the wall clock.
//
// Nil: ignored — the wall clock.
func WithClock(now func() time.Time) Option {
	return func(o *setOptions) { o.now = now }
}

// NewSet compiles every rule of meta declared `enforce: write`, for each
// entity type it applies to.
//
// It fails on the first condition that does not compile, or that uses
// `related(...)`. Failing is the safe direction: a rule that could not be
// compiled would otherwise be skipped, and a write it was declared to refuse
// would succeed.
//
// Nil: meta rejected — an error, because the caller is a wiring site.
func NewSet(meta *metamodel.Metamodel, opts ...Option) (*Set, error) {
	if meta == nil {
		return nil, errors.New("writerule.NewSet: nil metamodel")
	}
	var o setOptions
	for _, opt := range opts {
		opt(&o)
	}
	ev := predicatefns.NewEvaluator(meta)
	if o.now != nil {
		ev = predicatefns.NewEvaluatorWithClock(meta, o.now)
	}
	s := &Set{ev: ev, byType: map[string][]compiledRule{}}
	for _, rule := range meta.Validations {
		if !rule.IsWriteEnforced() {
			continue
		}
		types := rule.TargetTypes()
		if types == nil {
			types = meta.EntityTypes()
			sort.Strings(types)
		}
		for _, typ := range types {
			if _, ok := meta.GetEntityDef(typ); !ok {
				continue // reported by the metamodel loader
			}
			c, err := s.compileRule(rule, typ)
			if err != nil {
				return nil, fmt.Errorf("validation %q on %q: %w", rule.Name, typ, err)
			}
			s.byType[typ] = append(s.byType[typ], c)
		}
	}
	return s, nil
}

func (s *Set) compileRule(rule metamodel.ValidationRule, typ string) (compiledRule, error) {
	c := compiledRule{name: rule.Name, description: rule.Description, faces: rule.Faces}
	var err error
	if c.when, err = s.compileSide(typ, rule.When, rule.WhenCondition); err != nil {
		return c, fmt.Errorf("when: %w", err)
	}
	if c.then, err = s.compileSide(typ, rule.Then, rule.ThenCondition); err != nil {
		return c, fmt.Errorf("then: %w", err)
	}
	return c, nil
}

// compileSide compiles one side of a rule: the filter clauses and the
// expression, which the rule ANDs. A filter clause that does not transpile
// to the predicate engine is an error here, unlike in internal/validation,
// which falls back to the legacy filter matcher. The write path takes no
// fallback, so a rule means the same thing wherever it is enforced.
func (s *Set) compileSide(typ string, clauses []string, expr string) ([]*predicate.Program, error) {
	var progs []*predicate.Program
	if len(clauses) > 0 {
		prog, err := s.ev.CompileClauses(typ, clauses)
		if err != nil {
			return nil, err
		}
		progs = append(progs, prog)
	}
	if expr != "" {
		prog, err := s.ev.Compile(typ, expr)
		if err != nil {
			return nil, err
		}
		progs = append(progs, prog)
	}
	// A roll-up property needs no check here: the predicate environment does
	// not declare it, so a rule that reads one does not compile
	// (TestNewSet_RefusesRollup).
	for _, prog := range progs {
		if len(prog.Traversals()) > 0 {
			return nil, fmt.Errorf("%s(...) is not available on a write-enforced rule: "+
				"it reads only the entity being written", predicate.FuncRelated)
		}
	}
	return progs, nil
}

// Reads returns, for each compiled rule and entity type, the properties the
// rule reads. A caller that compares them with field-level ACL uses it.
func (s *Set) Reads() []Read {
	if s == nil {
		return nil
	}
	var out []Read
	for typ, rules := range s.byType {
		for _, r := range rules {
			seen := map[string]bool{}
			for _, prog := range slices.Concat(r.when, r.then) {
				for _, attr := range prog.Attributes(predicatefns.VarEntity) {
					seen[attr] = true
				}
			}
			props := make([]string, 0, len(seen))
			for p := range seen {
				props = append(props, p)
			}
			sort.Strings(props)
			out = append(out, Read{Type: typ, Rule: r.name, Properties: props})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// Check returns one error for each write-enforced rule e violates, in
// declaration order. A nil result means e may be written.
//
// A rule applies when its type and face scope cover e and every
// when-condition matches; it is violated when a then-condition does not hold
// or fails to evaluate.
//
// A when-condition that fails to evaluate fails closed, unlike in `rela
// validate`: the rule applies and is violated. The one exception is a
// condition whose properties are all unset on e, which leaves the rule out,
// so `amount > 10000` does not refuse a record without an amount. A value the
// type check turned into nil (a string on an integer property) is set, so it
// cannot switch the rule off.
//
// The message names the rule and quotes its description, both
// operator-written, and never a property value (see
// [metamodel.ValidationError]).
func (s *Set) Check(ctx context.Context, e *entity.Entity) []*metamodel.ValidationError {
	if s == nil || e == nil {
		return nil
	}
	rules := s.byType[e.Type]
	if len(rules) == 0 {
		return nil
	}
	var out []*metamodel.ValidationError
	for _, r := range rules {
		if len(r.faces) > 0 && !slices.Contains(r.faces, e.Face.String()) {
			continue
		}
		applies, evalErr := s.applies(ctx, r.when, e)
		if !applies {
			continue
		}
		if !evalErr && s.all(ctx, r.then, e) {
			continue
		}
		out = append(out, &metamodel.ValidationError{
			Type:    metamodel.ValidationErrorRule,
			Message: violationMessage(r),
		})
	}
	return out
}

// applies reports whether every when-program matches e. evalErr reports that
// a program failed to evaluate on a property e sets; the rule then applies
// and is violated. A program that fails with all its properties unset does
// not apply.
func (s *Set) applies(ctx context.Context, progs []*predicate.Program, e *entity.Entity) (applies, evalErr bool) {
	for _, prog := range progs {
		ok, err := s.ev.Matches(ctx, prog, e.Type, e.ID, e.Properties)
		if err != nil {
			if allUnset(prog, e) {
				return false, false
			}
			return true, true
		}
		if !ok {
			return false, false
		}
	}
	return true, false
}

// allUnset reports whether prog reads only properties e does not set. A
// program that uses the entity whole depends on more than its attribute list,
// so it never counts as unset.
func allUnset(prog *predicate.Program, e *entity.Entity) bool {
	if prog.UsesWhole(predicatefns.VarEntity) {
		return false
	}
	for _, attr := range prog.Attributes(predicatefns.VarEntity) {
		if v, ok := e.Properties[attr]; ok && !metamodel.IsEmptyValue(v) {
			return false
		}
	}
	return true
}

// all reports whether every program matches e. An evaluation error counts
// as no match.
func (s *Set) all(ctx context.Context, progs []*predicate.Program, e *entity.Entity) bool {
	for _, prog := range progs {
		ok, err := s.ev.Matches(ctx, prog, e.Type, e.ID, e.Properties)
		if err != nil || !ok {
			return false
		}
	}
	return true
}

func violationMessage(r compiledRule) string {
	if r.description == "" {
		return fmt.Sprintf("validation rule %q is not satisfied", r.name)
	}
	return fmt.Sprintf("validation rule %q is not satisfied: %s", r.name, r.description)
}
