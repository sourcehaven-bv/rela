package acl

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// GrantCondition is the compiled `when:` of a `role_relations:` entry
// (TKT-TKSCSBT): an AND of terms on the grant edge's own properties. An edge
// of that relation type confers its role, or counts as membership, only while
// every term holds.
//
// internal/acl cannot compile the `when:` string itself: that needs the
// predicate engine and the schema, which this package may not import. The
// wiring site compiles it ([CompileGrantConditions] takes the
// compiler) and [NewDeclarative] refuses a `when:` that was not compiled.
//
// A condition is bound to one read time per request ([GrantCondition.Bind]).
// Nothing caches a bound condition across requests, so access through an
// expired edge ends at the first request after the expiry. No job runs.
type GrantCondition struct {
	// Source is the `when:` text, for the audit report and errors.
	Source string
	Terms  []GrantTerm
}

// GrantOp is the comparison of one [GrantTerm].
type GrantOp int

// The comparisons a term may make. The property is on the left.
const (
	GrantEq GrantOp = iota + 1
	GrantGE
	GrantGT
	GrantLE
	GrantLT
)

// GrantOperand says what a [GrantTerm] compares the property with.
type GrantOperand int

const (
	// OperandLiteral compares with [GrantTerm.Literal].
	OperandLiteral GrantOperand = iota
	// OperandToday compares with the UTC day of the request's read time.
	OperandToday
)

// GrantTerm is one comparison `edge.<Property> <Op> <operand>`.
//
// A Date term reads a `date` property (YYYY-MM-DD) and may use every
// operator. A term that is not Date is a string or enum equality with a
// literal; the compiler refuses anything else.
type GrantTerm struct {
	Property string
	Op       GrantOp
	Operand  GrantOperand
	// Literal is the compared value for [OperandLiteral]: a YYYY-MM-DD date
	// when Date is set, else a non-empty string.
	Literal string
	Date    bool
}

const dateLayout = "2006-01-02"

// CheckShape reports a term the binder cannot lower. The compiler only emits
// valid terms; this guards a hand-built condition.
func (t GrantTerm) CheckShape() error {
	if t.Property == "" {
		return errors.New("grant condition: term names no property")
	}
	if t.Op < GrantEq || t.Op > GrantLT {
		return fmt.Errorf("grant condition: %s: unknown operator %d", t.Property, t.Op)
	}
	switch {
	case t.Date && t.Operand == OperandLiteral:
		if _, err := time.Parse(dateLayout, t.Literal); err != nil {
			return fmt.Errorf("grant condition: %s: literal %q is not a date", t.Property, t.Literal)
		}
	case t.Date:
		if t.Operand != OperandToday {
			return fmt.Errorf("grant condition: %s: unknown operand %d", t.Property, t.Operand)
		}
	default:
		if t.Op != GrantEq || t.Operand != OperandLiteral || t.Literal == "" {
			return fmt.Errorf("grant condition: %s: only a date may be ordered or compared "+
				"with today(); a string compares with == and a non-empty literal", t.Property)
		}
	}
	return nil
}

// Bind lowers the condition at the read time at into the store predicates an
// edge's properties must match. ok is false when no edge can match: a term
// reads today() and at is zero (no read time was captured, so the grant fails
// closed).
//
// The predicates compare byte-wise, which is time order only for a
// YYYY-MM-DD value. So every property a Date term reads also gets a
// [store.PropIsDate] guard: an absent, empty, list or malformed value never
// matches, in any direction.
//
// today() is the UTC day of at. Against a day D, d > D lowers to d >= D+1 and
// d < D to d <= D-1.
func (c *GrantCondition) Bind(at time.Time) (preds []store.PropPredicate, ok bool) {
	if c == nil {
		return nil, true
	}
	guarded := map[string]bool{}
	for _, t := range c.Terms {
		if t.CheckShape() != nil {
			return nil, false
		}
		if !t.Date {
			preds = append(preds, store.PropPredicate{
				Property: t.Property, Op: store.PropEqual, Value: t.Literal, Scalar: true,
			})
			continue
		}
		var day time.Time
		switch t.Operand {
		case OperandLiteral:
			day, _ = time.Parse(dateLayout, t.Literal)
		case OperandToday:
			if at.IsZero() {
				return nil, false
			}
			day = at.UTC().Truncate(utcDay)
		}
		next := day.AddDate(0, 0, 1).Format(dateLayout)
		prev := day.AddDate(0, 0, -1).Format(dateLayout)
		base := day.Format(dateLayout)
		var cmp []store.PropPredicate
		ge := func(v string) store.PropPredicate {
			return store.PropPredicate{Property: t.Property, Op: store.PropGreaterEqual, Value: v}
		}
		le := func(v string) store.PropPredicate {
			return store.PropPredicate{Property: t.Property, Op: store.PropLessEqual, Value: v}
		}
		switch t.Op {
		case GrantGE:
			cmp = []store.PropPredicate{ge(base)}
		case GrantGT:
			cmp = []store.PropPredicate{ge(next)}
		case GrantLE:
			cmp = []store.PropPredicate{le(base)}
		case GrantLT:
			cmp = []store.PropPredicate{le(prev)}
		case GrantEq:
			cmp = []store.PropPredicate{ge(base), le(base)}
		}
		if !guarded[t.Property] {
			guarded[t.Property] = true
			preds = append(preds, store.PropPredicate{Property: t.Property, Op: store.PropIsDate})
		}
		preds = append(preds, cmp...)
	}
	return preds, true
}

// Properties returns the edge properties the condition reads, sorted.
func (c *GrantCondition) Properties() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, t := range c.Terms {
		if !slices.Contains(out, t.Property) {
			out = append(out, t.Property)
		}
	}
	sort.Strings(out)
	return out
}

// GrantConditionCompiler compiles the `when:` of the role relation relType.
// The wiring site supplies it, because compiling needs the predicate engine
// and the schema.
type GrantConditionCompiler func(relType, source string) (*GrantCondition, error)

// CompileGrantConditions compiles every `when:` in p with compile and
// stores the result on an entry that has none yet. A `when:` that does not
// compile fails the load. It must run before [NewDeclarative], which refuses a
// `when:` that has no compiled condition.
//
// It compiles every entry, so a caller can validate a policy against another
// schema (the Configure preview). It writes only an entry with no condition:
// a policy a Declarative holds has one on every entry, so validating it again
// writes nothing while requests read it.
//
// Nil: compile is rejected when the policy declares any `when:`.
func CompileGrantConditions(p *Policy, compile GrantConditionCompiler) error {
	for _, relType := range slices.Sorted(maps.Keys(p.RoleRelations)) {
		def := p.RoleRelations[relType]
		if def.When == "" {
			continue
		}
		if compile == nil {
			return fmt.Errorf("role_relations: %s: `when:` needs a condition compiler", relType)
		}
		cond, err := compile(relType, def.When)
		if err != nil {
			return fmt.Errorf("role_relations: %s: when: %w", relType, err)
		}
		if cond == nil || len(cond.Terms) == 0 {
			return fmt.Errorf("role_relations: %s: when: compiled to no condition", relType)
		}
		for _, t := range cond.Terms {
			if err := t.CheckShape(); err != nil {
				return fmt.Errorf("role_relations: %s: when: %w", relType, err)
			}
		}
		if def.Condition != nil {
			continue
		}
		cond.Source = def.When
		def.Condition = cond
		p.RoleRelations[relType] = def
	}
	return nil
}

// checkGrantConditions is the [NewDeclarative] check: every `when:` has a
// compiled condition, and every conditional relation is write-gated.
func checkGrantConditions(p *Policy) error {
	for _, relType := range slices.Sorted(maps.Keys(p.RoleRelations)) {
		def := p.RoleRelations[relType]
		if def.When == "" && def.Condition == nil {
			continue
		}
		if def.When != "" && def.Condition == nil {
			return fmt.Errorf("role_relations: %s: `when:` was not compiled "+
				"(CompileGrantConditions must run before NewDeclarative)", relType)
		}
		if def.RequiresPermission == "" {
			return fmt.Errorf("role_relations: %s: a conditional grant needs requires_permission", relType)
		}
	}
	return nil
}

// utcDay is the length of a UTC calendar day, the unit a condition is judged in.
const utcDay = 24 * time.Hour

// grantConds is the conditions of one request, bound to its read time once
// at [Declarative.ForIdentity]. A relation type absent from the map is
// unconditional.
type grantConds map[string]boundCond

// boundCond is one bound condition. ok false means no edge of the type
// matches for this request.
type boundCond struct {
	preds []store.PropPredicate
	ok    bool
}

// bindGrantConds binds every conditional relation of p at at. A relation
// with a `when:` but no compiled condition is bound to "no edge counts":
// [NewDeclarative] refuses that policy, and this keeps a later mistake from
// turning the relation unconditional.
func bindGrantConds(p *Policy, at time.Time) grantConds {
	var out grantConds
	for relType, def := range p.RoleRelations {
		if def.Condition == nil && def.When == "" {
			continue
		}
		if out == nil {
			out = grantConds{}
		}
		if def.Condition == nil {
			out[relType] = boundCond{}
			continue
		}
		preds, ok := def.Condition.Bind(at)
		out[relType] = boundCond{preds: preds, ok: ok}
	}
	return out
}

// edge returns the edge predicates for relType and whether an edge of the
// type can count at all. Unconditional types return (nil, true).
func (g grantConds) edge(relType string) ([]store.PropPredicate, bool) {
	c, conditional := g[relType]
	if !conditional {
		return nil, true
	}
	return c.preds, c.ok
}

// conditional reports whether relType carries a condition.
func (g grantConds) conditional(relType string) bool {
	_, ok := g[relType]
	return ok
}
