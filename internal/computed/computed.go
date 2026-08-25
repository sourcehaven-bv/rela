// Package computed compiles and evaluates materialized entity-local computed
// properties. Expressions use predicate's strict Lua-compatible scalar IR;
// compilation infers dependencies and rejects cycles before any write occurs.
package computed

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

type compiledProperty struct {
	name         string
	def          metamodel.PropertyDef
	program      *predicate.Program
	dependencies []string
}

// Set is the immutable compiled computation plan for one metamodel. It is safe
// for concurrent evaluations; each call builds fresh predicate bindings.
type Set struct {
	meta  *metamodel.Metamodel
	now   func() time.Time
	plans map[string][]compiledProperty
	all   map[string]map[string]bool
}

// Compile validates and compiles every computed entity property. A malformed
// expression, type mismatch or dependency cycle is a project-load error.
func Compile(meta *metamodel.Metamodel) (*Set, error) {
	return CompileWithClock(meta, time.Now)
}

// CompileWithClock is Compile with a deterministic write-time clock for tests.
func CompileWithClock(meta *metamodel.Metamodel, now func() time.Time) (*Set, error) {
	if meta == nil {
		return nil, errors.New("computed: Compile: nil metamodel")
	}
	if now == nil {
		return nil, errors.New("computed: Compile: nil clock")
	}
	set := &Set{meta: meta, now: now, plans: map[string][]compiledProperty{}, all: map[string]map[string]bool{}}
	var problems []string
	for _, entityType := range sortedKeys(meta.Entities) {
		def := meta.Entities[entityType]
		env := predicate.NewEnv()
		if err := env.DeclareVar("entity", predicatefns.EntityRecordType(meta, &def)); err != nil {
			problems = append(problems, fmt.Sprintf("entity %q: %v", entityType, err))
			continue
		}
		if err := predicatefns.Declare(env); err != nil {
			problems = append(problems, fmt.Sprintf("entity %q: %v", entityType, err))
			continue
		}
		compiled := map[string]compiledProperty{}
		for _, name := range sortedKeys(def.Properties) {
			pd := def.Properties[name]
			if pd.Computed == "" {
				continue
			}
			if pd.List || pd.Type == metamodel.PropertyTypeFile {
				// The metamodel loader normally catches these; retain the guard
				// for programmatically-built metamodels used by embedders/tests.
				problems = append(problems, fmt.Sprintf("entity %q property %q: computed requires a supported scalar type", entityType, name))
				continue
			}
			typ, ok := predicatefns.ScalarTypeForProp(meta, &pd)
			if !ok {
				problems = append(problems, fmt.Sprintf("entity %q property %q: type %q is not supported by computed expressions", entityType, name, pd.Type))
				continue
			}
			prog, err := predicate.CompileValue(env, pd.Computed, predicate.ValueProfile(typ))
			if err != nil {
				problems = append(problems, fmt.Sprintf("entity %q property %q computed: %v", entityType, name, err))
				continue
			}
			compiled[name] = compiledProperty{name: name, def: pd, program: prog, dependencies: prog.Attributes("entity")}
		}
		order, err := topo(entityType, compiled)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if len(order) > 0 {
			set.all[entityType] = map[string]bool{}
			for _, name := range order {
				set.plans[entityType] = append(set.plans[entityType], compiled[name])
				set.all[entityType][name] = true
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("computed: invalid definitions:\n  %s", joinLines(problems))
	}
	return set, nil
}

// Evaluate recomputes every computed property of e in dependency order. Nil
// removes the materialized key; any other evaluation error aborts the write.
func (s *Set) Evaluate(ctx context.Context, e *entity.Entity) error {
	if e == nil {
		return errors.New("computed: Evaluate: nil entity")
	}
	def, ok := s.meta.GetEntityDef(e.Type)
	if !ok {
		return nil // normal metamodel validation owns the unknown-type error
	}
	if e.Properties == nil {
		e.Properties = map[string]any{}
	}
	for _, cp := range s.plans[e.Type] {
		bindings := predicate.NewBindings()
		if err := bindings.SetVar("entity", predicatefns.EntityRecord(s.meta, def, e.ID, e.Type, e.Properties)); err != nil {
			return fmt.Errorf("computed: entity %s property %s: %w", e.ID, cp.name, err)
		}
		if err := predicatefns.Bind(bindings, s.now()); err != nil {
			return fmt.Errorf("computed: entity %s property %s: %w", e.ID, cp.name, err)
		}
		value, err := cp.program.Eval(ctx, bindings)
		if err != nil {
			return fmt.Errorf("computed: entity %s property %s: %w", e.ID, cp.name, err)
		}
		if _, nilValue := value.(predicate.Nil); nilValue {
			delete(e.Properties, cp.name)
			continue
		}
		goValue, err := materialize(value, &cp.def)
		if err != nil {
			return fmt.Errorf("computed: entity %s property %s: %w", e.ID, cp.name, err)
		}
		e.Properties[cp.name] = goValue
	}
	return nil
}

// IsComputed reports whether property is materialized by this set.
func (s *Set) IsComputed(entityType, property string) bool {
	return s != nil && s.all[entityType][property]
}

// SQLPortable reports whether a computed property's complete program is
// portable to a future SQL lowering.
func (s *Set) SQLPortable(entityType, property string) bool {
	for _, cp := range s.plans[entityType] {
		if cp.name == property {
			return cp.program.SQLPortable()
		}
	}
	return false
}

func materialize(v predicate.Value, def *metamodel.PropertyDef) (any, error) {
	switch x := v.(type) {
	case predicate.String:
		return x.String(), nil
	case predicate.Int:
		return x.Int64(), nil
	case predicate.Bool:
		return x.Bool(), nil
	case predicate.Date:
		return x.Time().Format(def.GetDateFormat()), nil
	default:
		return nil, fmt.Errorf("unsupported result value %T", v)
	}
}

func topo(entityType string, compiled map[string]compiledProperty) ([]string, error) {
	state := map[string]uint8{} // 0 unseen, 1 visiting, 2 done
	var order, stack []string
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 2:
			return nil
		case 1:
			start := 0
			for i, n := range stack {
				if n == name {
					start = i
					break
				}
			}
			cycle := append(append([]string(nil), stack[start:]...), name)
			return fmt.Errorf("entity %q computed-property cycle: %v", entityType, cycle)
		}
		state[name] = 1
		stack = append(stack, name)
		deps := append([]string(nil), compiled[name].dependencies...)
		sort.Strings(deps)
		for _, dep := range deps {
			if _, computedDep := compiled[dep]; computedDep {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
		order = append(order, name)
		return nil
	}
	for _, name := range sortedKeys(compiled) {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func joinLines(lines []string) string {
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			out.WriteString("\n  ")
		}
		out.WriteString(line)
	}
	return out.String()
}
