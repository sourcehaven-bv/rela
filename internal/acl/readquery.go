package acl

import (
	"context"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ReadQueryResult is the response from Request.ReadQuery. Exactly one
// of (AllowAll, DenyAll, Query) is meaningful:
//
//	AllowAll → caller runs an unfiltered list of EntityType.
//	DenyAll  → caller returns an empty list of EntityType.
//	Query    → caller runs the composed store.GraphQuery to filter.
type ReadQueryResult struct {
	AllowAll bool
	DenyAll  bool
	Query    *store.GraphQuery

	// Faces narrows the result to these content states. Nil means EVERY
	// face — the meaning for an unfaced project, for a `read: ["*"]`
	// wildcard, and for every pre-faces acl.yaml, so those paths push no
	// predicate down at all (TKT-O7R2A1).
	//
	// It rides HERE rather than only on Query because the AllowAll branch
	// never builds a Query: listPushdown short-circuits it straight to
	// ListEntities. A face set carried only on Query would therefore filter
	// nothing for exactly the most privileged principals — the same
	// list/single-entity divergence GraphQuery.World's doc warns about.
	//
	// The UNION across granting roles, not the intersection: grants are
	// additive (DEC-RG878), so holding two roles reads what either allows.
	Faces []entity.Face
}

// readQuery composes a ReadQueryResult. AllowAll when any effective
// global role grants read on entityType; otherwise compose a
// GraphQuery whose HasInbound predicate matches entities reachable
// via the role-relations whose confers-role grants read on the type.
// DenyAll when no role grants any kind of read.
func (r *Request) readQuery(ctx context.Context, entityType string) ReadQueryResult {
	// A client ceiling that denies reads of this type short-circuits every
	// grant path below. Checked first because the ceiling can only ever remove:
	// if it says no, no role — global, conferred or inherited — can say yes.
	// This also closes the wildcard gap roleFor cannot express in a plain list
	// (a role holding `read: ["*"]` under `deny_read: [person]` keeps its
	// wildcard; see filterTypes).
	if !r.ceiling.permitsRead(entityType) {
		return ReadQueryResult{DenyAll: true}
	}

	globals := r.Globals(ctx)
	var (
		anyGlobal bool
		faceUnion []entity.Face
		allFaces  bool
	)
	for _, a := range globals.Attributions {
		role, ok := r.roleFor(a.Role)
		if !ok {
			continue
		}
		if !roleGrantsRead(role, entityType) {
			continue
		}
		anyGlobal = true
		fs, all := roleReadFaces(role, entityType)
		if all {
			allFaces = true
		}
		faceUnion = append(faceUnion, fs...)
	}
	if anyGlobal {
		if allFaces {
			return ReadQueryResult{AllowAll: true}
		}
		return ReadQueryResult{AllowAll: true, Faces: dedupeFaces(faceUnion)}
	}

	var conferring []string
	// Per conferring relation: the faces ITS role grants (nil = every face).
	// The union over all conferring roles is what the type-level Faces and
	// FaceIn carry; the per-relation sets become GraphQuery.Any branches
	// below, because a principal who reaches an entity through ONE relation
	// holds that relation's role and no other — a union would launder the
	// owner's draft grant through the reviewer's edge.
	facesByRel := map[string][]entity.Face{}
	allByRel := map[string]bool{}
	// The face union is recomputed over the CONFERRING roles, not inherited
	// from the global loop above: a role conferred by a relation grants what
	// IT declares, and the global loop found nothing (we are past its
	// early return). Reusing the global union here would grant faces from
	// roles the principal does not hold on this path.
	faceUnion, allFaces = nil, false
	for relType, def := range r.d.policy.RoleRelations {
		role, ok := r.roleFor(def.Confers)
		if !ok {
			continue
		}
		if !roleGrantsRead(role, entityType) {
			continue
		}
		conferring = append(conferring, relType)
		fs, all := roleReadFaces(role, entityType)
		if all {
			allFaces = true
		}
		faceUnion = append(faceUnion, fs...)
		facesByRel[relType] = dedupeFaces(fs)
		allByRel[relType] = all
	}
	if len(conferring) == 0 {
		return ReadQueryResult{DenyAll: true}
	}
	sort.Strings(conferring)

	// Fail closed on an empty member set. `Endpoints: nil` means "ANY
	// endpoint" to store.GraphQuery (the absence-query widening), so
	// handing it an empty set would degrade this gate from "reachable
	// from ME" to "reachable from ANYONE" — a read bypass, not a
	// narrowing. Today walkMembers only returns empty for an unstamped
	// principal, which Declarative.ForPrincipal already rejects; this
	// guard states the invariant HERE so the gate does not depend on a
	// validation living in another file to stay safe.
	if len(globals.Members) == 0 {
		return ReadQueryResult{DenyAll: true}
	}

	// World is deliberately left ZERO here, and that is CORRECT rather
	// than a gap: the world is stamped onto this query by the caller,
	// at internal/visibility's listPushdown seam, which copies it from
	// the EntityQuery it was given (TKT-WAV8XP PR-D).
	//
	// internal/acl structurally cannot resolve a world itself — arch-lint
	// forbids it from importing internal/metamodel, and a WorldScope is
	// compiled from the metamodel. So the scope belongs to the layer
	// above, which is why this composer emits a world-free query and the
	// wiring seam stamps it.
	//
	// Do NOT "fix" this by giving internal/acl a metamodel dependency:
	// that breaks the boundary keeping ACL evaluable without a schema.
	// And do NOT drop the copy at listPushdown — pushdown reaches past
	// every decorator to the raw store, so without it a world-scoped
	// list silently degrades to the default world for exactly the
	// ACL-gated principals (RR-GQWRLD).
	q := &store.GraphQuery{
		EntityType: entityType,
		HasInbound: &store.RelationPredicate{
			// Endpoints is the principal's group-member set, already
			// expanded by walkMembers over the *configured* membership
			// relation (Policy.membershipRelation). This is the deliberate
			// seam: group expansion happens once, upstream, so this read
			// path inherits the configured relation for free. Do NOT inline
			// the membership relation here as another RelationPredicate —
			// that would re-hardcode "member-of" and bypass the single
			// accessor guard (TKT-Z8A62F).
			Endpoints: globals.Members,
			OfTypes:   conferring,
		},
	}
	if len(r.d.policy.InheritRolesThrough) > 0 {
		q.HasInbound.EntityInheritThrough = append([]string(nil), r.d.policy.InheritRolesThrough...)
		q.HasInbound.EntityDepth = depthCap
	}
	if allFaces && sameFacesEverywhere(conferring, facesByRel, allByRel) {
		// Every conferring role reads every face: the union IS each role's
		// grant, so the single predicate is exact and no branches are needed.
		return ReadQueryResult{Query: q}
	}
	// The roles disagree on faces, so the query says which faces each
	// RELATION grants. HasInbound stays as the union (a backend that ignores
	// Any narrows at least to the reachable set; the conformance suite pins
	// that none does), and Faces stays the union for the AllowAll-shaped
	// consumers that never see a query.
	for _, relType := range conferring {
		br := store.GraphBranch{HasInbound: &store.RelationPredicate{
			Endpoints: globals.Members, OfTypes: []string{relType},
		}}
		if len(r.d.policy.InheritRolesThrough) > 0 {
			br.HasInbound.EntityInheritThrough = append([]string(nil), r.d.policy.InheritRolesThrough...)
			br.HasInbound.EntityDepth = depthCap
		}
		if !allByRel[relType] {
			br.FaceIn = facesByRel[relType]
		}
		q.Any = append(q.Any, br)
	}
	if allFaces {
		return ReadQueryResult{Query: q}
	}
	return ReadQueryResult{Query: q, Faces: dedupeFaces(faceUnion)}
}

// sameFacesEverywhere reports whether every conferring relation grants every
// face — the only case in which the union predicate is exact on its own.
func sameFacesEverywhere(conferring []string, _ map[string][]entity.Face, allByRel map[string]bool) bool {
	for _, rel := range conferring {
		if !allByRel[rel] {
			return false
		}
	}
	return true
}

// roleGrantsRead reports whether `role.Read` covers `target` — exact
// match or wildcard `"*"`.
func roleGrantsRead(role RoleDef, target string) bool {
	for _, t := range role.Read {
		if t == "*" || t == target {
			return true
		}
		// A face-scoped grant grants its TYPE for the purpose of this
		// predicate: `read: [policy@published]` means the role reads
		// policies, and WHICH faces is answered separately by
		// roleReadFaces. Without this a face grant alone would compose
		// no query at all and the role would read nothing (TKT-O7R2A1).
		if typ, _, isState, err := parseStateGrant(t); err == nil && isState && typ == target {
			return true
		}
	}
	return false
}

// roleReadFaces returns the faces of target this role may read, or nil
// meaning EVERY face (TKT-O7R2A1).
//
// # A bare type grant reads EVERY face, unlike the write side
//
// `read: [policy]` grants every face; only an explicit `read: [policy@nl]`
// narrows. That is the opposite of `update: [policy]`, which grants the default
// face alone, and the asymmetry is deliberate.
//
// Fail-closed was tried first and is WRONG here, for a reason worlds make
// concrete: a bare grant would mean "the default face only", but a world
// resolves each entity to whichever face its chain selects — commonly
// `published`, never the default. So a bare grant under a world would read
// NOTHING, not "less". Every existing world deployment goes dark, which is not
// a clear-and-easy-to-fix denial but a total outage. The parity test
// (TestWorldListGetParity_ACLGatedPrincipal) demonstrates it directly.
//
// The write side has no such interaction: writes address a face by id and never
// pass through a world, so narrowing a bare write grant to the default face
// costs an operator one explicit grant rather than all access.
//
// # What this still buys
//
// The leak it closes is the one an operator actually reaches for: naming
// `read: [policy@published]` now means that face and no other, so a reader can
// be given the published text without the draft. Before this, no grant could
// express that at all. What it does not do is retroactively tighten a config
// that never named a face — and `rela acl audit` reports those, so an operator
// auditing for confidentiality is told which roles read every face.
//
// Returning nil rather than an enumerated set keeps the query untouched for
// every unfaced project and every un-narrowed grant: no predicate is pushed
// down, so the pre-faces fast path is byte-identical.
func roleReadFaces(role RoleDef, target string) (faces []entity.Face, all bool) {
	for _, t := range role.Read {
		if t == "*" {
			return nil, true
		}
		if t == target {
			// A bare type grant addresses EVERY face — see the doc above for
			// why this differs from the write side.
			return nil, true
		}
		typ, p, isState, err := parseStateGrant(t)
		if err != nil || !isState || typ != target {
			continue
		}
		faces = append(faces, p)
	}
	return faces, false
}

// dedupeFaces removes duplicates and sorts, so a face set is stable across
// requests regardless of role iteration order — a query that reshuffles its
// arguments between identical requests defeats statement caching and makes a
// diff of two runs unreadable.
//
// Nil in, nil out: an empty set means "no face granted", which the caller
// turns into DenyAll rather than an empty predicate that would match nothing
// silently.
func dedupeFaces(in []entity.Face) []entity.Face {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[entity.Face]struct{}, len(in))
	out := make([]entity.Face, 0, len(in))
	for _, f := range in {
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	slices.Sort(out)
	return out
}
