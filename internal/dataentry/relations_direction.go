package dataentry

import (
	"fmt"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// resolveDirection maps a PATCH body key to its canonical relation type and
// reports whether the path entity is the target (incoming) of that edge. A
// canonical name wins over an inverse name; load-time validation keeps them
// disjoint. A symmetric self-inverse key is reported as outgoing. ok is false
// when the key names no known relation; the caller then reports
// `unknown_relation_type` or `no_inverse_defined`.
func resolveDirection(meta *metamodel.Metamodel, bodyKey string) (canonical string, incoming, ok bool) {
	if _, isCanonical := meta.Relations[bodyKey]; isCanonical {
		return bodyKey, false, true
	}
	owner, hasInverse := meta.InverseOwner(bodyKey)
	if !hasInverse {
		return "", false, false
	}
	if relDef, hasRel := meta.Relations[owner]; hasRel && relDef.Symmetric {
		return owner, false, true
	}
	return owner, true, true
}

// inverseRelationKey returns the wire key under which INCOMING edges of a
// relation type are serialized. Incoming edges are keyed by the relation's
// inverse name so they never collide with the same relation used outgoing (a
// row can carry both `implements` outgoing targets and `implements_inverse`
// incoming sources). When the metamodel declares an explicit inverse
// (`relDef.Inverse.ID`), that name is used; otherwise the synthetic
// `<type>_inverse` fallback applies. This is the single source of truth for
// the key — the per-entity relations handler (handleV1EntityRelations), the
// list-row serializer (entitySerializer.toV1), and the SPA's
// getFormattedCellValue must all agree on it. See MECHANISM.md.
func inverseRelationKey(relType string, relDef metamodel.RelationDef) string {
	if relDef.Inverse != nil && relDef.Inverse.ID != "" {
		return relDef.Inverse.ID
	}
	return relType + "_inverse"
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
	meta *metamodel.Metamodel, entityID string, desired map[string]v1.RelationsUpdate,
) error {
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
		_ = upd
	}

	for canonical, u := range byCanonical {
		if len(u.canonicalKeys) == 0 || len(u.inverseKeys) == 0 {
			continue
		}
		conflictKeys := []string{u.canonicalKeys[0], u.inverseKeys[0]}
		if pathEntityInBothSets(entityID, conflictKeys, desired) {
			return &v1.WireError{
				Code: "shape_conflict",
				Path: "/relations/" + v1.JSONPointerEscape(conflictKeys[0]),
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
func pathEntityInBothSets(entityID string, keys []string, desired map[string]v1.RelationsUpdate) bool {
	for _, k := range keys {
		upd, ok := desired[k]
		if !ok {
			return false
		}
		found := false
		for _, ref := range upd.Upserts() {
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
