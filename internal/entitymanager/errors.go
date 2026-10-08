package entitymanager

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

// entityNotFoundError wraps [ErrEntityNotFound] with the id and exposes a
// structural EntityNotFound() marker, so consumers that cannot import this
// package (the Lua bindings hold only a narrow consumer-side Mutator
// interface) can distinguish "missing entity" from any other hard error
// WITHOUT matching on the error text.
//
// Text matching is unsafe here: several hard errors embed caller-supplied
// values — an illegal state-machine transition formats the attempted value
// with %q — so a caller setting a property to the literal "entity not
// found" would have its transition rejection misreported as a 404.
//
// errors.Is(err, ErrEntityNotFound) keeps working: Unwrap returns the
// sentinel.
type entityNotFoundError struct{ id string }

func (e entityNotFoundError) Error() string {
	return fmt.Sprintf("%s: %s", ErrEntityNotFound.Error(), e.id)
}

func (e entityNotFoundError) Unwrap() error { return ErrEntityNotFound }

// EntityNotFound marks this as the entity-does-not-exist condition. See
// the lua package's NotFoundError for the consumer-side contract.
func (e entityNotFoundError) EntityNotFound() bool { return true }

// newEntityNotFound builds the not-found error for a given id.
func newEntityNotFound(id string) error { return entityNotFoundError{id: id} }

// ErrEntityAlreadyExists is returned by create paths when the supplied
// or generated ID collides with an existing entity.
var ErrEntityAlreadyExists = errors.New("entity already exists")

// ErrRenameNotSupported is returned by [Manager.RenameEntity] for an entity
// whose type generates its ids (`id_type: short` or `sequential`). Only a
// hand-typed id carries meaning a rename can improve.
var ErrRenameNotSupported = errors.New("rename is only available for types with id_type: manual")

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

// ErrFaceImmutable is the face-axis twin of [ErrTypeImmutable], and exists
// for the identical reason (BUG-Y0GNSB). The store keys a write on
// stateKey(ID, Face), so a body that names a face different from the stored
// one is not editing the record it addressed — it is redirecting the write
// to a sibling face. Since the ACL subject is bound to the stored face,
// allowing the divergence would mean authorizing against one face and
// writing another. Surfaced by the sync handler as HTTP 422, like its twin.
var ErrFaceImmutable = errors.New("entity face is immutable on update; body face differs from the stored face")

// ErrFaceRequired is returned when a create names no face for a type that
// declares `faces:` (BUG-HC6I2T).
//
// A faced type stores no row at the zero coordinate, so there is no default
// to fall back to. Choosing one silently is what the old create path did: it
// wrote the zero coordinate whatever the caller asked for, which under
// `bare_face` was a real face and made the ACL authorize one row while the
// write landed on another.
//
// Refusing is the fail-closed direction. Surfaced as HTTP 422.
var ErrFaceRequired = errors.New("this entity type declares content states; a create must name one")

// ErrRelationFaceRequired is returned when a `scope: content` relation from a
// faced source names no source face. Such an edge belongs to one face, and a
// zero tail would belong to none, so no world would show it as any face's
// content. Surfaced as HTTP 422.
var ErrRelationFaceRequired = errors.New("a content-scoped relation from a faced entity must name the source face")

// ErrFaceNotDeclared is returned when a create names a face the type does not
// declare, or names any face for a type declaring none.
//
// The second case is not pedantry: a type with no `faces:` has exactly one
// state and no name for it, so a caller passing a face has confused this type
// with another and would otherwise get a row nothing can address.
var ErrFaceNotDeclared = errors.New("entity type does not declare this content state")

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

// IsUniqueViolation reports whether err is a `unique:` collision. It arrives
// by two routes, both as a [ValidationError] carrying
// [metamodel.ValidationErrorUnique]: the manager's own scan raises it
// directly, and a pgstore derived-index violation is re-presented as the same
// error. A raw [store.UniquePropertyError] is matched too, for a caller that
// sees one before the manager maps it.
func IsUniqueViolation(err error) bool {
	var unique store.UniquePropertyError
	if errors.As(err, &unique) {
		return true
	}
	var invalid *ValidationError
	if errors.As(err, &invalid) {
		for _, v := range invalid.Errors {
			if v.Type == metamodel.ValidationErrorUnique {
				return true
			}
		}
	}
	return false
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

// ErrRelationNotOrderable reports a move on a relation type whose outgoing
// side declares no managed order.
var ErrRelationNotOrderable = errors.New("relation type is not orderable on the outgoing side")

// ErrInvalidOrderPosition reports an [entity.OrderPosition] that does not set
// exactly one of its fields, steps by something other than one place, names
// the moved edge itself, or comes with property changes.
var ErrInvalidOrderPosition = errors.New("invalid order position")

// ErrOrderRefNotSibling reports a Before/After sibling that is not an edge
// of the same source, type and tail. Callers answer it like a hidden
// sibling, so the error does not tell the two apart.
var ErrOrderRefNotSibling = errors.New("order position names no sibling edge")
