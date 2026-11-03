package storeutil

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ValidateFamilyReplacement is the check every backend runs before its first
// ReplaceEntityFamily write: the structural rules of
// [store.ValidateFamilyReplacement] plus [ValidateProperties] on each target
// row, so the four backends accept and refuse exactly the same plans.
func ValidateFamilyReplacement(
	id string, current []entity.Face, replacements []store.FamilyReplacement,
) (emptied []entity.Face, err error) {
	if idErr := ValidateID(id); idErr != nil {
		return nil, idErr
	}
	emptied, err = store.ValidateFamilyReplacement(id, current, replacements)
	if err != nil {
		return nil, err
	}
	for _, r := range replacements {
		if err := ValidateProperties(r.Entity.Properties); err != nil {
			return nil, err
		}
	}
	return emptied, nil
}
