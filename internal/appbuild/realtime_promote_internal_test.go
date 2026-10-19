package appbuild

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
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

// TestPromoteError pins when a failure says nothing was written: always
// when atomic, and otherwise only for a refused write.
func TestPromoteError(t *testing.T) {
	disk := errors.New("disk full")
	require.ErrorIs(t, promoteError(disk, true), realtime.ErrNothingWritten)
	require.ErrorIs(t, promoteError(disk, true), disk)
	require.NotErrorIs(t, promoteError(disk, false), realtime.ErrNothingWritten)
	require.ErrorIs(t, promoteError(&entitymanager.ValidationError{}, false), realtime.ErrNothingWritten)
	require.ErrorIs(t, promoteError(entitymanager.ErrEntityAlreadyExists, false), realtime.ErrNothingWritten)
	_, ok := errors.AsType[*realtime.DenialError](promoteError(entitymanager.ErrFaceRequired, false))
	require.True(t, ok)
}

func TestNewManagerPromoter_RejectsNil(t *testing.T) {
	_, err := newManagerPromoter(nil, nil)
	require.Error(t, err)
}
