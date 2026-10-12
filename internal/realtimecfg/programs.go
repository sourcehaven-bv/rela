package realtimecfg

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// ruleFunctions are the only host functions a trigger or auto rule may
// call. Each is a pure function of its arguments and the date. The list is
// an allowlist rather than an analysis: a rule is evaluated without a reader
// and its outcome becomes a message every channel reader sees, so anything
// that could read beyond the named properties is refused outright.
var ruleFunctions = []string{
	predicatefns.FuncContains,
	predicatefns.FuncDateAdd,
	predicatefns.FuncDaysBetween,
	predicatefns.FuncLen,
	predicatefns.FuncToday,
}

// entityEnv declares `entity` as entityType's record and the standard
// library.
func entityEnv(meta *metamodel.Metamodel, entityType string) (*predicate.Env, error) {
	def, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil, fmt.Errorf("unknown entity type %q", entityType)
	}
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicatefns.EntityRecordType(meta, def)); err != nil {
		return nil, err
	}
	if err := predicatefns.Declare(env); err != nil {
		return nil, err
	}
	return env, nil
}

// messageEnv declares `message` as the record of a message type's
// properties and the standard library.
func messageEnv(props map[string]Property) (*predicate.Env, error) {
	rec := predicate.RecordType{}
	for name, p := range props {
		switch p.Type {
		case PropertyInteger:
			rec[name] = predicate.IntType
		case PropertyBoolean:
			rec[name] = predicate.BoolType
		case PropertyDate:
			rec[name] = predicate.DateType
		case PropertyDatetime:
			rec[name] = predicate.DateTypeWithLayout(metamodel.DefaultDatetimeFormat)
		default:
			rec[name] = predicate.StringType
		}
	}
	env := predicate.NewEnv()
	if err := env.DeclareVar("message", rec); err != nil {
		return nil, err
	}
	if err := predicatefns.Declare(env); err != nil {
		return nil, err
	}
	return env, nil
}

// compileRule compiles a trigger `when:`, an `auto.when:` or a lifecycle
// `condition:` for entityType and holds it to the allowlist. A condition is
// held to it too: whether an operation is allowed or denied is a one-bit
// answer about the entity, so a condition on a hidden property is an oracle
// on its value. The allowlist is: no traversal, no function outside
// [ruleFunctions], `entity` only as entity.<property>, and no property a
// `visible:` grant restricts. ok is false when a problem was reported.
func (v *validator) compileRule(key, entityType, src string) (Rule, bool) {
	env, err := entityEnv(v.meta, entityType)
	if err != nil {
		v.add(key, err.Error())
		return Rule{}, false
	}
	prog, err := predicate.Compile(env, src)
	if err != nil {
		v.add(key, fmt.Sprintf("does not compile for %s: %v", entityType, err))
		return Rule{}, false
	}
	ok := true
	if len(prog.Traversals()) > 0 {
		v.add(key, "may not follow relations (related); a rule reads only the entity's own properties")
		ok = false
	}
	for _, fn := range prog.Functions() {
		if !slices.Contains(ruleFunctions, fn) {
			v.add(key, fmt.Sprintf("may not call %s; a rule may call only %s", fn, strings.Join(ruleFunctions, ", ")))
			ok = false
		}
	}
	if prog.UsesWhole("entity") {
		v.add(key, "may use the entity only as entity.<property>")
		ok = false
	}
	fields := prog.Attributes("entity")
	for _, f := range fields {
		if fieldRestricted(v.policy, entityType, f) {
			v.add(key, fmt.Sprintf("reads %s.%s, which a visible: grant in acl.yaml restricts; "+
				"a message posted by this rule would disclose its value to every channel reader", entityType, f))
			ok = false
		}
	}
	return Rule{Program: prog, Fields: fields}, ok
}

// fieldRestricted reports whether acl.yaml may hide property of entityType
// from some reader. It is deliberately broad: a property named in any role's
// `visible:` grant for the type counts, as does one a role's closed-world
// `visible:` list leaves out, and one a client baseline's `visible:` or
// `redact:` hides. A rule or role reading such a property would turn a
// message into an oracle on its value.
//
// Nil: policy is accepted and restricts nothing (no acl.yaml).
func fieldRestricted(policy *acl.Policy, entityType, property string) bool {
	if policy == nil {
		return false
	}
	for _, role := range policy.Roles {
		for key := range role.Visible {
			if appliesTo(key, entityType) {
				// A role's visible: list is closed-world: a property it names
				// is restricted by the design rule, and one it leaves out is
				// hidden from that role. Either way every property of the
				// type is restricted.
				return true
			}
		}
	}
	for _, b := range policy.ClientBaselines {
		for key, fields := range b.Visible {
			if appliesTo(key, entityType) && !slices.Contains(fields, property) {
				return true
			}
		}
		for key, fields := range b.Redact {
			if appliesTo(key, entityType) && slices.Contains(fields, property) {
				return true
			}
		}
	}
	return false
}

// appliesTo reports whether an acl.yaml grant key names entityType: the type
// itself, one of its faces (type@face), or every type.
func appliesTo(key, entityType string) bool {
	return key == entityType || key == "*" || strings.HasPrefix(key, entityType+"@")
}
