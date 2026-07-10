package entitymanager

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ErrHasRelations is returned by [Manager.DeleteEntity] when cascade
// is false but the entity has incident relations.
var ErrHasRelations = errors.New("entity has relations; set cascade=true to delete")

// ErrEntityNotFound is returned when an entity lookup fails. Wraps the
// underlying [store.ErrNotFound] via %w so callers can use
// [errors.Is].
var ErrEntityNotFound = errors.New("entity not found")

// ErrRelationNotFound is returned when a relation lookup fails.
var ErrRelationNotFound = errors.New("relation not found")

// ErrEntityVanishedOnUpdate is returned by the sync apply path when an
// update-intent write finds its target row gone at write time: the existence
// probe observed the entity, but the durable UpdateEntity hit
// [store.ErrNotFound] because a concurrent delete landed in between. It wraps
// [ErrEntityNotFound] via %w so existing errors.Is(err, ErrEntityNotFound)
// callers still match, while the sync handler can single it out and map it to
// a 412 conflict (symmetric with the relation vanished-on-update case) rather
// than the 404 reserved for a genuinely missing relation endpoint.
var ErrEntityVanishedOnUpdate = fmt.Errorf("%w (vanished concurrently on update)", ErrEntityNotFound)

// ErrEntityAlreadyExists is returned by create paths when the supplied
// or generated ID collides with an existing entity.
var ErrEntityAlreadyExists = errors.New("entity already exists")

// ErrTypeImmutable is returned by the upsert/apply path when the caller
// supplies a type that differs from the STORED type of an existing entity.
// An entity's type is immutable on update: an UPDATE is authorized and
// validated against the resource's stored type, so re-typing it via the
// body would (a) escalate a cross-type write past the ACL — a principal
// permitted to write type B could overwrite a stored type-A record by
// claiming type B in the body — and (b) corrupt the store (fsstore would
// write the record under a second type's path, orphaning the original).
// The v1 PATCH contract omits `type` from the body entirely for the same
// reason; sync PUT carries a type (it is a create-or-update upsert) but
// must reject a body type that contradicts the stored one. Surfaced by the
// sync handler as HTTP 422 (a structural impossibility per DEC-HWZHA — the
// storage layer cannot persist one ID as two types).
var ErrTypeImmutable = errors.New("entity type is immutable on update; body type differs from the stored type")

// ErrRelationAlreadyExists is returned by [Manager.CreateRelation]
// when the (from, type, to) tuple already exists.
var ErrRelationAlreadyExists = errors.New("relation already exists")

// ValidationError wraps multiple metamodel validation errors into a
// single error value. Returned by [Manager.CreateEntity] and
// [Manager.UpdateEntity] when the metamodel's per-property validation
// rejects the entity.
type ValidationError struct {
	Errors []*metamodel.ValidationError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return "validation errors:\n  " + strings.Join(msgs, "\n  ")
}

// newValidationError wraps a slice of metamodel validation errors.
func newValidationError(errs []*metamodel.ValidationError) *ValidationError {
	return &ValidationError{Errors: errs}
}

// customIDNotAllowedError formats the error returned when a caller
// supplies an explicit ID for an entity type whose id_type
// auto-generates. The message names the type, the id_type, the
// offending input, and tells the caller what to do instead.
func customIDNotAllowedError(entityType string, def *metamodel.EntityDef, offendingID string) error {
	hint := "omit the \"id\" field to auto-generate one"
	if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
		hint = fmt.Sprintf("omit the \"id\" field to auto-generate one (prefix %q)", prefixes[0])
	}
	return fmt.Errorf(
		"entity type %q uses id_type=%s; custom ID %q not allowed — %s",
		entityType, def.GetIDType(), offendingID, hint,
	)
}
