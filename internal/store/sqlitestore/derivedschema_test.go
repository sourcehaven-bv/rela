package sqlitestore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

var (
	queryStatus = store.DerivedObjectSpec{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"status"}}
	listByDue   = store.DerivedObjectSpec{
		Kind: store.DerivedListIndex, Type: "task", Properties: []string{"status"}, OrderBy: []string{"due"},
	}
)

func derived(t *testing.T, s *sqlitestore.Store) map[string]string {
	t.Helper()
	all, err := s.Indexes(context.Background())
	require.NoError(t, err)
	out := map[string]string{}
	for name, ddl := range all {
		if strings.HasPrefix(name, "rela_derived_") {
			out[name] = ddl
		}
	}
	return out
}

func states(out []store.DerivedObjectOutcome) []store.DerivedObjectState {
	var s []store.DerivedObjectState
	for _, o := range out {
		s = append(s, o.State)
	}
	return s
}

func TestReconcileLifecycle(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	desired := []store.DerivedObjectSpec{queryStatus, listByDue}

	out := reconcile(t, s, desired)
	require.ElementsMatch(t, []store.DerivedObjectState{store.DerivedCreated, store.DerivedCreated}, states(out))
	created := derived(t, s)
	require.Len(t, created, 2)

	// A second pass changes nothing: the stored CREATE text matches the DDL.
	out = reconcile(t, s, desired)
	require.ElementsMatch(t, []store.DerivedObjectState{store.DerivedEnforced, store.DerivedEnforced}, states(out))
	require.Equal(t, created, derived(t, s))

	// Dropping a spec from the desired set drops its index, and only that one.
	out = reconcile(t, s, []store.DerivedObjectSpec{queryStatus})
	require.ElementsMatch(t, []store.DerivedObjectState{store.DerivedEnforced, store.DerivedDropped}, states(out))
	require.Len(t, derived(t, s), 1)

	// An empty desired set drops every owned index.
	_, err := sqlitestore.Reconcile(ctx, s, nil, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Empty(t, derived(t, s))
}

func TestReconcileDryRunChangesNothing(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	reconcile(t, s, []store.DerivedObjectSpec{queryStatus})
	before := derived(t, s)

	out, err := sqlitestore.Reconcile(ctx, s, []store.DerivedObjectSpec{listByDue}, store.ReconcileOptions{DryRun: true})
	require.NoError(t, err)
	require.ElementsMatch(t, []store.DerivedObjectState{store.DerivedCreated, store.DerivedDropped}, states(out))
	for _, o := range out {
		require.True(t, o.WouldChange, "%v", o.State)
	}
	require.Equal(t, before, derived(t, s))

	out, err = sqlitestore.Reconcile(ctx, s, []store.DerivedObjectSpec{queryStatus}, store.ReconcileOptions{DryRun: true})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedEnforced, out[0].State)
	require.False(t, out[0].WouldChange)
}

// An owned index whose stored definition differs is rebuilt from the spec.
func TestReconcileRebuildsChangedDefinition(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	reconcile(t, s, []store.DerivedObjectSpec{queryStatus})
	want := derived(t, s)
	var name string
	for n := range want {
		name = n
	}
	require.NoError(t, s.ExecRaw(ctx, `DROP INDEX "`+name+`"`))
	require.NoError(t, s.ExecRaw(ctx, `CREATE INDEX "`+name+`" ON entities (type)`))

	out := reconcile(t, s, []store.DerivedObjectSpec{queryStatus})
	require.Equal(t, []store.DerivedObjectState{store.DerivedCreated}, states(out))
	require.Equal(t, want, derived(t, s))
}

// Indexes outside the owned prefixes are never touched, including the
// store's own.
func TestReconcileLeavesUnownedIndexes(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	require.NoError(t, s.ExecRaw(ctx, `CREATE INDEX operator_idx ON entities (updated_at)`))
	before, err := s.Indexes(ctx)
	require.NoError(t, err)

	reconcile(t, s, []store.DerivedObjectSpec{queryStatus})
	_, err = sqlitestore.Reconcile(ctx, s, nil, store.ReconcileOptions{})
	require.NoError(t, err)

	after, err := s.Indexes(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

// A property name the builder cannot spell as a literal JSON path is reported
// unenforced rather than interpolated.
func TestReconcileRejectsUnsafeNames(t *testing.T) {
	s := open(t)
	for _, spec := range []store.DerivedObjectSpec{
		{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{`a"b`}},
		{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"a\\b"}},
		{Kind: store.DerivedListIndex, Type: "task", OrderBy: []string{"a\nb"}},
		{Kind: store.DerivedQueryIndex, Type: "task"},
		{Kind: store.DerivedListIndex, Type: "task"},
	} {
		out, err := sqlitestore.Reconcile(context.Background(), s, []store.DerivedObjectSpec{spec}, store.ReconcileOptions{})
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Equal(t, store.DerivedUnenforced, out[0].State, "%+v", spec)
	}
	require.Empty(t, derived(t, s))
}

// A name that needs quoting but is safe (a quote, a dot, a space) indexes and
// is used by the query that filters on it.
func TestReconcileQuotedNameIsUsed(t *testing.T) {
	s := open(t)
	reconcile(t, s, []store.DerivedObjectSpec{
		{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"it's a.b"}},
	})
	plan := explain(t, s, store.GraphQuery{
		EntityType: "task",
		Props:      []store.PropPredicate{{Property: "it's a.b", Op: store.PropEqual, Value: "x", Scalar: true}},
	})
	require.Contains(t, plan, "rela_derived_query__")
}
