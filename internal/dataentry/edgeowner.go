package dataentry

import (
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ownedByFace reports whether rel may be served beside its SOURCE when that
// source is served at face. A content-scoped edge belongs to its tail face
// (Relation.FromFace); serving it beside another face would disclose content
// that face's grant withholds. Outgoing, the page's face must be the tail;
// incoming, the source's served face must be. Identity-scoped edges serve with
// every face, and the head's face never matters.
//
// Nil: meta is accepted; see [metamodel.IsContentScoped] for the verdict.
func ownedByFace(meta *metamodel.Metamodel, rel *entityPkg.Relation, face entityPkg.Face) bool {
	if !metamodel.IsContentScoped(meta, rel.Type) {
		return true
	}
	return rel.FromFace == face
}

// incomingOwnedAtZero keeps the incoming edges of row that a surface reading
// each neighbor at the ZERO coordinate may serve (export, list export, table
// relation columns). Only an edge the zero face owns qualifies, except a
// self-edge, whose source is row at its own face. It is defense in depth, not
// the gate: the neighbor read and visibleRelationIDs are.
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
