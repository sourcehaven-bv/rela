package visibility

import (
	"context"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// FaceSet is the set of faces of one type that a principal may read.
//
// It has three states: every face, a non-empty list, or none. The zero value
// is none, so a FaceSet that nobody filled in reads nothing.
//
// [FaceGate] cannot say "none": its empty slice means every face. A caller
// that turned an empty slice into a store predicate would therefore pass a
// nil FaceIn, which the store reads as every face, and fail open (RR-Z23T2T).
// FaceSet keeps the two meanings apart so "none" can stop a read before any
// query runs.
type FaceSet struct {
	all   bool
	faces []entity.Face
}

// AllFaces is the set holding every face of the type.
func AllFaces() FaceSet { return FaceSet{all: true} }

// SomeFaces is the set holding exactly faces. An empty list gives the empty
// set, never every face.
func SomeFaces(faces ...entity.Face) FaceSet {
	return FaceSet{faces: slices.Clone(faces)}
}

// NoFaces is the empty set.
func NoFaces() FaceSet { return FaceSet{} }

// FaceSetOf reads the face half of a composed read scope. A DenyAll scope
// reads no face; a nil Faces list reads every face, as
// [acl.ReadQueryResult.Faces] documents.
func FaceSetOf(r acl.ReadQueryResult) FaceSet {
	switch {
	case r.DenyAll:
		return NoFaces()
	case r.Faces == nil:
		return AllFaces()
	default:
		return SomeFaces(r.Faces...)
	}
}

// IsNone reports whether the set holds no face.
func (s FaceSet) IsNone() bool { return !s.all && len(s.faces) == 0 }

// Contains reports whether the set holds f.
func (s FaceSet) Contains(f entity.Face) bool {
	return s.all || slices.Contains(s.faces, f)
}

// QueryFaces is the set as a [store.EntityQuery.FaceIn] value: nil for every
// face, the list otherwise.
//
// The empty set has no FaceIn spelling, because nil means every face there
// too. Callers stop on [FaceSet.IsNone] before building a query; reaching
// here with the empty set is a bug, and the result would widen the read.
func (s FaceSet) QueryFaces() []entity.Face {
	if s.all {
		return nil
	}
	return slices.Clone(s.faces)
}

// FaceSetGate is the optional capability of a [RowGate] that can say "no
// face" directly. When a gate implements it, [ReadableFaces] prefers it over
// [FaceGate].
type FaceSetGate interface {
	ReadableFaces(ctx context.Context, entityType string) (FaceSet, error)
}

// ReadableFaces returns the faces of entityType that gate lets the ctx
// principal read. The [Resolver] uses it before any load.
//
//   - A [FaceSetGate] answers directly, and may answer "none".
//   - Otherwise the answer is the one [FaceAllowed] gives: a [FaceGate]
//     list, where an empty list means every face, or every face for a gate
//     that declares no faces.
//
// A gate error gives the empty set, so a failure hides rather than reveals.
func ReadableFaces(ctx context.Context, gate RowGate, entityType string) FaceSet {
	if sg, ok := gate.(FaceSetGate); ok {
		set, err := sg.ReadableFaces(ctx, entityType)
		if err != nil {
			return NoFaces()
		}
		return set
	}
	return faceGateSet(ctx, gate, entityType)
}

// faceGateSet is the face set a [FaceGate] reports, or every face for a gate
// that is not one. A FaceGate cannot say "none" except by failing.
func faceGateSet(ctx context.Context, gate RowGate, entityType string) FaceSet {
	fg, ok := gate.(FaceGate)
	if !ok {
		return AllFaces()
	}
	faces, err := fg.PermittedFaces(ctx, entityType)
	if err != nil {
		return NoFaces()
	}
	if len(faces) == 0 {
		return AllFaces()
	}
	return SomeFaces(faces...)
}
