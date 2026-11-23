package records_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const permSchema = mutableSchema + `
  gated:
    owner: site
    mutable: true
    permissions: {read: "gated:read", append: "gated:append"}
    fields:
      kwh: {type: decimal}
`

type heldPerms map[string]bool

func (h heldPerms) Holds(_ context.Context, perm string) bool { return h[perm] }

func newPermService(t *testing.T, held heldPerms) *records.Service {
	t.Helper()
	meta, err := metamodel.Parse([]byte(permSchema))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	o := &owners{types: map[string]string{"SITE-1": "site"}, props: map[string]map[string]any{},
		hidden: map[string]bool{}, readOnly: map[string]bool{}}
	s, err := records.NewService(records.Deps{Store: st, Owners: o, Meta: meta, Audit: audit.NewMemory(),
		Permissions: held, Now: func() time.Time { return t0 }})
	require.NoError(t, err)
	return s
}

func TestPermissions_NeededWiring(t *testing.T) {
	meta, err := metamodel.Parse([]byte(permSchema))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	_, err = records.NewService(records.Deps{Store: st, Owners: &owners{}, Meta: meta, Audit: audit.NewMemory()})
	require.Error(t, err)
}

func TestPermissions_AppendNarrowsOwnerRule(t *testing.T) {
	ctx := context.Background()
	in := []records.Input{{Owner: "SITE-1", Fields: map[string]any{"kwh": "1"}}}

	full := newPermService(t, heldPerms{"gated:read": true, "gated:append": true})
	rows, err := full.Append(ctx, "gated", in)
	require.NoError(t, err)

	readOnly := newPermService(t, heldPerms{"gated:read": true})
	_, err = readOnly.Append(ctx, "gated", in)
	require.ErrorIs(t, err, records.ErrForbidden)
	_, err = readOnly.Update(ctx, "gated", rows[0].ID, records.Change{Revision: 1, Fields: map[string]any{"kwh": "2"}})
	require.ErrorIs(t, err, records.ErrForbidden)
	require.ErrorIs(t, readOnly.Delete(ctx, "gated", rows[0].ID, 1), records.ErrForbidden)
	_, err = readOnly.List(ctx, "gated", "SITE-1", records.ListQuery{})
	require.NoError(t, err)

	// A type without permissions keeps the owner rule alone.
	_, err = readOnly.Append(ctx, "reading", []records.Input{{Owner: "SITE-1"}})
	require.NoError(t, err)
}

func TestPermissions_ReadGatesListAndAggregate(t *testing.T) {
	ctx := context.Background()
	s := newPermService(t, heldPerms{"gated:append": true})
	_, err := s.Append(ctx, "gated", []records.Input{{Owner: "SITE-1", Fields: map[string]any{"kwh": "1"}}})
	require.NoError(t, err)
	_, err = s.List(ctx, "gated", "SITE-1", records.ListQuery{})
	require.ErrorIs(t, err, records.ErrForbidden)
	_, err = s.Aggregate(ctx, records.AggregateQuery{Type: "gated", Owners: []string{"SITE-1"}})
	require.ErrorIs(t, err, records.ErrForbidden)
}

func TestMetamodel_RecordPermissionName(t *testing.T) {
	bad := mutableSchema + `
  bad:
    owner: site
    permissions: {read: "has space"}
    fields:
      x: {type: string}
`
	_, err := metamodel.Parse([]byte(bad))
	require.Error(t, err)
}
