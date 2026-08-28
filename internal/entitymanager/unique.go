package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// checkUniqueProperties enforces the `unique: true` natural-key
// constraint on e: for every property of e.Type declared unique, no other
// entity of the same type may carry the same non-empty value.
//
// It is called at every entity-write choke point right after
// [metamodel.Metamodel.ValidateEntity] — createCore (create), UpdateEntity
// (update), and ApplyEntity (sync). excludeSelfID names the entity's own
// ID so a re-save of an unchanged value does not collide with itself; pass
// "" on the create path.
//
// The check queries other entities and so cannot live in the pure,
// per-entity ValidateEntity. Violations are returned as
// [metamodel.ValidationError] values of type [metamodel.ValidationErrorUnique]
// (a HARD error) so the caller folds them into the same
// [newValidationError] → 422 path as structural validation failures.
//
// Semantics and limits:
//
//   - Empty values are exempt — a property is unique among the entities
//     that set it, matching the "email is unique for the people who have
//     one" intent. `list` properties are skipped (a natural key is a
//     scalar).
//   - Comparison is on the string value (identity keys — email, UPN — are
//     strings). Non-string property values are compared via their string
//     form and in practice only strings carry unique keys.
//   - This scan is a check-then-write, NOT an atomic constraint: the scan
//     and the durable write are separate operations with no lock held
//     across them, so under concurrent writers (the data-entry server is
//     one goroutine per request) two racing writes with the same value
//     could both pass the scan. On PostgreSQL the durable write is
//     backstopped by a store-level partial unique index that pgstore
//     maintains from the metamodel (TKT-3Q0GP1): the second writer's
//     insert fails atomically and surfaces as [store.UniquePropertyError],
//     which the write path maps to the SAME 422 this scan produces — so
//     the scan stays the friendly primary path and the index closes the
//     race. On fsstore/memstore there is no such index (single-writer, so
//     the scan suffices); the ACL resolver's multi-match fallback
//     (keep-raw) remains the runtime backstop for the identity-key case.
//     See docs/acl-security.md and docs/postgres-backend.md.
func checkUniqueProperties(
	ctx context.Context, deps Deps, e *entity.Entity, excludeSelfID string,
) error {
	def, ok := deps.Meta.GetEntityDef(e.Type)
	if !ok {
		return nil // unknown type is caught by ValidateEntity's own path
	}

	// Collect the unique, non-list properties this entity actually sets to
	// a non-empty value. If none, skip the scan entirely — the common case
	// for types without a natural key pays nothing.
	type uniqueProp struct {
		name  string
		value string
	}
	var toCheck []uniqueProp
	for name, pd := range def.PropertyDefs() {
		if !pd.Unique || pd.List {
			continue
		}
		if v := e.GetString(name); v != "" {
			toCheck = append(toCheck, uniqueProp{name: name, value: v})
		}
	}
	if len(toCheck) == 0 {
		return nil
	}

	var violations []*metamodel.ValidationError
	for other, err := range deps.Store.ListEntities(ctx, store.EntityQuery{Type: e.Type}) {
		if err != nil {
			// A partial scan cannot prove uniqueness — fail the write loud
			// rather than admit a possible duplicate.
			return fmt.Errorf("entitymanager: unique check for %s: %w", e.ID, err)
		}
		if other.ID == excludeSelfID {
			continue
		}
		for _, up := range toCheck {
			if other.GetString(up.name) == up.value {
				// Client-facing message names only the property + type — NOT
				// the colliding entity's ID or the value. For an identity key
				// (e.g. persoon.email) leaking "PERS-X already has alice@corp"
				// to whoever attempted the write is an enumeration oracle
				// (confirms an entity/value is registered). The full detail is
				// logged server-side instead, mirroring the RR-372L discipline.
				slog.Warn("entitymanager: unique constraint violated",
					"type", e.Type, "property", up.name,
					"attempted_by", e.ID, "conflicts_with", other.ID)
				violations = append(violations, &metamodel.ValidationError{
					Type:     metamodel.ValidationErrorUnique,
					Property: up.name,
					Message: fmt.Sprintf(
						"property %q must be unique for type %q; another entity already has this value",
						up.name, e.Type),
				})
			}
		}
	}
	if len(violations) > 0 {
		return newValidationError(violations)
	}
	return nil
}

// mapUniquePropertyConflict translates a store-level derived-unique-index
// violation ([store.UniquePropertyError], raised atomically by a pgstore partial
// unique index — TKT-3Q0GP1) into the SAME [ValidationError] the pre-write
// [checkUniqueProperties] scan produces, so a client cannot tell which
// enforcement path caught the duplicate: both yield a 422 naming the property
// and withholding the colliding value. It returns the original error unchanged
// when it is not an UniquePropertyError.
//
// This is the second-line backstop to the scan: the scan wins the common case
// (and is the only mechanism on fs/mem), but a concurrent writer that passed
// the scan is stopped atomically by the index and lands here. When the store
// could not attribute the violation to a property (empty Property — e.g. a
// rolling deploy against a peer-created index), it degrades to a generic
// property-less unique error rather than inventing a property name.
//
// ok reports whether err was a UniquePropertyError (and thus mapped); when
// false the returned error is err unchanged, so callers write
// `if ok, v := mapUniquePropertyConflict(err); ok { return v }`.
func mapUniquePropertyConflict(err error) (ok bool, mapped error) {
	var up store.UniquePropertyError
	if !errors.As(err, &up) {
		return false, err
	}
	msg := "a property that must be unique already has this value"
	if up.Property != "" {
		msg = fmt.Sprintf(
			"property %q must be unique; another entity already has this value", up.Property)
	}
	return true, newValidationError([]*metamodel.ValidationError{{
		Type:     metamodel.ValidationErrorUnique,
		Property: up.Property,
		Message:  msg,
	}})
}
