// Package conditionlint performs author-time validation of the condition
// expressions used by wizard forms (`visible_when` / `required_when`).
//
// The expressions are evaluated at runtime by the SPA's own (permissive)
// TypeScript engine. This package can't run that engine, but the grammar is
// deliberately kept congruent with the Go `internal/predicate` engine, so we
// reuse predicate's parser + type checker as an author-time sanity check.
//
// Two caveats, documented for callers:
//
//   - Predicate is STRICTER than the runtime engine (strict Lua-style typing;
//     cross-type comparisons are compile errors, where the runtime coerces).
//     So this lint flags a SUPERSET of real problems: every error it reports is
//     worth an author's attention, but a condition it rejects on type grounds
//     might still evaluate fine at runtime. It is a sanity check, not a 1:1
//     mirror.
//   - It only knows the `form` namespace (declared from the entity's
//     properties). References to `entity.*` / `current_user.*` are not used by
//     wizard forms today and are not checked here.
//
// The high-value checks that DO hold exactly: an expression that fails to parse
// is a real bug, and a `form.<field>` reference to a property that doesn't
// exist on the entity is a real bug. Those are what authors most often get
// wrong, and this catches them before the form silently misbehaves.
package conditionlint

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// Lint returns one message per invalid condition found across all forms in cfg.
// The messages are sorted for deterministic output. An empty slice means every
// condition parsed and referenced only known fields.
func Lint(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) []string {
	var errs []string
	for formID, form := range cfg.Forms {
		env, ok := formEnv(form.EntityType, meta)
		if !ok {
			// Unknown entity type is already reported by validateForms; skip
			// condition checks we can't ground.
			continue
		}
		lintForm(formID, form, env, &errs)
	}
	sort.Strings(errs)
	return errs
}

// lintForm checks every condition attached to a form's steps and fields.
func lintForm(formID string, form dataentryconfig.Form, env *predicate.Env, errs *[]string) {
	for si, step := range form.Steps {
		check(env, step.VisibleWhen, fmt.Sprintf("form %q: step[%d] visible_when", formID, si), errs)
		for i, f := range step.Fields {
			lintField(formID, fmt.Sprintf("step[%d] ", si), i, f, env, errs)
		}
		for i, r := range step.Relations {
			check(env, r.VisibleWhen,
				fmt.Sprintf("form %q: step[%d] relation[%d] visible_when", formID, si, i), errs)
		}
	}
	// Flat forms may also carry conditions on fields/relations.
	for i, f := range form.Fields {
		lintField(formID, "", i, f, env, errs)
	}
	for i, r := range form.Relations {
		check(env, r.VisibleWhen, fmt.Sprintf("form %q: relation[%d] visible_when", formID, i), errs)
	}
}

func lintField(formID, ctx string, i int, f dataentryconfig.FormField, env *predicate.Env, errs *[]string) {
	check(env, f.VisibleWhen, fmt.Sprintf("form %q: %sfield[%d] visible_when", formID, ctx, i), errs)
	check(env, f.RequiredWhen, fmt.Sprintf("form %q: %sfield[%d] required_when", formID, ctx, i), errs)
}

// check compiles one expression (if non-empty) and records a message on failure.
func check(env *predicate.Env, expr, where string, errs *[]string) {
	if expr == "" {
		return
	}
	if _, err := predicate.Compile(env, expr); err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: %v", where, err))
	}
}

// formEnv builds a predicate Env whose `form` variable is a record of the
// entity's properties, typed as closely as the metamodel allows. Returns false
// if the entity type is unknown.
func formEnv(entityType string, meta *metamodel.Metamodel) (*predicate.Env, bool) {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil, false
	}
	fields := make(predicate.RecordType, len(entDef.Properties))
	for name, def := range entDef.Properties {
		fields[name] = predicateType(def)
	}
	env := predicate.NewEnv()
	// DeclareVar only errors on empty name / nil type / redeclaration, none of
	// which happen here (single declaration, non-nil type).
	_ = env.DeclareVar("form", fields)
	return env, true
}

// predicateType maps a metamodel property type to a predicate scalar type.
// Booleans and integers map precisely; everything else (strings, enums, dates,
// custom types) is treated as a string, which is how the runtime engine
// compares them and keeps the lint from over-rejecting.
func predicateType(def metamodel.PropertyDef) predicate.Type {
	switch def.Type {
	case "boolean":
		return predicate.BoolType
	case "integer":
		return predicate.NumberType
	default:
		return predicate.StringType
	}
}
