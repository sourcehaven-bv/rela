package flowedits_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowedits"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const schema = `version: "1"
entities:
  case:
    label: Case
    id_type: manual
    properties:
      ernst: { type: string }
      notes: { type: string }
      betrokkenen: { type: string }
      count: { type: integer, computed: "1" }
  other:
    label: Other
    id_type: manual
    properties:
      ernst: { type: string }
`

const policy = `roles:
  officer:
    read: ["*"]
    update: ["*"]
    permissions: [privacy.officer]
`

// flows declares two flows on case with outside_edits, a third without, and
// a flow on other with outside_edits.
const flows = `flows:
  melden:
    subject: case
    per_subject: many
    start: [{on: create}]
    initial: intake
    outside_edits:
      default: allow
      properties:
        ernst: deny
        betrokkenen: { permission: privacy.officer }
      states:
        melden-ap: { default: deny }
    states:
      intake:
        on: { change: { to: melden-ap, when: "entity.notes == 'x'" } }
      melden-ap:
        on: { change: { to: done, when: "entity.notes == 'y'" } }
      done: { final: true }
  review:
    subject: case
    start: [{on: create}]
    initial: open
    outside_edits:
      properties:
        notes: deny
    states:
      open:
        on: { change: { to: closed, when: "entity.notes == 'z'" } }
      closed: { final: true }
  plain:
    subject: case
    start: [{on: create}]
    initial: open
    states:
      open:
        on: { change: { to: closed, when: "entity.notes == 'z'" } }
      closed: { final: true }
`

func compile(t *testing.T, src string) (*flowcfg.Compiled, *metamodel.Metamodel) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	require.NoError(t, err)
	pol, err := acl.ParsePolicy([]byte(policy), "acl.yaml")
	require.NoError(t, err)
	c, err := flowcfg.Parse([]byte(src), meta, pol)
	require.NoError(t, err)
	return c, meta
}

// fakeRuns is a run store that records its List calls.
type fakeRuns struct {
	mu    sync.Mutex
	runs  []flowrun.Run
	calls []flowrun.Query
	err   error
}

func (f *fakeRuns) add(id, flowName, subject, state string) {
	f.runs = append(f.runs, flowrun.Run{ID: flowrun.RunID(id), Flow: flowName,
		Subject: entity.Ref{ID: subject, Face: entity.ImplicitFace}, Instance: flow.Instance{State: state}})
	slices.SortFunc(f.runs, func(a, b flowrun.Run) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
}

func (f *fakeRuns) List(_ context.Context, q flowrun.Query) ([]flowrun.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := flowrun.CheckQuery(q); err != nil {
		return nil, err
	}
	f.calls = append(f.calls, q)
	if f.err != nil {
		return nil, f.err
	}
	out := []flowrun.Run{}
	for _, r := range f.runs {
		inSubjects := len(q.SubjectIDs) == 0 || slices.Contains(q.SubjectIDs, r.Subject.ID)
		if r.ID > q.After && inSubjects && len(out) < q.Limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRuns) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func newCeiling(t *testing.T, runs flowedits.RunLister) *flowedits.Ceiling {
	t.Helper()
	c, meta := compile(t, flows)
	ceil, err := flowedits.New(c, meta, runs)
	require.NoError(t, err)
	require.NotNil(t, ceil)
	return ceil
}

func caseRow(id string) *entity.Entity { return &entity.Entity{ID: id, Type: "case"} }

func TestCeiling_Withheld(t *testing.T) {
	holdsNone := func(string) bool { return false }
	holdsOfficer := func(p string) bool { return p == "privacy.officer" }
	tests := []struct {
		name   string
		runs   [][4]string // id, flow, subject, state
		within flowrun.RunID
		holds  func(string) bool
		row    *entity.Entity
		want   map[string]string
	}{
		{name: "no run", holds: holdsNone, row: caseRow("C-1")},
		{name: "deny and permission not held", runs: [][4]string{{"FR-1", "melden", "C-1", "intake"}},
			holds: holdsNone, row: caseRow("C-1"), want: map[string]string{
				"ernst":       "outside_edits:flow=melden state=intake",
				"betrokkenen": "outside_edits:flow=melden state=intake",
			}},
		{name: "a run that has not left its start is in the initial state",
			runs:  [][4]string{{"FR-1", "melden", "C-1", ""}},
			holds: holdsNone, row: caseRow("C-1"), want: map[string]string{
				"ernst":       "outside_edits:flow=melden state=intake",
				"betrokkenen": "outside_edits:flow=melden state=intake",
			}},
		{name: "permission held", runs: [][4]string{{"FR-1", "melden", "C-1", "intake"}},
			holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst": "outside_edits:flow=melden state=intake",
			}},
		{name: "state default overrides permission", runs: [][4]string{{"FR-1", "melden", "C-1", "melden-ap"}},
			holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst":       "outside_edits:flow=melden state=melden-ap",
				"notes":       "outside_edits:flow=melden state=melden-ap",
				"betrokkenen": "outside_edits:flow=melden state=melden-ap",
			}},
		{name: "two flows intersect", runs: [][4]string{
			{"FR-1", "melden", "C-1", "intake"}, {"FR-2", "review", "C-1", "open"}},
			holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst": "outside_edits:flow=melden state=intake",
				"notes": "outside_edits:flow=review state=open",
			}},
		{name: "two runs of one flow in different states", runs: [][4]string{
			{"FR-1", "melden", "C-1", "intake"}, {"FR-2", "melden", "C-1", "melden-ap"}},
			holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst":       "outside_edits:flow=melden state=intake",
				"notes":       "outside_edits:flow=melden state=melden-ap",
				"betrokkenen": "outside_edits:flow=melden state=melden-ap",
			}},
		{name: "own run exempt only", runs: [][4]string{
			{"FR-1", "melden", "C-1", "melden-ap"}, {"FR-2", "review", "C-1", "open"}},
			within: "FR-1", holds: holdsNone, row: caseRow("C-1"), want: map[string]string{
				"notes": "outside_edits:flow=review state=open",
			}},
		{name: "another run's mark exempts nothing", runs: [][4]string{{"FR-1", "melden", "C-1", "intake"}},
			within: "FR-9", holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst": "outside_edits:flow=melden state=intake",
			}},
		{name: "stranded state uses the flow rules", runs: [][4]string{{"FR-1", "melden", "C-1", "gone"}},
			holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst": "outside_edits:flow=melden state=gone",
			}},
		{name: "final state uses the flow rules", runs: [][4]string{{"FR-1", "melden", "C-1", "done"}},
			holds: holdsOfficer, row: caseRow("C-1"), want: map[string]string{
				"ernst": "outside_edits:flow=melden state=done",
			}},
		{name: "removed flow restricts nothing", runs: [][4]string{{"FR-1", "gone", "C-1", "intake"}},
			holds: holdsNone, row: caseRow("C-1")},
		{name: "flow without outside_edits restricts nothing", runs: [][4]string{{"FR-1", "plain", "C-1", "open"}},
			holds: holdsNone, row: caseRow("C-1")},
		{name: "run on another subject", runs: [][4]string{{"FR-1", "melden", "C-2", "intake"}},
			holds: holdsNone, row: caseRow("C-1")},
		{name: "type no flow governs", runs: [][4]string{{"FR-1", "melden", "C-1", "intake"}},
			holds: holdsNone, row: &entity.Entity{ID: "C-1", Type: "other"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runs := &fakeRuns{}
			for _, r := range tc.runs {
				runs.add(r[0], r[1], r[2], r[3])
			}
			ceil := newCeiling(t, runs)
			ctx := context.Background()
			if tc.within != "" {
				ctx = flowedits.WithinRun(ctx, tc.within)
			}
			got, err := ceil.Withheld(ctx, tc.row, tc.holds)
			require.NoError(t, err)
			if len(tc.want) == 0 {
				require.Empty(t, got)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCeiling_Permissions(t *testing.T) {
	ceil := newCeiling(t, &fakeRuns{})
	require.Equal(t, []string{"privacy.officer"}, ceil.Permissions("case"))
	require.Empty(t, ceil.Permissions("note"))
	var none *flowedits.Ceiling
	require.Nil(t, none.Permissions("case"))
}

func TestCeiling_LookupError(t *testing.T) {
	runs := &fakeRuns{err: errors.New("down")}
	ceil := newCeiling(t, runs)
	_, err := ceil.Withheld(context.Background(), caseRow("C-1"), func(string) bool { return true })
	require.ErrorContains(t, err, "down")

	_, err = ceil.PrimeRuns(context.Background(), []string{"C-1"})
	require.ErrorContains(t, err, "down")
}

func TestCeiling_PrimeRuns(t *testing.T) {
	runs := &fakeRuns{}
	runs.add("FR-1", "melden", "C-1", "intake")
	ceil := newCeiling(t, runs)

	ctx, err := ceil.PrimeRuns(context.Background(), []string{"C-1", "C-2"})
	require.NoError(t, err)
	require.Equal(t, 1, runs.callCount())

	holds := func(string) bool { return true }
	got, err := ceil.Withheld(ctx, caseRow("C-1"), holds)
	require.NoError(t, err)
	require.Equal(t, []string{"ernst"}, slices.Sorted(maps.Keys(got)))
	got, err = ceil.Withheld(ctx, caseRow("C-2"), holds)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Equal(t, 1, runs.callCount(), "primed rows look up nothing")

	_, err = ceil.Withheld(ctx, caseRow("C-3"), holds)
	require.NoError(t, err)
	require.Equal(t, 2, runs.callCount(), "an unprimed row looks up its own runs")

	_, err = ceil.Withheld(store.ContextInTx(ctx), caseRow("C-1"), holds)
	require.NoError(t, err)
	require.Equal(t, 3, runs.callCount(), "inside a store transaction a primed row looks up again")
}

func TestCeiling_Batches(t *testing.T) {
	tests := []struct {
		name      string
		ids       int
		runs      int
		wantCalls int
	}{
		{name: "one batch", ids: 3, runs: 3, wantCalls: 1},
		{name: "ids over a page split", ids: flowrun.MaxPage + 1, runs: 1, wantCalls: 2},
		{name: "runs over a page are paged", ids: 1, runs: flowrun.MaxPage + 1, wantCalls: 2},
		{name: "exactly a page reads one more", ids: 1, runs: flowrun.MaxPage, wantCalls: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runs := &fakeRuns{}
			ids := make([]string, tc.ids)
			for i := range ids {
				ids[i] = fmt.Sprintf("C-%04d", i)
			}
			for i := range tc.runs {
				runs.add(fmt.Sprintf("FR-%04d", i), "melden", ids[i%len(ids)], "intake")
			}
			ceil := newCeiling(t, runs)
			// Repeated ids are looked up once.
			ctx, err := ceil.PrimeRuns(context.Background(), append(slices.Clone(ids), ids[0]))
			require.NoError(t, err)
			require.Equal(t, tc.wantCalls, runs.callCount())
			for _, q := range runs.calls {
				require.LessOrEqual(t, len(q.SubjectIDs), flowrun.MaxPage)
			}
			// Every run was found: the last subject with a run is withheld.
			got, err := ceil.Withheld(ctx, caseRow(ids[(tc.runs-1)%len(ids)]), func(string) bool { return true })
			require.NoError(t, err)
			require.Contains(t, got, "ernst")
			require.Equal(t, tc.wantCalls, runs.callCount())
		})
	}
}

func TestNew(t *testing.T) {
	c, meta := compile(t, flows)
	_, err := flowedits.New(c, meta, nil)
	require.Error(t, err, "a run store is required when a flow declares outside_edits")
	_, err = flowedits.New(c, nil, &fakeRuns{})
	require.Error(t, err)

	none, err := flowedits.New(nil, meta, nil)
	require.NoError(t, err)
	require.Nil(t, none)

	plainOnly, meta := compile(t, `flows:
  plain:
    subject: case
    start: [{on: create}]
    initial: open
    states:
      open:
        on: { change: { to: closed, when: "entity.notes == 'z'" } }
      closed: { final: true }
`)
	none, err = flowedits.New(plainOnly, meta, nil)
	require.NoError(t, err)
	require.Nil(t, none)
	require.False(t, none.Governs("case"), "a nil ceiling governs nothing")

	ceil := newCeiling(t, &fakeRuns{})
	require.True(t, ceil.Governs("case"))
	require.False(t, ceil.Governs("other"))
}
