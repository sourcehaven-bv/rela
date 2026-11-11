package affordances_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// stubCeiling is a FlowEditCeiling that withholds a fixed set, and records
// what it was asked.
type stubCeiling struct {
	mu       sync.Mutex
	withheld map[string]string
	// permission, when set, is withheld unless holds grants it.
	permission string
	err        error
	primeErr   error
	asked      int
	primed     [][]string
}

func (s *stubCeiling) Governs(entityType string) bool { return entityType == "case" }

func (s *stubCeiling) Permissions(string) []string {
	if s.permission == "" {
		return nil
	}
	return []string{s.permission}
}

func (s *stubCeiling) PrimeRuns(ctx context.Context, ids []string) (context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.primed = append(s.primed, slices.Clone(ids))
	return ctx, s.primeErr
}

func (s *stubCeiling) Withheld(
	_ context.Context, _ *entity.Entity, holds func(string) bool,
) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked++
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]string{}
	maps.Copy(out, s.withheld)
	if s.permission != "" && !holds(s.permission) {
		out["betrokkenen"] = "outside_edits:flow=melden state=intake"
	}
	return out, nil
}

type flowEditWorld struct {
	resolver *affordances.PolicyResolver
	decl     *acl.Declarative
	ceiling  *stubCeiling
	row      *entity.Entity
}

func newFlowEditWorld(t *testing.T, policyYAML string, ceiling *stubCeiling) flowEditWorld {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"case": {Properties: map[string]metamodel.PropertyDef{
				"ernst":       {Type: metamodel.PropertyTypeString},
				"notes":       {Type: metamodel.PropertyTypeString},
				"betrokkenen": {Type: metamodel.PropertyTypeString},
			}},
			"note": {Properties: map[string]metamodel.PropertyDef{
				"text": {Type: metamodel.PropertyTypeString},
			}},
		},
	}
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	require.NoError(t, err)
	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	row := entity.New("C-1", "case")
	mustCreate(t, ms, row)
	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(ms), ms)
	require.NoError(t, err)
	resolver, err := affordances.New(meta, storeRelationLookup{ms}, decl, affordances.WithFlowEdits(ceiling))
	require.NoError(t, err)
	return flowEditWorld{resolver: resolver, decl: decl, ceiling: ceiling, row: row}
}

func (w flowEditWorld) ctx(t *testing.T) context.Context {
	t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	return acltest.Bind(ctx, t, w.decl)
}

const flowEditPolicy = `
roles:
  editor:
    read: [case, note]
    update: [case, note]
    create: [case]
assignments:
  alice: editor
`

const flowEditOfficerPolicy = `
roles:
  editor:
    read: [case, note]
    update: [case, note]
    permissions: [privacy.officer]
assignments:
  alice: editor
`

// notesReadOnly closes notes with a fields: grant.
const notesReadOnly = `
roles:
  editor:
    read: [case]
    update: [case]
    fields:
      case:
        - field: ernst
        - field: betrokkenen
assignments:
  alice: editor
`

func TestFieldVerdicts_FlowEdits(t *testing.T) {
	melden := "outside_edits:flow=melden state=intake"
	tests := []struct {
		name    string
		policy  string
		ceiling *stubCeiling
		// readOnly is the properties FieldVerdicts makes read-only.
		readOnly []string
		// attr is the attribution of each, by property.
		attr map[string]string
	}{
		{name: "nothing withheld", policy: flowEditPolicy, ceiling: &stubCeiling{}},
		{name: "deny", policy: flowEditPolicy, ceiling: &stubCeiling{withheld: map[string]string{"ernst": melden}},
			readOnly: []string{"ernst"},
			attr:     map[string]string{"ernst": "affordance:read-only role=" + melden}},
		{name: "permission not held", policy: flowEditPolicy, ceiling: &stubCeiling{permission: "privacy.officer"},
			readOnly: []string{"betrokkenen"}},
		{name: "permission held", policy: flowEditOfficerPolicy,
			ceiling: &stubCeiling{permission: "privacy.officer"}},
		{name: "allow does not open a field a grant closed", policy: notesReadOnly, ceiling: &stubCeiling{},
			readOnly: []string{"notes"}},
		{name: "grant and ceiling intersect", policy: notesReadOnly,
			ceiling:  &stubCeiling{withheld: map[string]string{"ernst": melden}},
			readOnly: []string{"ernst", "notes"},
			attr:     map[string]string{"ernst": "affordance:read-only role=" + melden}},
		{name: "lookup error fails closed", policy: flowEditPolicy, ceiling: &stubCeiling{err: errors.New("down")},
			readOnly: []string{"betrokkenen", "ernst", "notes"},
			attr: map[string]string{
				"ernst": "affordance:read-only role=outside_edits:unavailable",
				"notes": "affordance:read-only role=outside_edits:unavailable",
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newFlowEditWorld(t, tc.policy, tc.ceiling)
			v := w.resolver.FieldVerdicts(w.ctx(t), w.row)
			var got []string
			for name, ok := range v.Writable {
				if !ok {
					got = append(got, name)
				}
			}
			slices.Sort(got)
			require.Equal(t, tc.readOnly, got)
			for name, want := range tc.attr {
				require.Equal(t, want, v.Attribution[name], name)
			}
			require.Empty(t, v.Visible, "the ceiling leaves visibility alone")
			require.Equal(t, 1, tc.ceiling.asked)
		})
	}
}

func TestFieldVerdicts_FlowEditsScope(t *testing.T) {
	melden := map[string]string{"ernst": "outside_edits:flow=melden state=intake"}

	t.Run("create verdicts are not narrowed", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{withheld: melden})
		v := w.resolver.CreateFieldVerdicts(w.ctx(t), w.row)
		require.Empty(t, v.Writable)
		require.Zero(t, w.ceiling.asked)
	})
	t.Run("a historical snapshot is not narrowed", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{withheld: melden})
		v := w.resolver.FieldVerdicts(affordances.WithHistoricalSubject(w.ctx(t)), w.row)
		require.Empty(t, v.Writable)
		require.Zero(t, w.ceiling.asked)
	})
	t.Run("a type no flow governs is not asked", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{withheld: melden})
		v := w.resolver.FieldVerdicts(w.ctx(t), &entity.Entity{ID: "N-1", Type: "note"})
		require.Empty(t, v.Writable)
		require.Zero(t, w.ceiling.asked)
	})
	t.Run("a new row is not asked", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{withheld: melden})
		v := w.resolver.FieldVerdicts(w.ctx(t), &entity.Entity{Type: "case"})
		require.Empty(t, v.Writable)
		require.Zero(t, w.ceiling.asked)
	})
	t.Run("for visibility skips the lookup and fails closed on writable", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{withheld: melden})
		v := w.resolver.FieldVerdicts(affordances.ForVisibility(w.ctx(t)), w.row)
		require.Zero(t, w.ceiling.asked)
		require.Empty(t, v.Visible)
		require.Equal(t, map[string]bool{"ernst": false, "notes": false, "betrokkenen": false}, v.Writable)
		require.Equal(t, "affordance:read-only role=outside_edits:not-computed", v.Attribution["notes"])
	})
	t.Run("no identity still denies every field", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{})
		v := w.resolver.FieldVerdicts(context.Background(), w.row)
		require.Len(t, v.Writable, 3)
		require.Len(t, v.Visible, 3)
		require.Zero(t, w.ceiling.asked)
	})
}

func TestPrimeFlowRuns(t *testing.T) {
	rows := []*entity.Entity{
		{ID: "C-1", Type: "case"}, {ID: "N-1", Type: "note"}, nil, {Type: "case"}, {ID: "C-2", Type: "case"},
	}

	t.Run("primes governed rows in one call", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{})
		affordances.PrimeFlowRuns(context.Background(), w.resolver, rows)
		require.Equal(t, [][]string{{"C-1", "C-2"}}, w.ceiling.primed)
	})
	t.Run("nothing governed, nothing primed", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{})
		affordances.PrimeFlowRuns(context.Background(), w.resolver, rows[1:2])
		require.Empty(t, w.ceiling.primed)
	})
	t.Run("an error leaves ctx unprimed", func(t *testing.T) {
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{primeErr: errors.New("down")})
		ctx := context.Background()
		require.Equal(t, ctx, affordances.PrimeFlowRuns(ctx, w.resolver, rows))
	})
	t.Run("no resolver or no ceiling", func(t *testing.T) {
		ctx := context.Background()
		require.Equal(t, ctx, affordances.PrimeFlowRuns(ctx, nil, rows))
		w := newFlowEditWorld(t, flowEditPolicy, &stubCeiling{})
		plain, err := affordances.New(&metamodel.Metamodel{}, storeRelationLookup{memstore.New()}, w.decl)
		require.NoError(t, err)
		require.Equal(t, ctx, affordances.PrimeFlowRuns(ctx, plain, rows))
	})
}

// TestWriteGate_WriteRecheck pins the check the manager repeats inside a
// write's transaction (TKT-XI7IDBS): only the flow edit ceiling, with the
// permissions its rules ask for answered before the transaction.
func TestWriteGate_WriteRecheck(t *testing.T) {
	const melden = "outside_edits:flow=melden state=intake"
	ernst := map[string]any{"ernst": "hoog"}
	tests := []struct {
		name    string
		policy  string
		ceiling *stubCeiling
		row     *entity.Entity
		set     map[string]any
		unset   []string
		noCheck bool
		want    string // attribution of the refusal; empty means admitted
	}{
		{name: "ungoverned type", policy: flowEditPolicy, ceiling: &stubCeiling{},
			row: entity.New("N-1", "note"), set: map[string]any{"text": "x"}, noCheck: true},
		{name: "no property named", policy: flowEditPolicy, ceiling: &stubCeiling{}, noCheck: true},
		{name: "withheld set", policy: flowEditPolicy, ceiling: &stubCeiling{withheld: map[string]string{"ernst": melden}},
			set: ernst, want: "affordance:read-only role=" + melden},
		{name: "withheld unset", policy: flowEditPolicy,
			ceiling: &stubCeiling{withheld: map[string]string{"ernst": melden}},
			unset:   []string{"ernst"}, want: "affordance:read-only role=" + melden},
		{name: "other property", policy: flowEditPolicy,
			ceiling: &stubCeiling{withheld: map[string]string{"ernst": melden}}, set: map[string]any{"notes": "x"}},
		{name: "permission not held", policy: flowEditPolicy, ceiling: &stubCeiling{permission: "privacy.officer"},
			set: map[string]any{"betrokkenen": "x"}, want: "affordance:read-only role=" + melden},
		{name: "permission held", policy: flowEditOfficerPolicy, ceiling: &stubCeiling{permission: "privacy.officer"},
			set: map[string]any{"betrokkenen": "x"}},
		{name: "lookup error fails closed", policy: flowEditPolicy, ceiling: &stubCeiling{err: errors.New("down")},
			set: ernst, want: "affordance:read-only role=outside_edits:unavailable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newFlowEditWorld(t, tc.policy, tc.ceiling)
			gate, err := affordances.NewWriteGate(w.resolver)
			require.NoError(t, err)
			row := tc.row
			if row == nil {
				row = w.row
			}
			recheck := gate.WriteRecheck(w.ctx(t), row, tc.set, tc.unset)
			if tc.noCheck {
				require.Nil(t, recheck)
				return
			}
			require.NotNil(t, recheck)
			// The check reads only the runs: the permissions were answered
			// above, so it needs no identity on its ctx.
			err = recheck(context.Background())
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			var fe *affordances.FieldWriteError
			require.ErrorAs(t, err, &fe)
			require.Equal(t, affordances.RuleFieldReadOnly, fe.Rule)
			require.Equal(t, tc.want, fe.Attribution)
		})
	}
}
