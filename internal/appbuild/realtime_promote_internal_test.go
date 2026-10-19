package appbuild

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// TestClassifyPromote pins the answer of each create error class, including
// those the service tests cannot reach with generated ids.
func TestClassifyPromote(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		typ    string
	}{
		{"acl", &acl.ForbiddenError{}, http.StatusForbidden, realtime.TypePromoteDenied},
		{"guard", fmt.Errorf("x: %w", statemachine.ErrGuardDenied), http.StatusForbidden, realtime.TypePromoteDenied},
		{"entity exists", entitymanager.ErrEntityAlreadyExists, http.StatusConflict, realtime.TypePromoteConflict},
		{"relation exists", entitymanager.ErrRelationAlreadyExists, http.StatusConflict, realtime.TypePromoteConflict},
		{"validation", &entitymanager.ValidationError{}, http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"face", entitymanager.ErrFaceRequired, http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"face undeclared", entitymanager.ErrFaceNotDeclared, http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"target", entitymanager.ErrEntityNotFound, http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"illegal entry", fmt.Errorf("%w: status", statemachine.ErrIllegalEntry),
			http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"precondition", fmt.Errorf("%w: status", statemachine.ErrPreconditionFailed),
			http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"cardinality", entitymanager.ErrCardinalityExceeded, http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"custom id", entitymanager.ErrCustomIDNotAllowed, http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := classifyPromote(tc.err)
			require.NotNil(t, d)
			require.Equal(t, tc.status, d.Status)
			require.Equal(t, tc.typ, d.Type)
		})
	}
	require.Nil(t, classifyPromote(errors.New("disk full")))
}

// TestPromoteDenial_EveryClass fails when the shared classifier gains a
// class the promoter does not answer: the HTTP create route classifies
// through the same function, so the two cannot drift.
func TestPromoteDenial_EveryClass(t *testing.T) {
	for _, class := range entitymanager.AllCreateErrorClasses() {
		d := promoteDenial(class, errors.New("x"))
		if class == entitymanager.CreateErrUnknown {
			require.Nil(t, d)
			continue
		}
		require.NotNilf(t, d, "class %d has no promote answer", class)
	}
}

// TestPromoteDenial_TitleQuotesNoError pins that a refusal's title names
// properties, never the error text, which can hold entity ids the caller
// may not read.
func TestPromoteDenial_TitleQuotesNoError(t *testing.T) {
	err := &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{{
		Type: metamodel.ValidationErrorUnique, Property: "code", Message: "code is already used by MIN-SECRET",
	}}}
	d := classifyPromote(err)
	require.NotNil(t, d)
	require.Contains(t, d.Title, "code (unique)")
	require.NotContains(t, d.Error(), "MIN-SECRET")
	for _, e := range []error{
		fmt.Errorf("%w: MIN-SECRET", entitymanager.ErrEntityAlreadyExists),
		fmt.Errorf("%w: MIN-SECRET", entitymanager.ErrEntityNotFound),
		fmt.Errorf("%w: MIN-SECRET", entitymanager.ErrFaceRequired),
	} {
		require.NotContains(t, classifyPromote(e).Error(), "MIN-SECRET")
	}
}

// TestPromoteError pins when a failure says nothing was written: always
// when atomic, and otherwise only when CreateLinked reports it, which the
// real-wiring tests cover.
func TestPromoteError(t *testing.T) {
	disk := errors.New("disk full")
	require.ErrorIs(t, promoteError(disk, true), realtime.ErrNothingWritten)
	require.ErrorIs(t, promoteError(disk, true), disk)
	require.NotErrorIs(t, promoteError(disk, false), realtime.ErrNothingWritten)
	_, ok := errors.AsType[*realtime.DenialError](promoteError(entitymanager.ErrFaceRequired, false))
	require.True(t, ok)
}

func TestNewManagerPromoter_RejectsNil(t *testing.T) {
	_, err := newManagerPromoter(nil, nil)
	require.Error(t, err)
}
