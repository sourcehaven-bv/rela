package triggers

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The checks below hold a rule against the schema: the types, faces,
// copies and piles it names must be declared, and it must not fire on or
// write an external ref. [Check] runs them with the rule-shape checks.

// singleTypeErrors refuses a `create.type` that names an abstract type: a
// create writes one concrete type.
func singleTypeErrors(meta *metamodel.Metamodel, r Rule) []string {
	var errs []string
	for _, a := range r.Do {
		if a.CreateEntity == nil {
			continue
		}
		if variants, ok := metamodel.VariantsOf(meta, a.CreateEntity.Type); ok {
			errs = append(errs, fmt.Sprintf(
				"%s: create.type: %q is abstract and this setting takes one concrete type; name one of %s",
				ruleWhere(r), a.CreateEntity.Type, strings.Join(variants, ", ")))
		}
	}
	return errs
}

// faceErrors checks that every face in `on.faces:` is one the triggering
// type declares (TKT-4Y6CMV). A scope that matches nothing narrows the rule
// to nothing, so it is a load error rather than a silent no-match.
func faceErrors(meta *metamodel.Metamodel, r Rule) []string {
	var errs []string
	types := r.On.Entity
	for _, face := range r.On.Faces {
		if len(types) == 0 {
			if !slices.ContainsFunc(metamodel.EntityTypeNames(meta), func(t string) bool {
				return metamodel.DeclaresFace(meta, t, face)
			}) {

				errs = append(errs, fmt.Sprintf("%s: `on.faces: [%s]` names a face no entity type declares; "+
					"the trigger would never fire", ruleWhere(r), face))
			}
			continue
		}
		if slices.ContainsFunc(types, func(t string) bool { return metamodel.DeclaresFace(meta, t, face) }) {
			continue
		}
		hasFaces := slices.ContainsFunc(types, func(t string) bool { return metamodel.HasFaces(meta, t) })
		if !hasFaces {
			errs = append(errs, fmt.Sprintf("%s: `on.faces: [%s]` but %s declares no `faces:`; a type with one "+
				"content state needs no scope, so drop the key or declare the states on the type",
				ruleWhere(r), face, strings.Join(types, ", ")))
			continue
		}
		errs = append(errs, fmt.Sprintf("%s: `on.faces: [%s]` names no declared face of %s; "+
			"the trigger would never fire", ruleWhere(r), face, strings.Join(types, ", ")))
	}
	return errs
}

// copyErrors checks an `on.copy:` scope against the declared copies.
func copyErrors(meta *metamodel.Metamodel, r Rule) []string {
	if len(r.On.Copy) == 0 {
		return nil
	}
	var errs []string
	bad := func(format string, args ...any) {
		errs = append(errs, ruleWhere(r)+": "+fmt.Sprintf(format, args...))
	}
	if r.On.RelationCreated != "" || r.On.RelationRemoved != "" {
		bad("`on.copy` cannot be combined with `relation_created` or `relation_removed`; " +
			"a copy raises only entity events")
	}
	for _, name := range r.On.Copy {
		to, declared, ok := metamodel.CopyTargetOf(meta, name)
		if !declared {
			bad("`on.copy: [%s]` names no declared copy", name)
			continue
		}
		if !ok {
			continue // the schema loader reports the target
		}
		if len(r.On.Entity) > 0 && !slices.Contains(r.On.Entity, to.Type) {
			bad("`on.copy: [%s]` writes %q, which `on.entity` does not name; the trigger would never fire",
				name, to.Type)
		}
		if len(r.On.Faces) > 0 && !slices.Contains(r.On.Faces, to.Face) {
			bad("`on.copy: [%s]` writes face %q, which `on.faces` does not name; the trigger would never fire",
				name, to.Face)
		}
	}
	return errs
}

// pileErrors checks the add_to_pile and notify outputs against the declared
// piles.
func pileErrors(meta *metamodel.Metamodel, r Rule) []string {
	var errs []string
	for i, a := range r.Do {
		at := fmt.Sprintf("%s action %d", ruleWhere(r), i+1)
		if a.AddToPile != nil && strings.TrimSpace(a.AddToPile.Pile) == "" {
			errs = append(errs, at+": add_to_pile requires `pile:` (the pile name)")
		}
		if n := a.Notify; n != nil {
			errs = append(errs, notifyErrors(meta, n, at+": notify", len(r.On.Entity) > 0)...)
		}
	}
	return errs
}

func notifyErrors(meta *metamodel.Metamodel, n *NotifyAction, at string, hasOnEntity bool) []string {
	var errs []string
	decl, ok := meta.Piles[n.Pile]
	switch {
	case strings.TrimSpace(n.Pile) == "":
		errs = append(errs, at+" requires `pile:` (a key of the schema's top-level `piles:`)")
	case !ok:
		errs = append(errs, fmt.Sprintf("%s: pile %q is not declared under the schema's top-level `piles:`", at, n.Pile))
	case !decl.ReadState:
		errs = append(errs, fmt.Sprintf("%s: pile %q needs `read_state: true` to take notifications", at, n.Pile))
	}
	if ok && n.Reason != "" {
		if _, known := decl.Reasons[n.Reason]; !known {
			errs = append(errs, fmt.Sprintf("%s: reason %q is not one of pile %q's reasons", at, n.Reason, n.Pile))
		}
	}
	if strings.TrimSpace(n.Property) == "" {
		errs = append(errs, at+" requires `to.property:` (the property that holds the recipient)")
	}
	if len(n.Via) > 0 && !hasOnEntity {
		errs = append(errs, at+": `to.via:` needs the trigger's `on.entity:` to resolve the path")
	}
	if n.Type != "" && len(n.Via) == 0 {
		errs = append(errs, at+": `to.type:` applies only with `to.via:`")
	}
	return errs
}

// externalRefErrors refuses a rule that triggers on an external ref or
// writes one. A `set` or a `create` property is a system write that no field
// gate sees, and its value is an interpolated string, never an {id, url}
// object: only a script granted ref writes may write a ref.
func externalRefErrors(meta *metamodel.Metamodel, r Rule) []string {
	types := r.On.Entity
	if len(types) == 0 {
		types = metamodel.EntityTypeNames(meta)
	}
	var errs []string
	for _, t := range types {
		if r.On.Property != "" {
			if _, ok := metamodel.ExternalRefSystem(meta, t, r.On.Property); ok {
				errs = append(errs, fmt.Sprintf("%s: `on.property` %q is an external ref on %q; "+
					"a ref changes only through sync, so it cannot fire a trigger", ruleWhere(r), r.On.Property, t))
			}
		}
		for _, a := range r.Do {
			if a.Set == "" {
				continue
			}
			if _, ok := metamodel.ExternalRefSystem(meta, t, a.Set); ok {
				errs = append(errs, fmt.Sprintf("%s: `set` %q is an external ref on %q; only a script may write a ref",
					ruleWhere(r), a.Set, t))
			}
		}
	}
	for _, a := range r.Do {
		if a.CreateEntity == nil {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(a.CreateEntity.Properties)) {
			if _, ok := metamodel.ExternalRefSystem(meta, a.CreateEntity.Type, name); ok {
				errs = append(errs, fmt.Sprintf("%s: `create` property %q is an external ref on %q; "+
					"only a script may write a ref", ruleWhere(r), name, a.CreateEntity.Type))
			}
		}
	}
	return errs
}

// unsetSchemaErrors checks each `set: unset:` name against every type the
// rule fires on. The name must be a declared property, and one a trigger may
// remove: not required and not a state machine (the write would then fail
// on every firing), not
// computed (the manager owns its value), not a file (attachments change only
// through their API) and not an external ref (only sync and scripts write
// one). A rule without `on.entity:` is checked against every type that
// declares the property, and an unknown name there is one no type declares.
func unsetSchemaErrors(meta *metamodel.Metamodel, r Rule) []string {
	if meta == nil {
		return nil
	}
	var errs []string
	for _, a := range r.Do {
		if a.Unset == "" {
			continue
		}
		at := fmt.Sprintf("%s: `set.unset` %q", ruleWhere(r), a.Unset)
		types := r.On.Entity
		if len(types) == 0 {
			types = slices.DeleteFunc(metamodel.EntityTypeNames(meta), func(t string) bool {
				def, ok := meta.GetEntityDef(t)
				_, has := def.Properties[a.Unset]
				return !ok || !has
			})
			if len(types) == 0 {
				errs = append(errs, at+" is not a property of any entity type")
				continue
			}
		}
		for _, t := range types {
			def, ok := meta.GetEntityDef(t)
			if !ok {
				continue // reported by the entity-type checks
			}
			prop, ok := def.Properties[a.Unset]
			switch {
			case !ok:
				errs = append(errs, fmt.Sprintf("%s is not a property of %q", at, t))
			case prop.Required:
				errs = append(errs, fmt.Sprintf("%s is required on %q; removing it would fail every write", at, t))
			case prop.Rollup == nil && len(meta.Types[prop.Type].Transitions) > 0:
				errs = append(errs, fmt.Sprintf(
					"%s has state machine %q on %q; removing it is not a declared transition", at, prop.Type, t))
			case prop.Computed != "":
				errs = append(errs, fmt.Sprintf("%s is computed on %q; its value is derived on every write", at, t))
			case slices.Contains(metamodel.FileProperties(meta, t), a.Unset):
				errs = append(errs, fmt.Sprintf(
					"%s is a file property of %q; file values change only through the attachments API", at, t))
			default:
				if _, ext := metamodel.ExternalRefSystem(meta, t, a.Unset); ext {
					errs = append(errs, fmt.Sprintf("%s is an external ref on %q; only a script may write a ref", at, t))
				}
			}
		}
	}
	return errs
}
