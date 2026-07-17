package entitymanager

import (
	"context"
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
// [metamodel.ValidationError]s of type [metamodel.ValidationErrorUnique]
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
//   - This is a check-then-write, NOT an atomic constraint. The scan and
//     the durable write are separate operations and no lock is held
//     across them, so under concurrent writers (the data-entry server is
//     one goroutine per request) two racing writes with the same value
//     can both pass the scan and both commit — on every backend, not just
//     pgstore. The window is small, and the ACL resolver's multi-match
//     fallback (keep-raw) is the runtime backstop for the identity-key
//     case. The only race-free enforcement is a store-level unique index
//     (a partial unique index on pgstore), which surfaces a duplicate as
//     [store.ErrConflict] on the write; operators who need atomicity add
//     it. See docs/acl-security.md.
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
