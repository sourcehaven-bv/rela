package appbuild

import (
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// grantEdgeVar is the record a role relation's `when:` names: the grant
// edge itself, with the relation type's declared properties.
const grantEdgeVar = "edge"

// grantConditionCompiler compiles a `role_relations:` `when:` against the
// relation type's declared properties (TKT-TKSCSBT). It accepts only what
// [acl.GrantCondition.Bind] lowers exactly into a store query:
//
//   - an AND of comparisons `edge.<prop> <op> <operand>`
//   - on a `date` property with the default format: ==, <, <=, >, >= against
//     today() or a date literal at UTC midnight (now() once TKT-08ZI5XN adds it)
//   - on a string or enum property: == against a non-empty literal
//
// Anything else fails the policy load. A datetime property is refused
// because a stored datetime keeps its offset, so its byte order is not its
// time order.
func grantConditionCompiler(meta *metamodel.Metamodel) acl.GrantConditionCompiler {
	return func(relType, source string) (*acl.GrantCondition, error) {
		def, ok := meta.GetRelationDef(relType)
		if !ok {
			return nil, fmt.Errorf("relation type %q is not declared in the schema", relType)
		}
		env := predicate.NewEnv()
		if err := env.DeclareVar(grantEdgeVar, relationRecordType(meta, def)); err != nil {
			return nil, err
		}
		if err := predicatefns.Declare(env); err != nil {
			return nil, err
		}
		prog, err := predicate.Compile(env, source)
		if err != nil {
			if kw := reservedEdgeProperty(def); kw != "" {
				return nil, fmt.Errorf("%w (property %q is a reserved word, so edge.%s cannot be written; "+
					"rename it, for example to valid_%s)", err, kw, kw, kw)
			}
			return nil, err
		}
		cmps, ok := prog.OrderedConjunction(grantEdgeVar, map[string]bool{"today": true, "now": true})
		if !ok {
			return nil, errors.New("must be an AND of comparisons `edge.<property> <op> <value>`, " +
				"with today() or a literal as the value")
		}
		cond := &acl.GrantCondition{Source: source}
		for _, c := range cmps {
			term, err := grantTerm(def, c)
			if err != nil {
				return nil, err
			}
			cond.Terms = append(cond.Terms, term)
		}
		return cond, nil
	}
}

// grantTerm maps one comparison to a term, checking it against the
// property's declared type.
func grantTerm(def *metamodel.RelationDef, c predicate.OrderedComparison) (acl.GrantTerm, error) {
	prop, ok := def.Properties[c.Attribute]
	if !ok {
		return acl.GrantTerm{}, fmt.Errorf("edge.%s is not a declared property of the relation", c.Attribute)
	}
	if prop.List {
		return acl.GrantTerm{}, fmt.Errorf("edge.%s is a list; a condition compares one value", c.Attribute)
	}
	term := acl.GrantTerm{Property: c.Attribute, Op: grantOps[c.Op]}
	switch prop.Type {
	case metamodel.PropertyTypeDate:
		if prop.Format != "" && prop.Format != metamodel.DefaultDateFormat {
			return acl.GrantTerm{}, fmt.Errorf("edge.%s has format %q; a condition needs the default %s",
				c.Attribute, prop.Format, metamodel.DefaultDateFormat)
		}
		term.Date = true
		switch c.Func {
		case "today":
			term.Operand = acl.OperandToday
		case "now":
			term.Operand = acl.OperandNow
		default:
			d, isDate := c.Literal.(predicate.Date)
			if !isDate {
				return acl.GrantTerm{}, fmt.Errorf("edge.%s compares with a non-date", c.Attribute)
			}
			lit := d.Time().UTC()
			if !lit.Equal(lit.Truncate(utcDay)) {
				return acl.GrantTerm{}, fmt.Errorf("edge.%s compares with %s, which has a time of day; "+
					"a date property compares with a date", c.Attribute, lit.Format(time.RFC3339))
			}
			term.Literal = lit.Format(time.DateOnly)
		}
	case metamodel.PropertyTypeDatetime:
		return acl.GrantTerm{}, fmt.Errorf("edge.%s is a datetime; declare it as a date "+
			"(a stored datetime keeps its offset, so the store cannot compare it exactly)", c.Attribute)
	default:
		s, isString := c.Literal.(predicate.String)
		if c.Op != "==" || !isString || s.String() == "" {
			return acl.GrantTerm{}, fmt.Errorf("edge.%s: only a date property may be ordered or "+
				"compared with today() or now(); others compare with == and a non-empty text", c.Attribute)
		}
		term.Literal = s.String()
	}
	if err := term.CheckShape(); err != nil {
		return acl.GrantTerm{}, err
	}
	return term, nil
}

// reservedEdgeProperty returns a declared relation property whose name is a
// reserved word of the condition language, or "" if there is none. Such a
// property cannot be named in a condition; `until` is the likely one.
func reservedEdgeProperty(def *metamodel.RelationDef) string {
	for _, kw := range []string{"until", "end", "do", "then", "repeat", "function", "local", "return", "in"} {
		if _, ok := def.Properties[kw]; ok {
			return kw
		}
	}
	return ""
}

// utcDay is the length of a UTC calendar day: a date literal must start one.
const utcDay = 24 * time.Hour

var grantOps = map[string]acl.GrantOp{
	"==": acl.GrantEq, ">=": acl.GrantGE, ">": acl.GrantGT, "<=": acl.GrantLE, "<": acl.GrantLT,
}

// relationRecordType is predicatefns.EntityRecordType for a relation's
// declared properties.
func relationRecordType(meta *metamodel.Metamodel, def *metamodel.RelationDef) predicate.RecordType {
	rt := make(predicate.RecordType, len(def.Properties))
	for name, prop := range def.Properties {
		p := prop
		scalar, ok := predicatefns.ScalarTypeForProp(meta, &p)
		if !ok {
			continue
		}
		if prop.List {
			rt[name] = predicate.ListType{Elem: scalar}
			continue
		}
		rt[name] = scalar
	}
	return rt
}
