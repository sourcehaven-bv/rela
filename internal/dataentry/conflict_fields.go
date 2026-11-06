package dataentry

import (
	"context"
	"errors"
	"reflect"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/conflict"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// errConflictSourceUnreadable is returned by [conflictFieldDenial] when a
// relation's source has no readable row. The meta check would pass such a
// row unjudged, so the resolve is refused as not found instead.
var errConflictSourceUnreadable = errors.New("conflict: relation source has no readable row")

// conflictFieldWrite returns the property writes a resolution makes. A
// property the two sides disagree on is written whatever the caller chose:
// set to the resolved value, or unset when the resolved side lacks it. A
// property both sides agree on keeps its value, so it is no write.
func conflictFieldWrite(ours, theirs, resolved map[string]any) (set map[string]any, unset []string) {
	set = map[string]any{}
	for _, props := range []map[string]any{ours, theirs} {
		for name := range props {
			ov, inOurs := ours[name]
			tv, inTheirs := theirs[name]
			if inOurs == inTheirs && reflect.DeepEqual(ov, tv) {
				continue
			}
			if v, ok := resolved[name]; ok {
				set[name] = v
			} else if !slices.Contains(unset, name) {
				unset = append(unset, name)
			}
		}
	}
	slices.Sort(unset)
	return set, unset
}

// sideProperties returns the properties of one parsed side, or nil for an
// absent side (the file was deleted on that branch).
func sideProperties(side *conflict.ParsedSide) map[string]any {
	switch {
	case side == nil:
		return nil
	case side.Entity != nil:
		return side.Entity.Properties
	case side.Relation != nil:
		return side.Relation.Properties
	}
	return nil
}

// resolvedProperties returns the properties of the resolved row.
func resolvedProperties(e *entityPkg.Entity, rel *entityPkg.Relation) map[string]any {
	if rel != nil {
		return rel.Properties
	}
	if e != nil {
		return e.Properties
	}
	return nil
}

// conflictFieldDenial runs the field write check a PATCH runs on the
// properties a resolution writes (BUG-197XPAU). It returns the first denial
// and the row it was judged on, which the handler answers with the PATCH's
// 403 and audit row, so a caller cannot decide a field's value by choosing a
// side.
//
// An entity's verdicts are judged on each parsed side, as the row at the
// file's face, because a `when:` can read either side's values; a field
// either side denies is refused. A relation's meta is judged on the edge's
// source rows, as a relation update judges it. A source with no readable row
// returns [errConflictSourceUnreadable] rather than being judged permissive.
// It runs after the read gate and the row write check have passed.
//
// cf: Nil: rejected by the caller; conflict.Resolve has parsed it. Exactly
// one of e and rel is non-nil, the resolved row.
//
//archplan:guard=authz reason="field write check for the properties a conflict resolution writes"
func conflictFieldDenial(
	ctx context.Context, svc affordanceService, id conflictIdentity, cf *conflict.ConflictedFile,
	e *entityPkg.Entity, rel *entityPkg.Relation,
) (*entityPkg.Entity, *AffordanceDenialError, error) {
	set, unset := conflictFieldWrite(sideProperties(cf.Ours), sideProperties(cf.Theirs), resolvedProperties(e, rel))
	if rel != nil {
		sources, err := svc.relationSources(ctx, nil,
			entityPkg.Ref{ID: rel.From, Face: rel.FromFace}, string(DirectionIncoming), rel.Type)
		if err != nil {
			return nil, nil, err
		}
		for _, src := range sources {
			if src.row == nil {
				return nil, nil, errConflictSourceUnreadable
			}
		}
		set, unset = governedRelationMeta(ctx, svc, sources, rel.Type, set, unset)
		subject, denial := svc.relationMetaDenial(ctx, sources, rel.Type, set, unset)
		return subject, denial, nil
	}
	for _, side := range conflictSides(cf) {
		if side.Entity == nil {
			continue
		}
		row := side.Entity.Clone()
		row.ID, row.Type, row.Face = id.file.ID, id.file.Type, id.file.Face
		if denial := conflictEntityFieldDenial(ctx, svc, row, set, unset); denial != nil {
			return row, denial, nil
		}
	}
	return nil, nil, nil
}

// conflictEntityFieldDenial is [affordanceService.validateFieldWrite] for a
// conflict resolution. It leaves out a property the schema does not declare
// and no field rule names. A PATCH refuses such a name, so a caller cannot
// probe which names exist (F8). Here the name comes from the file, which the
// caller has already been shown, and no `writable:` or `visible:` rule
// governs it. Refusing it would make the conflict unresolvable for every
// principal.
func conflictEntityFieldDenial(
	ctx context.Context, svc affordanceService, e *entityPkg.Entity, set map[string]any, unset []string,
) *AffordanceDenialError {
	v := svc.fieldVerdicts(ctx, e)
	declared := affordances.DeclaredProperties(svc.meta(), e.Type)
	governed := func(name string) bool {
		_, w := v.Writable[name]
		_, vis := v.Visible[name]
		_, opt := v.Options[name]
		return declared[name] || w || vis || opt
	}
	keptSet := make(map[string]any, len(set))
	for k, val := range set {
		if governed(k) {
			keptSet[k] = val
		}
	}
	keptUnset := slices.DeleteFunc(slices.Clone(unset), func(k string) bool { return !governed(k) })
	return checkFieldWrite(svc.meta(), e, v, keptSet, keptUnset)
}

// governedRelationMeta is [conflictEntityFieldDenial]'s rule for relation
// meta: it keeps only the meta keys some source's verdict lists. Under a
// meta allowlist the verdict lists every declared key and every key a grant
// names, so this leaves out exactly the undeclared keys no grant names. A
// PATCH refuses such a key, but here the key comes from the file, and
// refusing it would make the conflict unresolvable for every principal the
// allowlist applies to. Without an allowlist every key is writable, so
// leaving keys out changes nothing.
func governedRelationMeta(
	ctx context.Context, svc affordanceService, sources []relationSource, relType string,
	set map[string]any, unset []string,
) (keptSet map[string]any, keptUnset []string) {
	governed := map[string]bool{}
	for _, src := range sources {
		for name := range svc.verdictsOf(ctx, src).Types[relType].Fields {
			governed[name] = true
		}
	}
	keptSet = make(map[string]any, len(set))
	for k, val := range set {
		if governed[k] {
			keptSet[k] = val
		}
	}
	keptUnset = slices.DeleteFunc(slices.Clone(unset), func(k string) bool { return !governed[k] })
	return keptSet, keptUnset
}
