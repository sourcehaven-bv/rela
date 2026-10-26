package appbuild

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestOutputWriter_Errors pins the code each write error maps to, whether
// a retry may repeat it, and that no code or property list quotes the
// error text or an id.
func TestOutputWriter_Errors(t *testing.T) {
	const secretID = "INC-042"
	invalid := &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{
		{Property: "status", Type: metamodel.ValidationErrorType("enum"), Message: "bad value for " + secretID},
	}}
	unique := &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{
		{Property: "code", Type: metamodel.ValidationErrorUnique, Message: "held by " + secretID},
	}}
	cases := []struct {
		name      string
		err       error
		patch     bool
		code      string
		props     []string
		retryable bool
	}{
		{"denied", &acl.ForbiddenError{}, true, realtime.OutputCodeForbidden, nil, false},
		{"field gate", &affordances.FieldWriteError{Path: "owner"}, true, realtime.OutputCodeFieldDenied, nil, false},
		{"validation", invalid, true, realtime.OutputCodeInvalid, []string{"status"}, false},
		{"unique", unique, true, realtime.OutputCodeConflict, nil, false},
		{"illegal transition", fmt.Errorf("%w: phase of %s", statemachine.ErrIllegalTransition, secretID), true,
			realtime.OutputCodeInvalid, nil, false},
		{"not found", fmt.Errorf("%w: %s", entitymanager.ErrEntityNotFound, secretID), true,
			realtime.OutputCodeNotFound, nil, false},
		{"version race", &store.VersionConflictError{}, true, realtime.OutputCodeConflict, nil, true},
		{"create denied", &acl.ForbiddenError{}, false, realtime.OutputCodeForbidden, nil, false},
		{"create field gate", &affordances.FieldWriteError{Path: "owner"}, false,
			realtime.OutputCodeFieldDenied, nil, false},
		{"create exists", entitymanager.ErrEntityAlreadyExists, false, realtime.OutputCodeConflict, nil, false},
		{"create validation", invalid, false, realtime.OutputCodeInvalid, []string{"status"}, false},
		{"create unique", unique, false, realtime.OutputCodeConflict, nil, false},
		{"create face", entitymanager.ErrFaceNotDeclared, false, realtime.OutputCodeInvalid, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.patch {
				err = patchDenial(tc.err)
			} else {
				err = createFailure(tc.err, true)
			}
			f, ok := errors.AsType[*realtime.OutputError](err)
			require.Truef(t, ok, "want an *OutputError, got %v", err)
			require.Equal(t, tc.code, f.Code)
			require.Equal(t, tc.props, f.Props)
			require.Equal(t, tc.retryable, f.Retryable)
			require.True(t, f.WroteNothing)
			require.NotContains(t, f.Error(), secretID)
		})
	}

	t.Run("unknown", func(t *testing.T) {
		boom := errors.New("disk full")
		require.Equal(t, boom, patchDenial(boom), "an unknown set error is retried as it is")
		require.Equal(t, boom, createFailure(boom, false), "an unknown create that may have written is not classified")
		f, ok := errors.AsType[*realtime.OutputError](createFailure(boom, true))
		require.True(t, ok)
		require.True(t, f.Retryable && f.WroteNothing, "a create that wrote nothing may be retried")
	})
}

// TestOutputCheckError pins the pre-check's answers: a refusal is a 403
// naming the rule, a hidden row is refused like a denied one, and any
// other error is a server error.
func TestOutputCheckError(t *testing.T) {
	forbidden := &acl.ForbiddenError{Decision: acl.Decision{RuleID: "rule-7"}}
	d, ok := errors.AsType[*realtime.DenialError](outputCheckError(forbidden))
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, d.Status)
	require.Equal(t, realtime.TypeSessionOutputDenied, d.Type)
	require.Equal(t, "rule-7", d.Rule)

	hidden, ok := errors.AsType[*realtime.DenialError](outputCheckError(entitymanager.ErrEntityNotFound))
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, hidden.Status)

	boom := errors.New("disk full")
	require.Equal(t, boom, outputCheckError(boom))
}

func TestNewManagerOutputs_RejectsNil(t *testing.T) {
	_, err := newManagerOutputs(nil, nil, nil, nil, nil)
	require.Error(t, err)
}

// TestSetRow_FaceFromMetamodel pins that the row a set writes is chosen by
// the anchor type's declaration: a faceless type without a face, a faced
// type only at the target's face, never as a bare id, and a row of another
// type than the channel's anchor type is not found.
func TestSetRow_FaceFromMetamodel(t *testing.T) {
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "T-"
    properties: { title: { type: string } }
  page:
    label: Page
    id_prefix: "P-"
    properties: { title: { type: string } }
    faces: { draft: {}, final: {} }
`))
	require.NoError(t, err)
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "task"},
		{ID: "P-1", Type: "page", Face: "draft"},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	o := managerOutputs{st: st, meta: meta}
	target := func(id, typ string, face entity.Face) realtime.OutputTarget {
		return realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: id, AnchorType: typ, Face: face}
	}
	cases := []struct {
		name   string
		target realtime.OutputTarget
		want   string
	}{
		{"faceless type", target("T-1", "task", ""), "T-1"},
		{"faced type at its face", target("P-1", "page", "draft"), "P-1@draft"},
		{"faced type without a face", target("P-1", "page", ""), ""},
		{"faced type at a face it lacks", target("P-1", "page", "final"), ""},
		{"another type than the anchor's", target("T-1", "page", "draft"), ""},
		{"missing row", target("T-9", "task", ""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := o.setRow(ctx, tc.target)
			if tc.want == "" {
				require.ErrorIs(t, err, entitymanager.ErrEntityNotFound)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, row.Ref().String())
		})
	}
}

// TestOutputCheck_BindError pins that a bind that fails for a server
// reason surfaces as that error, not as a 403 that names the output; an
// unstamped principal is the one bind failure that is a refusal.
func TestOutputCheck_BindError(t *testing.T) {
	policy, err := acl.LoadPolicyBytes([]byte("roles:\n  editor: {read: [\"*\"]}\n"))
	require.NoError(t, err)
	d, err := acl.NewDeclarative(policy, acl.NullGraph{}, acl.NullGraphQueryer{})
	require.NoError(t, err)
	boom := errors.New("membership walk failed")
	target := realtime.OutputTarget{Kind: realtimecfg.OutputCreate, Type: "tl"}
	cases := []struct {
		name    string
		bindErr error
		denied  bool
	}{
		{"server error", boom, false},
		{"unstamped", fmt.Errorf("wrapped: %w", acl.ErrUnstampedPrincipal), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := managerOutputs{policy: d, bind: func(ctx context.Context) (context.Context, error) {
				return ctx, tc.bindErr
			}}
			err := o.Check(context.Background(), target)
			_, denied := errors.AsType[*realtime.DenialError](err)
			require.Equal(t, tc.denied, denied, "%v", err)
			if !tc.denied {
				require.ErrorIs(t, err, boom)
			}
		})
	}
}
