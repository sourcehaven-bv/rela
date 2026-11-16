package appbuild

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/computed"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/scopes"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
	"github.com/Sourcehaven-BV/rela/internal/writerule"
)

// CheckSchemaCompiles compiles every expression in meta and returns one
// message per failure. It runs the compilers a server build runs (worlds,
// computed properties, query scopes, transitions, rules enforced on write).
// It also compiles the validation rules' conditions and filters, which
// otherwise compile only when validation runs and would then fail per rule
// rather than on save.
//
// The flows.yaml triggers are checked by [CheckTriggers].
//
// st answers nothing here: the transition compiler accepts
// related(...) only when a binder is wired, and a binder needs a store.
func CheckSchemaCompiles(meta *metamodel.Metamodel, st store.Store) []string {
	var problems []string
	add := func(what string, err error) {
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", what, err))
		}
	}
	_, err := worlds.Compile(meta)
	add("worlds", err)
	_, err = computed.Compile(meta)
	add("computed properties", err)
	_, err = rollup.Compile(meta)
	add("roll-up properties", err)
	_, err = scopes.Compile(meta)
	add("query scopes", err)
	_, err = CompileHeadlessFlows(meta, st)
	add("transitions", err)
	_, err = writerule.NewSet(meta)
	add("validations enforced on write", err)
	return append(problems, checkValidationRules(meta)...)
}

// LoadTriggers reads flows.yaml through r and decodes its `triggers:` against
// meta. It returns nil when the file is absent or declares no triggers.
func LoadTriggers(ctx context.Context, r configReader, meta *metamodel.Metamodel) ([]triggers.Rule, error) {
	data, err := readFlowsFile(ctx, r)
	if err != nil {
		return nil, err
	}
	return triggers.Parse(data, meta)
}

// CheckTriggers decodes and compiles the flows.yaml triggers against meta,
// as a server build does. A schema change can break a trigger that names a
// type, property or relation the change removes. The error text starts with
// the file name.
//
// Compiling a `related(...)` condition checks it against the schema and
// queries nothing, so `rela validate`, which opens no store, passes nil.
func CheckTriggers(ctx context.Context, r configReader, meta *metamodel.Metamodel, st store.Store) error {
	rules, err := LoadTriggers(ctx, r, meta)
	if err != nil || len(rules) == 0 {
		return err
	}
	match := func(context.Context, store.GraphQuery, []string) (map[string]bool, error) {
		return nil, errors.New("appbuild: CheckTriggers answers no traversal")
	}
	if st != nil {
		match = store.IDMatcher(st)
	}
	binder, err := relresolve.NewBinder(meta, relresolve.Ungated, match)
	if err != nil { // coverage-ignore: defensive: meta, gate and match are all non-nil here
		return err
	}
	//nolint:contextcheck // New takes no ctx; its git user lookup has its own timeout, as before the move
	if _, err := triggers.New(meta, rules, triggers.WithTraversals(binder)); err != nil {
		return fmt.Errorf("flows.yaml: triggers: %w", err)
	}
	return nil
}

// checkValidationRules compiles each rule's conditions against every entity
// type the rule applies to, and parses its filters.
func checkValidationRules(meta *metamodel.Metamodel) []string {
	ev := predicatefns.NewEvaluator(meta)
	var problems []string
	for _, rule := range meta.Validations {
		label := fmt.Sprintf("validation %q", rule.Name)
		for _, f := range [][]string{rule.When, rule.Then} {
			if _, err := filter.ParseAll(f); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", label, err))
			}
		}
		for rel, c := range rule.Relations {
			if _, err := filter.ParseAll(c.Where); err != nil {
				problems = append(problems, fmt.Sprintf("%s: relations %q: %v", label, rel, err))
			}
		}
		for _, src := range []string{rule.WhenCondition, rule.ThenCondition} {
			if src == "" {
				continue
			}
			for _, t := range ruleTypes(meta, rule) {
				prog, err := ev.Compile(t, src)
				if err == nil {
					err = predicatefns.ValidateTraversals(meta, t, prog)
				}
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: condition %q on %q: %v", label, src, t, err))
				}
			}
		}
	}
	return problems
}

// ruleTypes lists the entity types a validation rule applies to, sorted so
// messages come out in a stable order.
func ruleTypes(meta *metamodel.Metamodel, rule metamodel.ValidationRule) []string {
	if types := rule.TargetTypes(); types != nil {
		sort.Strings(types)
		return types
	}
	types := make([]string, 0, len(meta.Entities))
	for t := range meta.Entities {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
