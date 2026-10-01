package acl

import (
	"context"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// FaceMatcher runs a read verdict against stored rows and reports, per id,
// the faces whose row matches. The store implements it; see
// [store.GraphQueryer.MatchingFaces] for the contract.
type FaceMatcher interface {
	MatchingFaces(ctx context.Context, q store.GraphQuery, ids []string) (map[string][]entity.Face, error)
}

// FaceVerdict is the set of one entity's faces a principal may read.
//
// It has three explicit states: every face, a non-empty list, or none. There
// is no nil-means-every-face shape, so an empty list can never widen a read.
// The zero value is none: a verdict nobody filled in reads nothing.
type FaceVerdict struct {
	all   bool
	faces []entity.Face
}

// AllFacesVerdict is the verdict that reads every face.
func AllFacesVerdict() FaceVerdict { return FaceVerdict{all: true} }

// NoFacesVerdict is the verdict that reads no face.
func NoFacesVerdict() FaceVerdict { return FaceVerdict{} }

// FacesVerdict is the verdict that reads exactly faces. An empty list is
// none, never every face.
func FacesVerdict(faces ...entity.Face) FaceVerdict {
	if len(faces) == 0 {
		return FaceVerdict{}
	}
	return FaceVerdict{faces: store.SortedFaces(faces)}
}

// All reports whether the verdict reads every face.
func (v FaceVerdict) All() bool { return v.all }

// None reports whether the verdict reads no face.
func (v FaceVerdict) None() bool { return !v.all && len(v.faces) == 0 }

// Faces returns the listed faces, sorted. It is nil for [FaceVerdict.All]
// and for [FaceVerdict.None]; check those first.
func (v FaceVerdict) Faces() []entity.Face {
	if v.all {
		return nil
	}
	return slices.Clone(v.faces)
}

// Contains reports whether the verdict reads f.
func (v FaceVerdict) Contains(f entity.Face) bool {
	return v.all || slices.Contains(v.faces, f)
}

// FaceVerdicts is the answer of [Request.ReadableFacesMany]: one
// [FaceVerdict] per id.
//
// A verdict that does not depend on the row (a global grant, or no grant at
// all) is stored once for every id. An id the per-row verdict did not match
// is absent and answers none. The zero value answers none for every id.
type FaceVerdicts struct {
	uniform   bool
	verdict   FaceVerdict
	perEntity map[string]FaceVerdict
}

// UniformVerdicts answers v for every id.
func UniformVerdicts(v FaceVerdict) FaceVerdicts {
	return FaceVerdicts{uniform: true, verdict: v}
}

// PerEntityVerdicts answers byID[id] for each id, and none for an id it
// does not hold. The map is copied.
func PerEntityVerdicts(byID map[string]FaceVerdict) FaceVerdicts {
	out := make(map[string]FaceVerdict, len(byID))
	maps.Copy(out, byID)
	return FaceVerdicts{perEntity: out}
}

// For returns the verdict for id.
func (vs FaceVerdicts) For(id string) FaceVerdict {
	if vs.uniform {
		return vs.verdict
	}
	return vs.perEntity[id]
}

// ReadableFacesMany answers, for each id of entityType, which stored faces
// the principal may read: the faces in the grant's allowlist whose row
// satisfies the read verdict.
//
//   - A global grant answers every id the same way, with no query: every
//     face, or the faces its `type@face` grants list.
//   - No grant answers none for every id, with no query.
//   - A relation-scoped verdict runs one [FaceMatcher.MatchingFaces] query
//     over every stored face row of ids. Each row is tested on its own, and
//     each conferring relation's branch applies its own face list, so a
//     reviewer edge cannot lend the owner's draft grant to another face.
//
// Existence is not verified: a global grant answers for ids that do not
// exist. Callers that need existence read the row afterwards.
func (r *Request) ReadableFacesMany(ctx context.Context, entityType string, ids []string) (FaceVerdicts, error) {
	return r.readableFacesMany(ctx, r.readQuery(ctx, entityType), ids)
}

// readableFacesMany is [Request.ReadableFacesMany] for a composed scope.
// The template query is copied: it is shared by every call.
func (r *Request) readableFacesMany(ctx context.Context, rqr ReadQueryResult, ids []string) (FaceVerdicts, error) {
	switch {
	case rqr.DenyAll:
		return UniformVerdicts(NoFacesVerdict()), nil
	case rqr.AllowAll:
		if rqr.Faces == nil {
			return UniformVerdicts(AllFacesVerdict()), nil
		}
		return UniformVerdicts(FacesVerdict(rqr.Faces...)), nil
	// coverage-ignore: defensive: readQuery always sets exactly one of AllowAll/DenyAll/Query, so once AllowAll and
	// DenyAll are false Query is non-nil — a zero ReadQueryResult cannot occur
	case rqr.Query == nil:
		return FaceVerdicts{}, errReadQueryZero
	}
	q := *rqr.Query
	q.Faces = store.AllFaces()
	q.FaceIn = rqr.Faces
	matched, err := r.d.faceMatcher.MatchingFaces(ctx, q, ids)
	if err != nil {
		return FaceVerdicts{}, err
	}
	byID := make(map[string]FaceVerdict, len(matched))
	for id, faces := range matched {
		if v := FacesVerdict(faces...); !v.None() {
			byID[id] = v
		}
	}
	return FaceVerdicts{perEntity: byID}, nil
}
