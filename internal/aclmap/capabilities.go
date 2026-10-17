package aclmap

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Extras asks a per-principal map to also report the capabilities that are
// not entity verbs: named permissions and relation writes. Together with the
// entity verbs they are everything an acl.yaml grant can confer, which is what
// a CI gate on a principal's access has to compare.
//
// The engine has no metamodel or data-entry dependency, so the caller supplies
// both universes. The zero value asks for nothing extra.
type Extras struct {
	// Permissions are the permission names to probe. Duplicates and "*" are
	// ignored; a role's `permissions: ["*"]` never matched a named permission
	// at runtime either.
	Permissions []string
	// Relations are the relation types to probe, each with the entity types
	// an edge of it may start from.
	Relations []RelationType
}

// RelationType is one relation type of the schema: its name and the entity
// types its edges may start from.
type RelationType struct {
	Name string
	From []string
}

// RelationAccess is one relation type the principal may write, with the
// verbs it holds and the source types each verb is held from.
type RelationAccess struct {
	Relation string `json:"relation"`
	// Verbs maps each held verb (create, update, delete) to the source entity
	// types it is held from, sorted.
	Verbs map[string][]string `json:"verbs"`
}

// isZero reports whether no extra capability was requested.
func (x Extras) isZero() bool {
	return len(x.Permissions) == 0 && len(x.Relations) == 0
}

// MapPrincipalFull is [Engine.MapPrincipalAs] plus the capabilities extras
// asks for: the permissions the principal holds and the relation types it may
// write. Both are decided by the runtime gate itself
// ([acl.Request.HoldsPermission], [acl.Request.AuthorizeWrite]), under the
// same client ceiling as the entity verbs.
func (e *Engine) MapPrincipalFull(
	ctx context.Context, rawPrincipal string, verbFilter acl.Verb, typeFilter string,
	entityTypes []string, view ClientView, extras Extras,
) (*MapPrincipalResult, error) {
	verbs, err := resolveVerbs(verbFilter)
	if err != nil {
		return nil, err
	}
	user, rawShown, err := e.resolveEffective(ctx, rawPrincipal)
	if err != nil {
		return nil, err
	}
	if user == "" {
		// See MapPrincipalAs: a blank key maps to nothing rather than failing
		// a whole-graph inventory.
		return &MapPrincipalResult{
			SchemaVersion: schemaVersion,
			Verbs:         verbStrings(verbs),
			EveryoneOnly:  true,
		}, nil
	}
	p := principal.Principal{User: user, Tool: principal.ToolCLI, RawUser: rawShown}
	if !view.isZero() {
		// Stamp the attenuation claims through the verified constructor — the
		// only path that can populate them — so the ceiling this map reports is
		// the one the request path would compute.
		p = principal.VerifiedFrom(user, principal.ToolCLI, principal.Claims{
			PrincipalType: view.PrincipalType,
			Scopes:        view.Scopes,
		})
		p.RawUser = rawShown
	}
	result := &MapPrincipalResult{
		SchemaVersion: schemaVersion,
		Principal:     user,
		Raw:           rawShown,
		As:            view.PrincipalType,
		Scopes:        view.Scopes,
	}
	return e.mapFor(ctx, p, result, verbs, typeFilter, entityTypes, extras)
}

// mapFor fills result with p's entity access and the requested extras.
// result carries the identity fields the caller already decided.
func (e *Engine) mapFor(
	ctx context.Context, p principal.Principal, result *MapPrincipalResult,
	verbs []acl.Verb, typeFilter string, entityTypes []string, extras Extras,
) (*MapPrincipalResult, error) {
	req, err := e.requestFor(ctx, p)
	if err != nil {
		return nil, err
	}
	result.Verbs = verbStrings(verbs)
	result.Baseline = acl.CeilingBaseline(req)
	result.EveryoneOnly = true // cleared as soon as a non-everyone route is found

	for _, typ := range mapTypes(typeFilter, entityTypes) {
		ta, sawNonEveryone, err := e.typeAccess(ctx, req, typ, verbs)
		if err != nil {
			return nil, err
		}
		if sawNonEveryone {
			result.EveryoneOnly = false
		}
		if len(ta.Baseline) > 0 || len(ta.Exceptions) > 0 || len(ta.Faces) > 0 {
			result.Types = append(result.Types, ta)
		}
	}
	if extras.isZero() {
		return result, nil
	}
	result.Permissions = heldPermissions(ctx, req, extras.Permissions)
	result.Relations = e.writableRelations(ctx, req, extras.Relations)
	return result, nil
}

// heldPermissions returns the names in universe the principal holds, sorted.
func heldPermissions(ctx context.Context, req *acl.Request, universe []string) []string {
	var held []string
	for _, perm := range dedupeSorted(universe) {
		if perm == "*" {
			continue
		}
		if req.HoldsPermission(ctx, perm) {
			held = append(held, perm)
		}
	}
	return held
}

// relationOps are the relation write verbs in report order.
var relationOps = []struct {
	verb acl.Verb
	op   acl.Op
}{
	{acl.VerbCreate, acl.OpCreate},
	{acl.VerbUpdate, acl.OpUpdate},
	{acl.VerbDelete, acl.OpDelete},
}

// writableRelations asks the write gate, per relation type, source type and
// verb, whether the principal may write such an edge. The subject is the one
// can-relation builds for an entity with no named face: a faced source type
// is authorized on every face it declares, as an identity-scoped edge is.
//
// Relation writes are decided on the principal's global roles only (see
// acl's authorizeRelationWrite), so no entity is needed and the answer holds
// for every edge of that shape.
func (e *Engine) writableRelations(ctx context.Context, req *acl.Request, rels []RelationType) []RelationAccess {
	sorted := slices.Clone(rels)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var out []RelationAccess
	for _, rt := range sorted {
		verbs := map[string][]string{}
		for _, from := range dedupeSorted(rt.From) {
			for _, ro := range relationOps {
				d := req.AuthorizeWrite(ctx, acl.WriteRequest{Op: ro.op, Subject: acl.RelationSubject{
					Type:        rt.Name,
					FromType:    from,
					FamilyFaces: e.faces(from),
				}})
				if d.Allow {
					verbs[string(ro.verb)] = append(verbs[string(ro.verb)], from)
				}
			}
		}
		if len(verbs) > 0 {
			out = append(out, RelationAccess{Relation: rt.Name, Verbs: verbs})
		}
	}
	return out
}

// dedupeSorted returns the distinct non-empty values of in, sorted.
func dedupeSorted(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return slices.Compact(out)
}
