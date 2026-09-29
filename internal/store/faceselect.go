package store

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// selectMode is which of the three selections a [FaceSelection] holds.
type selectMode int

const (
	selectUnset selectMode = iota
	selectWorld
	selectAll
	selectFaces
)

// FaceSelection says which faces a query reads (TKT-KQXVF7). It is required
// on [EntityQuery] and [GraphQuery].
//
// The zero value is invalid: every backend answers a query carrying it with
// [ErrInvalidQuery]. There is no default selection, because a default is how
// faced types went missing: the zero query used to read the "" face only,
// where a faced type has no row.
//
// Fields are unexported so the only ways to build one are [InWorld],
// [AllFaces] and [AtFaces].
type FaceSelection struct {
	mode  selectMode
	world WorldScope
	faces []entity.Face
}

// InWorld resolves each entity to at most one face, ranked by w. The zero
// WorldScope is a valid w: it resolves every type to the "" face. Callers
// without a request world take one from worlds.Compiled.Default, never from
// [DefaultWorld] directly; a guard in internal/archguard pins the exceptions.
func InWorld(w WorldScope) FaceSelection { return FaceSelection{mode: selectWorld, world: w} }

// AllFaces returns every face row, as raw storage truth. Several rows per id
// are normal. It is for infrastructure and entity-level scans (search
// backfill, analysis, export, id batches that are gated per face afterwards),
// not for choosing which face of an entity a reader sees.
func AllFaces() FaceSelection { return FaceSelection{mode: selectAll} }

// AtFaces returns the rows at exactly these faces: a set, not a ranking.
// With no arguments it matches nothing, so a caller that computed "no faces"
// gets no rows rather than every row.
func AtFaces(faces ...entity.Face) FaceSelection {
	return FaceSelection{mode: selectFaces, faces: slices.Clone(faces)}
}

// IsZero reports whether s is the invalid zero selection.
func (s FaceSelection) IsZero() bool { return s.mode == selectUnset }

// World returns the world of an [InWorld] selection. ok is false for every
// other mode.
func (s FaceSelection) World() (w WorldScope, ok bool) {
	if s.mode != selectWorld {
		return WorldScope{}, false
	}
	return s.world, true
}

// Faces returns a copy of the face set of an [AtFaces] selection. ok is false
// for every other mode; an AtFaces selection with no faces returns an empty,
// non-nil slice and true.
func (s FaceSelection) Faces() (faces []entity.Face, ok bool) {
	if s.mode != selectFaces {
		return nil, false
	}
	return append([]entity.Face{}, s.faces...), true
}

// IsAll reports whether s is [AllFaces].
func (s FaceSelection) IsAll() bool { return s.mode == selectAll }

// IsDefaultWorld reports whether s is InWorld of the default world: every
// type at the "" face. Backends branch on it for the flat single-row-per-id
// fast path.
func (s FaceSelection) IsDefaultWorld() bool {
	return s.mode == selectWorld && s.world.IsDefaultWorld()
}

// Admits reports whether a row at face f can be returned under s before any
// world ranking: every face under AllFaces, the listed faces under AtFaces,
// and under InWorld the world's candidates are decided per family, so every
// face is admitted here and ranking decides afterwards. The zero selection
// admits nothing.
func (s FaceSelection) Admits(f entity.Face) bool {
	switch s.mode {
	case selectAll, selectWorld:
		return true
	case selectFaces:
		return slices.Contains(s.faces, f)
	default:
		return false
	}
}

// Validate returns [ErrInvalidQuery] for the zero selection.
func (s FaceSelection) Validate() error {
	if s.IsZero() {
		return fmt.Errorf("%w: no face selection (use store.InWorld, store.AllFaces or store.AtFaces)",
			ErrInvalidQuery)
	}
	return nil
}

// String names the selection for logs and test failures.
func (s FaceSelection) String() string {
	switch s.mode {
	case selectWorld:
		if s.world.IsDefaultWorld() {
			return "in-world(default)"
		}
		types := s.world.Types()
		slices.Sort(types)
		return "in-world(" + strings.Join(types, ",") + ")"
	case selectAll:
		return "all-faces"
	case selectFaces:
		names := make([]string, len(s.faces))
		for i, f := range s.faces {
			names[i] = fmt.Sprintf("%q", f.String())
		}
		return "at-faces(" + strings.Join(names, ",") + ")"
	default:
		return "unset"
	}
}
