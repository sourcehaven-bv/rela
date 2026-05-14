package dataentry

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// resolveDirection maps a body key from the modern PATCH wire format
// to its canonical relation type plus a flag indicating whether the
// path entity is on the TARGET side of the underlying canonical edge
// (incoming) or the SOURCE side (outgoing).
//
// Precedence: a canonical relation name always wins. Inverse-name
// lookup happens only when the body key isn't a canonical relation.
// The load-time validation in TKT-4VLN guarantees that an inverse
// name cannot also be a canonical name (other than the symmetric
// self-inverse case), so this precedence is unambiguous at runtime.
//
// Symmetric relations (`symmetric: true`) where the inverse name
// equals the canonical name are always reported as outgoing — the
// path entity is the source by convention. Callers that want to
// inspect symmetry directly can read `relDef.Symmetric` after
// looking up the canonical name.
//
// Returns:
//
//   - canonical: the canonical relation type, suitable for looking
//     up `meta.Relations[canonical]`.
//   - incoming: true iff the body key was an inverse alias and the
//     relation is NOT symmetric. False for canonical names and for
//     symmetric self-inverse keys.
//   - ok: true iff the body key resolved to a known relation type.
//     When false, the caller should surface a structural error
//     (`unknown_relation_type` if the name matches no canonical name
//     and isn't a known inverse; `no_inverse_defined` if it looks
//     like an attempted inverse alias for a relation without one).
func resolveDirection(meta *metamodel.Metamodel, bodyKey string) (canonical string, incoming, ok bool) {
	if _, isCanonical := meta.Relations[bodyKey]; isCanonical {
		return bodyKey, false, true
	}
	owner, hasInverse := meta.InverseOwner(bodyKey)
	if !hasInverse {
		return "", false, false
	}
	// Symmetric self-inverse: path entity is source by convention,
	// not target. Callers should treat this as outgoing for write
	// purposes; the metamodel's `symmetric: true` flag tells the
	// reconciler the edge has no preferred direction.
	if relDef, hasRel := meta.Relations[owner]; hasRel && relDef.Symmetric {
		return owner, false, true
	}
	return owner, true, true
}

// edgeEndpoints resolves the (from, to) endpoints of an edge written
// against `entityID`, given a peer ID and a direction flag. Centralizes
// the source/target flip so callers don't repeat it inline.
//
// When incoming, the peer is the source and the path entity is the
// target; the canonical edge on disk is `peerID --relType--> entityID`.
func edgeEndpoints(entityID, peerID string, incoming bool) (from, to string) {
	if incoming {
		return peerID, entityID
	}
	return entityID, peerID
}

// detectSelfLoopShapeConflict returns a structural error when the
// body contains BOTH the canonical name AND its inverse for the same
// canonical relation, AND the path entity appears in the desired set
// of both keys. That refers to the same physical self-loop edge twice
// and is rejected so the client can fix its intent.
//
// Symmetric relations (which by design have no preferred direction)
// are exempt: a symmetric self-loop is just a single edge regardless
// of how it's named.
func detectSelfLoopShapeConflict(
	meta *metamodel.Metamodel, entityID string, desired map[string]V1RelationsUpdate,
) error {
	// Group body keys by the canonical relation they resolve to,
	// collecting whether they were submitted as canonical or inverse.
	type usage struct {
		canonicalKeys []string
		inverseKeys   []string
	}
	byCanonical := map[string]*usage{}

	for bodyKey, upd := range desired {
		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			continue
		}
		if relDef, hasRel := meta.Relations[canonical]; hasRel && relDef.Symmetric {
			continue
		}
		// `incoming=true` here means the body key was an inverse alias
		// of a non-symmetric relation. (Canonical names and symmetric
		// self-inverse both report incoming=false.)
		entry := byCanonical[canonical]
		if entry == nil {
			entry = &usage{}
			byCanonical[canonical] = entry
		}
		if incoming {
			entry.inverseKeys = append(entry.inverseKeys, bodyKey)
		} else {
			entry.canonicalKeys = append(entry.canonicalKeys, bodyKey)
		}
		// `upd` not needed once we know which buckets to populate.
		_ = upd
	}

	for canonical, u := range byCanonical {
		if len(u.canonicalKeys) == 0 || len(u.inverseKeys) == 0 {
			continue
		}
		// Both shapes present for the same canonical relation. Check
		// the intersection of their desired sets against the path
		// entity — that's the only collision surface.
		conflictKeys := []string{u.canonicalKeys[0], u.inverseKeys[0]}
		if pathEntityInBothSets(entityID, conflictKeys, desired) {
			return &wireError{
				Code: "shape_conflict",
				Path: "/relations/" + jsonPointerEscape(conflictKeys[0]),
				Detail: fmt.Sprintf(
					"relation %q is referenced via both %q and its inverse %q with the path entity on both sides — "+
						"the body refers to the same self-loop edge twice; remove one of the keys",
					canonical, conflictKeys[0], conflictKeys[1]),
			}
		}
	}
	return nil
}

// pathEntityInBothSets returns true when entityID appears in the
// desired-set of every body key in keys.
func pathEntityInBothSets(entityID string, keys []string, desired map[string]V1RelationsUpdate) bool {
	for _, k := range keys {
		upd, ok := desired[k]
		if !ok {
			return false
		}
		found := false
		for _, ref := range upd.Data {
			if ref.ID == entityID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// currentEdgesByPeer returns the existing edges of the given canonical
// relation type that touch `entityID`, keyed by the peer ID (the other
// end of each edge relative to `entityID`). The direction flag picks
// whether the path entity is on the source (outgoing) or target
// (incoming) side. This is the read-side mirror of edgeEndpoints.
func (a *App) currentEdgesByPeer(entityID, canonical string, incoming bool) map[string]*entity.Relation {
	current := map[string]*entity.Relation{}
	var edges []*entity.Relation
	if incoming {
		edges = a.incomingRelations(entityID)
	} else {
		edges = a.outgoingRelations(entityID)
	}
	for _, edge := range edges {
		if edge.Type != canonical {
			continue
		}
		peerID := edge.To
		if incoming {
			peerID = edge.From
		}
		current[peerID] = edge
	}
	return current
}
