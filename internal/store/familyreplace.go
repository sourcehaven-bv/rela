package store

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ErrInvalidFamilyReplacement is wrapped by every structural rejection of a
// [FamilyReplacer.ReplaceEntityFamily] call, so a caller can tell a malformed
// plan from a storage failure.
var ErrInvalidFamilyReplacement = errors.New("store: invalid family replacement")

// ValidateFamilyReplacement checks a replacement plan for id against the faces
// currently stored at id, before anything is written. Every backend runs it
// (through storeutil.ValidateFamilyReplacement, which adds the property rules),
// and the entity manager may run it inside its Tx to refuse a plan before the
// first write.
//
// The plan must name every current face exactly once as a source, give every
// target row the id and one shared non-empty type, and map no two sources onto
// one target face. It returns the source faces no target row occupies: those
// coordinates stop existing, so the backend reports them as deleted.
//
// An empty current returns [ErrNotFound]; every other rejection wraps
// [ErrInvalidFamilyReplacement].
func ValidateFamilyReplacement(
	id string, current []entity.Face, replacements []FamilyReplacement,
) (emptied []entity.Face, err error) {
	if len(current) == 0 {
		return nil, ErrNotFound
	}
	if id == "" || len(replacements) == 0 {
		return nil, fmt.Errorf("%w: requires an id and at least one row", ErrInvalidFamilyReplacement)
	}
	seenSource := make(map[entity.Face]bool, len(replacements))
	seenTarget := make(map[entity.Face]bool, len(replacements))
	typeName := ""
	for _, r := range replacements {
		e := r.Entity
		if e == nil || e.ID != id || e.Type == "" {
			return nil, fmt.Errorf("%w: rows must use id %q and a type", ErrInvalidFamilyReplacement, id)
		}
		if !slices.Contains(current, r.SourceFace) || seenSource[r.SourceFace] {
			return nil, fmt.Errorf("%w: source face %q is missing or repeated", ErrInvalidFamilyReplacement, r.SourceFace)
		}
		seenSource[r.SourceFace] = true
		if typeName == "" {
			typeName = e.Type
		} else if e.Type != typeName {
			return nil, fmt.Errorf("%w: mixed types %q and %q", ErrInvalidFamilyReplacement, typeName, e.Type)
		}
		if seenTarget[e.Face] {
			return nil, fmt.Errorf("%w: multiple rows map to face %q", ErrInvalidFamilyReplacement, e.Face)
		}
		seenTarget[e.Face] = true
	}
	if len(seenSource) != len(current) {
		return nil, fmt.Errorf("%w: every source face must be accounted for", ErrInvalidFamilyReplacement)
	}
	for _, face := range current {
		if !seenTarget[face] {
			emptied = append(emptied, face)
		}
	}
	return emptied, nil
}
