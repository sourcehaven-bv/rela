package entitymanager

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// CreateErrorClass is the kind of refusal an entity create error is. The
// HTTP create route and the channel promote both answer from it, so a new
// refusal is classified once for both.
type CreateErrorClass int

const (
	// CreateErrUnknown is not a refusal the manager knows: a server error.
	CreateErrUnknown CreateErrorClass = iota
	// CreateErrForbidden is a refusal of the caller: the create ACL, a
	// field grant, or a state-machine guard.
	CreateErrForbidden
	// CreateErrConflict is an entity or relation that already exists.
	CreateErrConflict
	// CreateErrFace is a missing or undeclared face: the address, not the
	// payload, is wrong.
	CreateErrFace
	// CreateErrInvalid is a payload the schema refuses: validation,
	// including a unique collision, an illegal state-machine entry or
	// precondition, a file or external-ref property, a custom id, a
	// relation the metamodel or its cardinality refuses, or a missing
	// relation target.
	CreateErrInvalid
)

// AllCreateErrorClasses lists every [CreateErrorClass], so a caller's test
// can check that it answers each one.
func AllCreateErrorClasses() []CreateErrorClass {
	return []CreateErrorClass{CreateErrUnknown, CreateErrForbidden, CreateErrConflict, CreateErrFace, CreateErrInvalid}
}

// ClassifyCreateError returns the class of an error from
// [Manager.CreateEntity] or [CreateLinked].
func ClassifyCreateError(err error) CreateErrorClass {
	var forbidden *acl.ForbiddenError
	var denial auditedDenial
	var invalid *ValidationError
	var invalidRel *InvalidRelationError
	var fileWrite *FileWriteError
	switch {
	case err == nil:
		return CreateErrUnknown
	case errors.As(err, &forbidden), errors.As(err, &denial), errors.Is(err, statemachine.ErrGuardDenied):
		return CreateErrForbidden
	case errors.Is(err, ErrEntityAlreadyExists), errors.Is(err, ErrRelationAlreadyExists):
		return CreateErrConflict
	case errors.Is(err, ErrFaceRequired), errors.Is(err, ErrFaceNotDeclared):
		return CreateErrFace
	case errors.As(err, &invalid), errors.As(err, &invalidRel), errors.As(err, &fileWrite),
		errors.Is(err, statemachine.ErrIllegalEntry), errors.Is(err, statemachine.ErrPreconditionFailed),
		errors.Is(err, ErrCustomIDNotAllowed), errors.Is(err, ErrCardinalityExceeded),
		errors.Is(err, ErrEntityNotFound):
		return CreateErrInvalid
	}
	return CreateErrUnknown
}
