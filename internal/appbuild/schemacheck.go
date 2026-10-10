package appbuild

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/computed"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/scopes"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// CheckSchemaCompiles compiles every expression in meta and returns one
// message per failure. It runs the compilers a server build runs (worlds,
// computed properties, query scopes, transitions, automations) and also the validation
// rules' conditions and filters, which otherwise compile only when
// validation runs and would then fail per rule rather than on save.
//
// st answers nothing here: the transition and automation compilers accept
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
	_, err = scopes.Compile(meta)
	add("query scopes", err)
	_, err = statemachine.Compile(meta, statemachine.WithTraversals(ungatedBinder(meta, st)))
	add("transitions", err)
	if len(meta.Automations) > 0 {
		_, err = automation.NewEngineFromMetamodel(meta, meta.Automations,
			automation.WithTraversals(ungatedBinder(meta, st)))
		add("automations", err)
	}
	return append(problems, checkValidationRules(meta)...)
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
	if rule.EntityType != "" {
		return []string{rule.EntityType}
	}
	types := make([]string, 0, len(meta.Entities))
	for t := range meta.Entities {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
