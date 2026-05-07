package entitymanager

import (
	"errors"
	"fmt"
)

// ErrEntityNotFound is returned when the target entity doesn't exist
// (or its type doesn't match the request's expected type).
var ErrEntityNotFound = errors.New("entity not found")

// ErrETagMismatch is returned by UpdateWithRelations when the request's
// If-Match value doesn't match the entity's current ETag.
var ErrETagMismatch = errors.New("etag mismatch")

// RequestShapeError indicates the request body or its translation into
// a manager request is malformed in a way that's detectable without
// consulting the metamodel — e.g. a relations entry missing required
// fields. HTTP callers should map this to 400.
type RequestShapeError struct {
	Detail string
}

func (e *RequestShapeError) Error() string { return e.Detail }

// ValidationError indicates the request violated a schema or invariant
// that requires the metamodel to detect — unknown relation type, target
// type mismatch, validator rejected meta property value, etc. HTTP
// callers should map this to 422.
type ValidationError struct {
	Detail string
}

func (e *ValidationError) Error() string { return e.Detail }

func validationErrorf(format string, args ...interface{}) error {
	return &ValidationError{Detail: fmt.Sprintf(format, args...)}
}

func requestShapeErrorf(format string, args ...interface{}) error {
	return &RequestShapeError{Detail: fmt.Sprintf(format, args...)}
}
