package statemachine

import (
	"context"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// buildEnv constructs the predicate environment a machine's `when:` expressions
// compile and evaluate against. The surface is intentionally small: an `entity`
// record (id, type, and this machine's own value are the useful fields) plus the
// two graph host functions has_relation / count_relations. This mirrors the
// affordances env but carries no user/role vars — a transition precondition is a
// data check, not authorization (authorization is the guard).
func buildEnv(_ metamodel.CustomType) (*predicate.Env, error) {
	env := predicate.NewEnv()

	// `entity.value` is the machine property's OWN current value, bound
	// independently of what that property is named (RR-NODYR — the old
	// hardcoded `status` field misbehaved for machines on other-named
	// properties). id/type are the stable pseudo-fields.
	entityRec := predicate.RecordType{
		"id":    predicate.StringType,
		"type":  predicate.StringType,
		"value": predicate.StringType,
	}
	// coverage-ignore-start: defensive: DeclareVar on a fresh env with the fixed non-empty name "entity" and a non-nil
	// RecordType never errors
	if err := env.DeclareVar("entity", entityRec); err != nil {
		return nil, err
	}
	// coverage-ignore-end

	rec := predicate.RecordType{}
	str := predicate.StringType
	num := predicate.NumberType
	boolT := predicate.BoolType
	funcs := []struct {
		name string
		sig  predicate.FuncSig
	}{
		{"has_relation", predicate.FuncSig{Params: []predicate.Type{rec, str}, Return: boolT}},
		{"count_relations", predicate.FuncSig{Params: []predicate.Type{rec, str}, Return: num}},
	}
	for _, f := range funcs {
		// coverage-ignore-start: defensive: DeclareFunc on a fresh env with fixed non-empty names, scalar return types,
		// and non-nil params never
		// errors
		if err := env.DeclareFunc(f.name, f.sig); err != nil {
			return nil, err
		}
		// coverage-ignore-end
	}
	return env, nil
}

// evalWhen evaluates a compiled precondition against the written entity and the
// graph. Returns true if the precondition holds (or is absent). A graph lookup
// is required for has_relation/count_relations; if the predicate references
// them and lookup is nil, evaluation errors (surfaced by the caller as a
// precondition failure) rather than silently passing.
func evalWhen(
	ctx context.Context, prog *predicate.Program, e *entity.Entity, prop string, lookup GraphLookup,
) (bool, error) {
	b := predicate.NewBindings()
	// coverage-ignore-start: defensive: SetVar with the fixed name "entity" and the always-non-nil record from
	// entityRecord never errors
	if err := b.SetVar("entity", entityRecord(e, prop)); err != nil {
		return false, err
	}
	// coverage-ignore-end
	gb := &graphBindings{entityID: e.ID, lookup: lookup}
	// coverage-ignore-start: defensive: SetFunc with a fixed non-empty name and a non-nil FuncFunc never errors
	if err := b.SetFunc("has_relation", predicate.FuncFunc(gb.hasRelation)); err != nil {
		return false, err
	}
	// coverage-ignore-end
	// coverage-ignore-start: defensive: SetFunc with a fixed non-empty name and a non-nil FuncFunc never errors
	if err := b.SetFunc("count_relations", predicate.FuncFunc(gb.countRelations)); err != nil {
		return false, err
	}
	// coverage-ignore-end

	v, err := prog.Eval(ctx, b)
	if err != nil {
		return false, err
	}
	bv, ok := v.(predicate.Bool)
	// coverage-ignore-start: defensive: predicate.Compile rejects a non-bool top-level expression, so a compiled
	// program always evaluates to a
	// Bool
	if !ok {
		return false, nil
	}
	// coverage-ignore-end
	return bv.Bool(), nil
}

// entityRecord coerces an entity into the predicate record the `when:` env
// expects: id, type, and `value` = the machine property's own current value
// (read from prop, whatever it is named). Missing values bind as empty strings.
func entityRecord(e *entity.Entity, prop string) predicate.Value {
	return predicate.NewRecord(map[string]predicate.Value{
		"id":    predicate.NewString(e.ID),
		"type":  predicate.NewString(e.Type),
		"value": predicate.NewString(e.GetString(prop)),
	})
}

// graphBindings supplies has_relation / count_relations for one evaluation,
// scanning the graph at most once via the lookup.
type graphBindings struct {
	entityID string
	lookup   GraphLookup

	counts map[string]int
	ready  bool
}

func (g *graphBindings) outgoing(ctx context.Context) map[string]int {
	if !g.ready {
		if g.lookup != nil {
			g.counts = g.lookup.OutgoingCounts(ctx, g.entityID)
		}
		g.ready = true
	}
	return g.counts
}

func (g *graphBindings) hasRelation(ctx context.Context, args []predicate.Value) (predicate.Value, error) {
	relType := stringArg(args, 1)
	return predicate.NewBool(g.outgoing(ctx)[relType] > 0), nil
}

func (g *graphBindings) countRelations(ctx context.Context, args []predicate.Value) (predicate.Value, error) {
	relType := stringArg(args, 1)
	return predicate.NewNumber(float64(g.outgoing(ctx)[relType])), nil
}

// stringArg reads the i-th arg as a string, "" if absent/wrong type.
func stringArg(args []predicate.Value, i int) string {
	// coverage-ignore-start: defensive: both callers pass i=1 and the FuncSig declares 2 params, so the compiler's
	// arity check guarantees args
	// has index 1
	if i >= len(args) {
		return ""
	}
	// coverage-ignore-end
	if s, ok := args[i].(predicate.String); ok {
		return s.String()
	}
	// coverage-ignore: defensive: arg 1 is declared StringType in the FuncSig, so the type checker guarantees it is a
	// predicate.String at
	// runtime
	return ""
}

// sortedKeys returns the map keys sorted, for deterministic iteration.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func joinLines(ss []string) string { return strings.Join(ss, "\n  ") }
