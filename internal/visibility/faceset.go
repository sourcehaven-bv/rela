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
// too, so it reports ok=false and the caller must not query at all. Making
// the caller branch keeps the one wrong use from widening the read.
func (s FaceSet) QueryFaces() (faces []entity.Face, ok bool) {
	if s.all {
		return nil, true
	}
	if len(s.faces) == 0 {
		return nil, false
	}
	return slices.Clone(s.faces), true
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
// A gate error is returned with the empty set, so a caller that ignores it
// still hides rather than reveals. The [Resolver] returns it, like a row-gate
// error.
func ReadableFaces(ctx context.Context, gate RowGate, entityType string) (FaceSet, error) {
	if sg, ok := gate.(FaceSetGate); ok {
		set, err := sg.ReadableFaces(ctx, entityType)
		if err != nil {
			return NoFaces(), err
		}
		return set, nil
	}
	fg, ok := gate.(FaceGate)
	if !ok {
		return AllFaces(), nil
	}
	faces, err := fg.PermittedFaces(ctx, entityType)
	if err != nil {
		return NoFaces(), err
	}
	return faceListSet(faces), nil
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
	return faceListSet(faces)
}

// faceListSet reads a [FaceGate] list: empty means every face.
func faceListSet(faces []entity.Face) FaceSet {
	if len(faces) == 0 {
		return AllFaces()
	}
	return SomeFaces(faces...)
}

// VerdictSet is the face set one [acl.FaceVerdict] reads.
func VerdictSet(v acl.FaceVerdict) FaceSet {
	switch {
	case v.All():
		return AllFaces()
	case v.None():
		return NoFaces()
	default:
		return SomeFaces(v.Faces()...)
	}
}

// Intersect returns the faces both sets hold.
func (s FaceSet) Intersect(o FaceSet) FaceSet {
	switch {
	case s.all:
		return o.clone()
	case o.all:
		return s.clone()
	}
	var out []entity.Face
	for _, f := range s.faces {
		if slices.Contains(o.faces, f) {
			out = append(out, f)
		}
	}
	return FaceSet{faces: out}
}

// clone returns s with its own face list.
func (s FaceSet) clone() FaceSet {
	return FaceSet{all: s.all, faces: slices.Clone(s.faces)}
}
