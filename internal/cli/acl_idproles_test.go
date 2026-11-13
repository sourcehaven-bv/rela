package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/output"
)

type fakeForgetter struct {
	n   int
	err error
	got string
}

func (f *fakeForgetter) Forget(_ context.Context, id string) (int, error) {
	f.got = id
	return f.n, f.err
}

func TestACLForgetIDPRolesCmd(t *testing.T) {
	buf := withOutput(t, output.FormatTable)
	ctx := context.Background()
	cmd := &ACLForgetIDPRolesCmd{ID: "PER-1"}

	require.ErrorContains(t, cmd.Run(ctx, &writeServices{}), "no store of IdP roles")

	for _, n := range []int{0, 1} {
		f := &fakeForgetter{n: n}
		require.NoError(t, cmd.Run(ctx, &writeServices{IDPRoles: f}))
		require.Equal(t, "PER-1", f.got)
	}
	require.Contains(t, buf.String(), "Erased the IdP roles stored for PER-1")

	f := &fakeForgetter{err: errors.New("boom")}
	require.ErrorContains(t, cmd.Run(ctx, &writeServices{IDPRoles: f}), "boom")
}
