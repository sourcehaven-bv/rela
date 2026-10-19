package entitymanager

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// TestClassifyCreateError pins the class of each refusal a create returns,
// also when it arrives wrapped.
func TestClassifyCreateError(t *testing.T) {
	t.Parallel()
	def := &metamodel.EntityDef{}
	cases := []struct {
		name string
		err  error
		want CreateErrorClass
	}{
		{"nil", nil, CreateErrUnknown},
		{"store failure", errors.New("disk full"), CreateErrUnknown},
		{"acl", &acl.ForbiddenError{}, CreateErrForbidden},
		{"field grant", &affordances.FieldWriteError{Rule: "fields", Path: "status"}, CreateErrForbidden},
		{"guard", &statemachine.GuardError{}, CreateErrForbidden},
		{"entity exists", fmt.Errorf("%w: X-1", ErrEntityAlreadyExists), CreateErrConflict},
		{"relation exists", ErrRelationAlreadyExists, CreateErrConflict},
		{"face required", ErrFaceRequired, CreateErrFace},
		{"face not declared", ErrFaceNotDeclared, CreateErrFace},
		{"validation", newValidationError(nil), CreateErrInvalid},
		{"invalid relation", &InvalidRelationError{err: errors.New("x")}, CreateErrInvalid},
		{"file property", &FileWriteError{EntityType: "doc", Properties: []string{"f"}}, CreateErrInvalid},
		{"illegal entry", fmt.Errorf("%w: status", statemachine.ErrIllegalEntry), CreateErrInvalid},
		{"precondition", fmt.Errorf("%w: status", statemachine.ErrPreconditionFailed), CreateErrInvalid},
		{"custom id", customIDNotAllowedError("doc", def, "DOC-9"), CreateErrInvalid},
		{"cardinality", fmt.Errorf("%w: max 1", ErrCardinalityExceeded), CreateErrInvalid},
		{"missing target", fmt.Errorf("%w: DOC-9", ErrEntityNotFound), CreateErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyCreateError(fmt.Errorf("wrapped: %w", beforeWriteError{tc.err})); tc.err != nil && got != tc.want {
				t.Errorf("wrapped class = %d, want %d", got, tc.want)
			}
			if got := ClassifyCreateError(tc.err); got != tc.want {
				t.Errorf("class = %d, want %d", got, tc.want)
			}
		})
	}
}
