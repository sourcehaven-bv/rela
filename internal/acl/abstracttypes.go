package acl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// expandAbstractTypes rewrites every entity-type name in the policy that
// names an abstract schema type into the names of its variants. An abstract
// type stores no entities, so a grant on it would otherwise match nothing:
// the operator believes they granted access to every task, and granted none.
//
// Type lists (`create: [task]`, `read: [task@draft]`) expand in place and
// keep a face suffix. Type-keyed maps expand by key, and an entry naming
// the variant itself wins over the abstract entry: that is the narrower
// reading, so a variant entry written to restrict a role does not gain the
// abstract entry's fields. A ceiling's `redact:` is the exception. It is a
// denylist, so the variant gets the abstract entry's fields as well as its
// own: an override there would un-redact a field. Where a variant also has
// its own `visible:`, the redacted fields are removed from it (see
// foldRedactIntoVisible).
//
// user_entity_type names one type, so an abstract name there is an error.
// The rest of the policy is still expanded first, so a caller that keeps
// a policy despite the error never holds an unexpanded deny or redact.
//
// Idempotent, and a policy that names no abstract type is not written to:
// the `rela acl` commands validate a policy that may already be in use.
func expandAbstractTypes(p *Policy, meta MetamodelView) error {
	var err error
	if _, ok := meta.Variants(strings.TrimSpace(p.UserEntityType)); ok {
		err = fmt.Errorf("acl: user_entity_type %q is an abstract type; name one of its variants",
			p.UserEntityType)
	}
	ex := typeExpander{meta: meta}
	if !namesAbstractType(p, ex) {
		return err
	}
	for name, role := range p.Roles {
		role.Create = ex.list(role.Create)
		role.Update = ex.list(role.Update)
		role.Delete = ex.list(role.Delete)
		role.Convert = ex.list(role.Convert)
		role.Rename = ex.list(role.Rename)
		role.Read = ex.list(role.Read)
		role.Fields = expandOverride(ex, role.Fields)
		role.Visible = expandOverride(ex, role.Visible)
		role.Options = expandOverride(ex, role.Options)
		role.Relations = expandOverride(ex, role.Relations)
		p.Roles[name] = role
	}
	for name, b := range p.ClientBaselines {
		b.Restriction = ex.restriction(b.Restriction)
		p.ClientBaselines[name] = b
	}
	for name, g := range p.ScopeGrants {
		g.Restriction = ex.restriction(g.Restriction)
		p.ScopeGrants[name] = g
	}
	return err
}

// namesAbstractType reports whether any type list or type-keyed map in the
// policy names an abstract type.
func namesAbstractType(p *Policy, ex typeExpander) bool {
	for _, role := range p.Roles {
		if ex.anyAbstract(role.Create, role.Update, role.Delete, role.Convert, role.Rename, role.Read) ||
			hasAbstractKey(ex, role.Fields) || hasAbstractKey(ex, role.Visible) ||
			hasAbstractKey(ex, role.Options) || hasAbstractKey(ex, role.Relations) {

			return true
		}
	}
	restrictions := make([]Restriction, 0, len(p.ClientBaselines)+len(p.ScopeGrants))
	for _, b := range p.ClientBaselines {
		restrictions = append(restrictions, b.Restriction)
	}
	for _, g := range p.ScopeGrants {
		restrictions = append(restrictions, g.Restriction)
	}
	for _, r := range restrictions {
		if ex.anyAbstract(r.Read, r.Create, r.Update, r.Delete,
			r.DenyRead, r.DenyCreate, r.DenyUpdate, r.DenyDelete, r.DenyWrite) ||
			hasAbstractKey(ex, r.Visible) || hasAbstractKey(ex, r.Redact) {

			return true
		}
	}
	return false
}

type typeExpander struct{ meta MetamodelView }

// names returns the variants of an abstract type, or nil for any other name.
func (ex typeExpander) names(typ string) []string {
	variants, ok := ex.meta.Variants(typ)
	if !ok {
		return nil
	}
	return variants
}

// anyAbstract reports whether an entry of any list names an abstract type.
func (ex typeExpander) anyAbstract(lists ...[]string) bool {
	for _, list := range lists {
		for _, entry := range list {
			if ex.names(baseType(entry)) != nil {
				return true
			}
		}
	}
	return false
}

// baseType strips a `@face` suffix.
func baseType(entry string) string {
	typ, _, _ := strings.Cut(entry, entity.StateRefSeparator)
	return typ
}

// list expands each entry of a type list, keeping a `@face` suffix and the
// nil-versus-empty distinction (an omitted ceiling axis is nil).
func (ex typeExpander) list(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, entry := range in {
		typ := baseType(entry)
		suffix := entry[len(typ):]
		variants := ex.names(typ)
		if variants == nil {
			out = appendUnique(out, entry)
			continue
		}
		for _, v := range variants {
			out = appendUnique(out, v+suffix)
		}
	}
	return out
}

func (ex typeExpander) restriction(r Restriction) Restriction {
	r.Read, r.Create, r.Update, r.Delete =
		ex.list(r.Read), ex.list(r.Create), ex.list(r.Update), ex.list(r.Delete)
	r.DenyRead, r.DenyCreate, r.DenyUpdate, r.DenyDelete =
		ex.list(r.DenyRead), ex.list(r.DenyCreate), ex.list(r.DenyUpdate), ex.list(r.DenyDelete)
	r.DenyWrite = ex.list(r.DenyWrite)
	r.Visible = expandOverride(ex, r.Visible)
	r.Redact = expandUnionStrings(ex, r.Redact)
	return foldRedactIntoVisible(r)
}

// foldRedactIntoVisible handles a type that ends up with both a `visible:`
// and a `redact:` entry, which expansion can produce: a variant's own
// `visible:` plus the abstract type's `redact:`. A ceiling reads one
// spelling per type and the allowlist wins, so the redact would be lost
// and the field shown. The fields the redact names are removed from the
// allowlist instead, which keeps them hidden and leaves one spelling.
func foldRedactIntoVisible(r Restriction) Restriction {
	for typ, redacted := range r.Redact {
		visible, both := r.Visible[typ]
		if !both {
			continue
		}
		r.Visible[typ] = slices.DeleteFunc(slices.Clone(visible), func(f string) bool {
			return slices.Contains(redacted, f)
		})
		delete(r.Redact, typ)
	}
	return r
}

// expandUnion moves an abstract key's entries onto each variant, after the
// variant's own entries.
func expandUnion[T any](ex typeExpander, in map[string][]T) map[string][]T {
	if !hasAbstractKey(ex, in) {
		return in
	}
	out := make(map[string][]T, len(in))
	for key, grants := range in {
		if ex.names(key) == nil {
			out[key] = append(out[key], grants...)
		}
	}
	for _, key := range sortedKeys(in) {
		for _, v := range ex.names(key) {
			out[v] = append(out[v], in[key]...)
		}
	}
	return out
}

// expandUnionStrings is expandUnion for a field-name map, without
// duplicate names.
func expandUnionStrings(ex typeExpander, in map[string][]string) map[string][]string {
	out := expandUnion(ex, in)
	if len(out) == len(in) && !hasAbstractKey(ex, in) {
		return out
	}
	for key, fields := range out {
		var dedup []string
		for _, f := range fields {
			dedup = appendUnique(dedup, f)
		}
		out[key] = dedup
	}
	return out
}

// expandOverride copies an abstract key's entry to each variant that has no
// entry of its own.
func expandOverride[T any](ex typeExpander, in map[string][]T) map[string][]T {
	if !hasAbstractKey(ex, in) {
		return in
	}
	out := make(map[string][]T, len(in))
	for key, fields := range in {
		if ex.names(key) == nil {
			out[key] = fields
		}
	}
	for _, key := range sortedKeys(in) {
		for _, v := range ex.names(key) {
			if _, own := in[v]; !own {
				out[v] = slices.Clone(in[key])
			}
		}
	}
	return out
}

func hasAbstractKey[T any](ex typeExpander, in map[string]T) bool {
	for key := range in {
		if ex.names(key) != nil {
			return true
		}
	}
	return false
}

func appendUnique(out []string, s string) []string {
	if slices.Contains(out, s) {
		return out
	}
	return append(out, s)
}
