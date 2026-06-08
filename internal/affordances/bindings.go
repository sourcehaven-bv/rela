package affordances

import (
	"context"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// RelationLookup is the narrow contract the resolver needs from the
// graph to answer has_relation / count_relations and to resolve local
// roles. Defined at the consumer (CLAUDE.md "interfaces at the call
// site"); the wiring site supplies a snapshot-backed implementation.
//
// OutgoingCounts returns, for fromID, a map of relation type → count
// of outgoing edges of that type. One call answers both has_relation
// (type present) and count_relations (the count), so the binding
// context scans the graph once per resolve rather than once per
// predicate (RR-08AK). HasEdge reports whether a specific edge
// fromID --relType--> toID exists — a targeted query used for
// local-role resolution (principal --role-relation--> entity).
type RelationLookup interface {
	OutgoingCounts(ctx context.Context, fromID string) map[string]int
	HasEdge(ctx context.Context, fromID, relType, toID string) bool
}

// bindingContext carries everything a single Resolver call needs to
// build per-entity predicate Bindings: the principal, the entity, the
// principal's effective role set (globals plus ancestor-conferred plus
// direct local), the principal's globals-only role set, and the graph
// lookup. It is constructed once per resolver call (snapshot-once).
//
// The caller's request context is NOT stored here — it is threaded as
// a method parameter (passes, evalGrants, …) into predicate Eval and
// the host-function calls it makes, matching the predicate package's
// own ctx-as-parameter convention (golangci-lint containedctx) and
// the caller-ctx pattern from TKT-WFB6 / PR#825.
type bindingContext struct {
	principal principal.Principal
	entity    *entity.Entity
	// entityRoles is the per-entity effective role set: globals (incl.
	// group expansion) ∪ ancestor-conferred ∪ direct-local. This is
	// the answer to "does the principal hold role X on this entity"
	// and the source the has_role host function consults (RR-JRPZ).
	entityRoles map[string]bool
	// globalRoles is the globals-only set (no per-entity grants). Used
	// by has_global_role for predicates that want to discriminate
	// "globally a admin" from "admin on this one entity."
	globalRoles map[string]bool
	lookup      RelationLookup
	resolver    *PolicyResolver
	// userID is the principal's identity as it appears on role-relation
	// edges and current_user.id.
	userID string

	// outgoing caches the entity's outgoing-edge counts, loaded once
	// on first host-func use (has_relation / count_relations) so a
	// resolve call scans the graph at most once for them.
	outgoing      map[string]int
	outgoingReady bool
}

// outgoingCounts returns the entity's outgoing-edge counts, loading
// and caching them on first call.
func (bc *bindingContext) outgoingCounts(ctx context.Context) map[string]int {
	if !bc.outgoingReady {
		bc.outgoing = bc.lookup.OutgoingCounts(ctx, bc.entity.ID)
		bc.outgoingReady = true
	}
	return bc.outgoing
}

// newBindings builds the predicate Bindings for evaluating a grant's
// predicate against bc's entity. The entity record is coerced from
// the metamodel-declared property types; host functions close over bc.
func (bc *bindingContext) newBindings(meta *metamodel.Metamodel) (*predicate.Bindings, error) {
	b := predicate.NewBindings()

	if err := b.SetVar("entity", bc.entityRecord(meta)); err != nil {
		return nil, err
	}
	if err := b.SetVar("current_user", predicate.NewRecord(map[string]predicate.Value{
		"id":   predicate.NewString(bc.userID),
		"tool": predicate.NewString(bc.principal.Tool),
	})); err != nil {
		return nil, err
	}

	setters := []struct {
		name string
		fn   predicate.Func
	}{
		{"has_role", predicate.FuncFunc(bc.hasRole)},
		{"has_global_role", predicate.FuncFunc(bc.hasGlobalRole)},
		{"has_relation", predicate.FuncFunc(bc.hasRelation)},
		{"count_relations", predicate.FuncFunc(bc.countRelations)},
		{"string_in_list", predicate.FuncFunc(stringInList)},
	}
	for _, s := range setters {
		if err := b.SetFunc(s.name, s.fn); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// entityRecord coerces the entity's properties into a predicate.Record
// using the metamodel-declared types. Off-type or missing values bind
// as Nil rather than erroring (DR-C2): permissive storage must not
// turn data drift into an Eval failure. id and type are always set.
func (bc *bindingContext) entityRecord(meta *metamodel.Metamodel) predicate.Value {
	fields := map[string]predicate.Value{
		"id":   predicate.NewString(bc.entity.ID),
		"type": predicate.NewString(bc.entity.Type),
	}
	if meta != nil {
		if def, ok := meta.Entities[bc.entity.Type]; ok {
			for name, prop := range def.Properties {
				if _, modeled := propertyPredicateType(meta, prop); !modeled {
					continue
				}
				fields[name] = coerceValue(prop, bc.entity.Properties[name])
			}
		}
	}
	return predicate.NewRecord(fields)
}

// coerceValue best-effort coerces a stored property value to the
// predicate Value matching its metamodel type. Unconvertible or
// missing values become Nil. List properties become a List of coerced
// scalars (single scalar promoted to a one-element list).
func coerceValue(prop metamodel.PropertyDef, raw interface{}) predicate.Value {
	if prop.List {
		return coerceList(prop, raw)
	}
	return coerceScalar(prop.Type, raw)
}

// coerceList coerces a list-typed property. Surprising-but-deliberate
// fail-soft choices (M3):
//   - A bare scalar (not a slice) is promoted to a one-element list,
//     so a hand-edited `tags: vip` reads the same as `tags: [vip]`.
//   - A nil/absent value is the empty list (so list membership checks
//     are false, never an Eval error).
//   - Non-coercible elements (e.g. a non-string in a string list)
//     become Nil holes rather than failing the whole list.
func coerceList(prop metamodel.PropertyDef, raw interface{}) predicate.Value {
	elems := []predicate.Value{}
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			elems = append(elems, coerceScalar(prop.Type, e))
		}
	case nil:
		// empty list
	default:
		// single scalar promoted to one-element list
		elems = append(elems, coerceScalar(prop.Type, raw))
	}
	return predicate.NewList(elems)
}

func coerceScalar(typeName string, raw interface{}) predicate.Value {
	if raw == nil {
		return predicate.NewNil()
	}
	switch typeName {
	case metamodel.PropertyTypeInteger:
		return coerceNumber(raw)
	case metamodel.PropertyTypeBoolean:
		return coerceBool(raw)
	default:
		// string / enum / date / rrule / custom — string-valued.
		if s, ok := raw.(string); ok {
			return predicate.NewString(s)
		}
		return predicate.NewNil()
	}
}

func coerceNumber(raw interface{}) predicate.Value {
	switch v := raw.(type) {
	case int:
		return predicate.NewNumberFromInt(v)
	case int64:
		return predicate.NewNumber(float64(v))
	case float64:
		return predicate.NewNumber(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return predicate.NewNumber(f)
		}
	}
	return predicate.NewNil()
}

func coerceBool(raw interface{}) predicate.Value {
	switch v := raw.(type) {
	case bool:
		return predicate.NewBool(v)
	case string:
		if v == "true" {
			return predicate.NewBool(true)
		}
		if v == "false" {
			return predicate.NewBool(false)
		}
	}
	return predicate.NewNil()
}
