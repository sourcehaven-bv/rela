package dataentry

import (
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ownedByFace reports whether rel may be served beside its SOURCE when that
// source is served at face (BUG-ISJHML).
//
// A content-scoped edge belongs to one face of its source: its tail
// (Relation.FromFace) names that face. Serving it beside any other face of
// the source presents one face's content as another's, and for a reader
// granted only the other face it discloses content the grant withholds. The
// rule therefore applies in both directions:
//
//   - outgoing: the page's own face must be the edge's tail;
//   - incoming: the source's served face (the face the world or an explicit
//     address resolves it to) must be the edge's tail.
//
// An identity-scoped edge belongs to the entity, so it is served with every
// face. The HEAD of an edge is entity-level, so the target's face never
// matters.
//
// Nil: meta is accepted; see [metamodel.IsContentScoped] for the verdict.
func ownedByFace(meta *metamodel.Metamodel, rel *entityPkg.Relation, face entityPkg.Face) bool {
	if !metamodel.IsContentScoped(meta, rel.Type) {
		return true
	}
	return rel.FromFace == face
}

// incomingOwnedAtZero keeps the incoming edges of row that may be served on a
// surface which reads each neighbor at the ZERO coordinate (a bare-id read:
// export, list export, table relation columns). The relations routes serve
// an edge editor instead, and name each edge's face (App.readableIncoming).
//
// Such a surface serves the source at its zero face, so only an edge that
// face owns may be served; a content edge tailed at any other face would be
// presented as the zero face's. The exception is a self-edge, whose source
// is row itself, served at row's own face.
//
// Because a bare-id read never resolves a faced source, this filter reduces
// in practice to "the source is faceless". It is defense in depth for these
// surfaces, not their gate: the neighbor read and visibleRelationIDs are.
func incomingOwnedAtZero(
	meta *metamodel.Metamodel, rels []*entityPkg.Relation, row *entityPkg.Entity,
) []*entityPkg.Relation {
	out := rels[:0:0]
	for _, rel := range rels {
		if ownedByFace(meta, rel, incomingSourceFace(rel, row.Face)) {
			out = append(out, rel)
		}
	}
	return out
}

// incomingSourceFace is the face at which a zero-coordinate surface serves
// the source of an incoming edge on a row served at rowFace; see
// [incomingOwnedAtZero].
func incomingSourceFace(rel *entityPkg.Relation, rowFace entityPkg.Face) entityPkg.Face {
	if rel.From == rel.To {
		return rowFace
	}
	return ""
}

// edgesOwnedBy keeps the edges of rels whose source, served at face, owns
// them. Every edge in rels must share that served source face: the outgoing
// edges of one entity, or incoming edges whose sources are all read at the
// same coordinate.
func edgesOwnedBy(
	meta *metamodel.Metamodel, rels []*entityPkg.Relation, face entityPkg.Face,
) []*entityPkg.Relation {
	out := rels[:0:0]
	for _, rel := range rels {
		if ownedByFace(meta, rel, face) {
			out = append(out, rel)
		}
	}
	return out
}
