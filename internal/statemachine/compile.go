package statemachine

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// Compile builds the executable state-machine [Set] from a metamodel's
// declarative transitions. It runs once at startup. It returns an error
// (aggregating every problem found) if any machine is malformed — a dangling
// from/to/initial, or a `when:` predicate that fails to compile — so a bad
// metamodel is rejected at boot, not at write time.
//
// A metamodel with no transitions compiles to an empty [Set]; [Set.Enforce] is
// then a no-op and every write behaves exactly as it did before this feature.
//
// meta must not be nil (a required input from appbuild).
func Compile(meta *metamodel.Metamodel) (*Set, error) {
	if meta == nil {
		return nil, errors.New("statemachine: Compile: nil metamodel")
	}

	set := &Set{
		machines: map[string]*Machine{},
		propType: map[string]map[string]string{},
	}
	var problems []string

	// Build one machine per state-machine custom type, in name order so the
	// error report and the compiled env are deterministic.
	for _, typeName := range sortedKeys(meta.Types) {
		ct := meta.Types[typeName]
		if len(ct.Transitions) == 0 {
			continue // not a machine
		}
		m, errs := compileMachine(typeName, ct)
		problems = append(problems, errs...)
		if m != nil {
			set.machines[typeName] = m
		}
	}

	// Index which (entity type, property) pairs are state machines, so Enforce
	// can find the machine-typed properties of a written entity without
	// consulting the metamodel again.
	for _, etName := range sortedKeys(meta.Entities) {
		et := meta.Entities[etName]
		for _, propName := range sortedKeys(et.Properties) {
			pd := et.Properties[propName]
			if _, ok := set.machines[pd.Type]; !ok {
				continue
			}
			// A state machine is a single-valued lifecycle; a list-typed
			// property has no single "current value" to transition (Enforce
			// reads it via GetString, which would flatten the list to a
			// coincidental scalar). Reject at boot rather than mis-enforce
			// (RR-F30CZ/N4).
			if pd.List {
				problems = append(problems, fmt.Sprintf(
					"entity %q property %q: type %q declares transitions but the property is a list; "+
						"state machines require a single-valued property", etName, propName, pd.Type))
				continue
			}
			if set.propType[etName] == nil {
				set.propType[etName] = map[string]string{}
			}
			set.propType[etName][propName] = pd.Type
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("statemachine: invalid transitions:\n  %s", joinLines(problems))
	}
	return set, nil
}

// compileMachine builds one machine and returns it plus any well-formedness
// problems. A machine with fatal problems still returns a (partial) machine so
// indexing proceeds, but Compile aggregates the problems into a boot error.
func compileMachine(typeName string, ct metamodel.CustomType) (machine *Machine, problems []string) {
	values := map[string]bool{}
	for _, v := range ct.Values {
		values[v] = true
	}

	// Entry value: Initial if set, else Default. Whichever supplies it must be
	// a declared value — validate the RESOLVED entry, not just Initial, so a
	// typo'd Default-as-entry fails fast at boot too (RR-VB2DE).
	entry := ct.Initial
	if entry == "" {
		entry = ct.Default
	}
	switch {
	case entry == "":
		// A state machine (compileMachine is only reached for types WITH
		// transitions) MUST declare an entry value, so create is pinned to it
		// (BUG-X1C7S). Without one, EnforceCreate would treat create as
		// unconstrained — letting an entity enter ANY state, including a
		// guard-protected target, bypassing the machine via create instead of
		// a transition. Reject at boot; the operator declares `initial` (or
		// `default`) to say which state new entities start in.
		problems = append(problems, fmt.Sprintf(
			"type %q: a state machine (has transitions) must declare an `initial` "+
				"(or `default`) entry value so creates are constrained to it", typeName))
	case !values[entry]:
		source := "initial"
		if ct.Initial == "" {
			source = "default"
		}
		problems = append(problems, fmt.Sprintf(
			"type %q: %s %q is not a declared value", typeName, source, entry))
	}

	env, err := buildEnv(ct)
	if err != nil {
		problems = append(problems, fmt.Sprintf("type %q: %v", typeName, err))
	}

	edges := map[transitionKey]edge{}
	for i, tr := range ct.Transitions {
		if !values[tr.From] {
			problems = append(problems, fmt.Sprintf(
				"type %q: transition[%d] from %q is not a declared value", typeName, i, tr.From))
		}
		if !values[tr.To] {
			problems = append(problems, fmt.Sprintf(
				"type %q: transition[%d] to %q is not a declared value", typeName, i, tr.To))
		}
		key := transitionKey{tr.From, tr.To}
		if _, dup := edges[key]; dup {
			problems = append(problems, fmt.Sprintf(
				"type %q: transition[%d] %s→%s is declared more than once", typeName, i, tr.From, tr.To))
		}

		var prog *predicate.Program
		if tr.When != "" && env != nil {
			prog, err = predicate.Compile(env, tr.When)
			if err != nil {
				problems = append(problems, fmt.Sprintf(
					"type %q: transition[%d] %s→%s when: %v", typeName, i, tr.From, tr.To, err))
			}
		}
		edges[key] = edge{guard: tr.Guard, when: prog}
	}

	return &Machine{name: typeName, edges: edges, entry: entry}, problems
}
