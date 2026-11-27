package flowengine

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// person returns a person PER-1 with props.
func person(props map[string]any) *entity.Entity {
	e := entity.New("PER-1", "person")
	e.Properties = props
	return e
}

// edit returns a copy of old with props set on it.
func edit(old *entity.Entity, props map[string]any) *entity.Entity {
	e := person(maps.Clone(old.Properties))
	maps.Copy(e.Properties, props)
	return e
}

// holds returns a role check that holds roles.
func holds(roles ...string) func(string) bool {
	return func(r string) bool { return slices.Contains(roles, r) }
}

// stateOf returns the state run's own token is in.
func stateOf(t *testing.T, h harness, run flowrun.Run) string {
	t.Helper()
	f, err := h.eng.flowOf(context.Background(), run)
	require.NoError(t, err)
	return flow.Read(f.Definition(), run.Instance, run.CreatedAt).State
}

// write is an ordinary write by alice, who holds hr, from old to new.
func write(h harness, old, e *entity.Entity) Write {
	return Write{Runs: h.runs, Old: old, New: e, By: "alice", Holds: holds("hr"), Starts: true}
}

func TestChanged_FiresTheChangeEvent(t *testing.T) {
	old := person(map[string]any{"name": "Ada"})
	tests := []struct {
		name  string
		w     func(Write) Write
		props map[string]any
		moved bool
	}{
		{name: "guard holds, role held", props: map[string]any{"documents": true}, moved: true},
		{name: "role not held", props: map[string]any{"documents": true},
			w: func(w Write) Write { w.Holds = holds("manager"); return w }},
		{name: "guard does not hold", props: map[string]any{"documents": false}},
		{name: "no property changed", props: map[string]any{}},
		{name: "the write changed no property the guard reads", props: map[string]any{"name": "Bea"}},
		{name: "a run wrote it", props: map[string]any{"documents": true},
			w: func(w Write) Write { w.Machine = true; return w }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, "")
			ctx := context.Background()
			run := startSample(t, h, maps.Clone(old.Properties))
			w := write(h, old, edit(old, tc.props))
			if tc.w != nil {
				w = tc.w(w)
			}
			ch, err := NewOnChange(h.eng).Changed(ctx, w)
			require.NoError(t, err)
			stored, err := h.runs.Get(ctx, run.ID)
			require.NoError(t, err)
			if !tc.moved {
				require.Empty(t, ch.Settle)
				require.Equal(t, "collect", stateOf(t, h, stored))
				return
			}
			require.Equal(t, []flow.RunID{run.ID}, ch.Settle)
			require.Equal(t, "review", stateOf(t, h, stored))
			require.NotNil(t, stored.Pending, "the move settles after the commit")
			last := stored.Log[len(stored.Log)-1]
			require.Equal(t, flowrun.ViaChange, last.Via)
			require.Equal(t, "alice", last.By)

			require.NoError(t, NewOnChange(h.eng).Settle(ctx, run.ID))
			_, err = h.runs.Get(ctx, run.ID)
			require.ErrorIs(t, err, flowrun.ErrNotFound, "review is done at once and the run finished")
		})
	}
}

// TestChanged_SkipsTheStepWritesRun pins that a person's step write fires
// no event on the run whose step it completes.
func TestChanged_SkipsTheStepWritesRun(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	old := person(map[string]any{"name": "Ada"})
	run := startSample(t, h, maps.Clone(old.Properties))
	w := write(h, old, edit(old, map[string]any{"documents": true}))
	w.Skip = run.ID
	ch, err := NewOnChange(h.eng).Changed(ctx, w)
	require.NoError(t, err)
	require.Empty(t, ch.Settle)
	stored, err := h.runs.Get(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "collect", stateOf(t, h, stored))
}

// TestChanged_BranchWithoutWhenIsNotTaken pins that a `change` branch
// with no `when:` is taken only by a step, never by an ordinary write.
func TestChanged_BranchWithoutWhenIsNotTaken(t *testing.T) {
	h := newHarness(t, createFlows)
	ctx := context.Background()
	old := person(map[string]any{"status": "hired"})
	h.f.addPerson("PER-1", maps.Clone(old.Properties))
	id, err := h.eng.Start(ctx, "once", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	ch, err := NewOnChange(h.eng).Changed(ctx, write(h, old, edit(old, map[string]any{"documents": true})))
	require.NoError(t, err)
	require.Empty(t, ch.Settle)
	stored, err := h.runs.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "a", stateOf(t, h, stored))
}

// TestChanged_ClaimedStepIsLeftToItsClaimant pins that a write by another
// principal does not move a step someone has claimed, and that the
// claimant's own write does.
func TestChanged_ClaimedStepIsLeftToItsClaimant(t *testing.T) {
	for _, by := range []string{"bob", "alice"} {
		t.Run(by, func(t *testing.T) {
			h := newHarness(t, "")
			ctx := context.Background()
			old := person(map[string]any{"name": "Ada"})
			run := startSample(t, h, maps.Clone(old.Properties))
			require.NoError(t, h.eng.Claim(ctx, run.ID, run.Instance.Generation, "alice"))
			w := write(h, old, edit(old, map[string]any{"documents": true}))
			w.By = by
			_, err := NewOnChange(h.eng).Changed(ctx, w)
			require.NoError(t, err)
			stored, err := h.runs.Get(ctx, run.ID)
			require.NoError(t, err)
			want := map[string]string{"bob": "collect", "alice": "review"}[by]
			require.Equal(t, want, stateOf(t, h, stored))
		})
	}
}

// pendingFlow starts on create and waits in a for a change of documents.
const pendingFlow = `
flows:
  pend:
    subject: person
    run_as: system:onboarding-bot
    start: [{ on: create }]
    initial: a
    states:
      a:
        waiting_for: [hr]
        outputs:
          - set: { properties: { stage: a } }
        on:
          change: { to: b, roles: [hr], when: "entity.documents == true" }
      b:
        waiting_for: [hr]
        on: { change: { to: c, roles: [hr] } }
      c: { final: true }
`

// TestChanged_KeptWhileSettling pins that an event on a token that is
// still settling is kept on its pending mark and taken when it settles,
// and dropped when its guard no longer holds on the subject by then.
func TestChanged_KeptWhileSettling(t *testing.T) {
	tests := []struct {
		name  string
		after map[string]any
		state string
	}{
		{name: "taken", after: map[string]any{"name": "Ada", "documents": true}, state: "b"},
		{name: "guard fails by then", after: map[string]any{"name": "Ada", "documents": false}, state: "a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { keptWhileSettling(t, tc.after, tc.state) })
	}
}

func keptWhileSettling(t *testing.T, after map[string]any, state string) {
	t.Helper()
	h := newHarness(t, pendingFlow)
	ctx := context.Background()
	old := person(map[string]any{"name": "Ada"})
	h.f.addPerson("PER-1", maps.Clone(old.Properties))
	subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)
	require.Len(t, ids, 1)

	ch, err := NewOnChange(h.eng).Changed(ctx, write(h, old, edit(old, map[string]any{"documents": true})))
	require.NoError(t, err)
	require.Equal(t, ids, ch.Settle)
	stored, err := h.runs.Get(ctx, ids[0])
	require.NoError(t, err)
	require.Equal(t, "a", stateOf(t, h, stored))
	require.NotNil(t, stored.Pending)
	require.NotNil(t, stored.Pending.Change, "the event waits on the mark")
	require.Equal(t, "alice", stored.Pending.Change.By)

	h.f.addPerson("PER-1", after)
	require.NoError(t, NewOnChange(h.eng).Settle(ctx, ids[0]))
	stored, err = h.runs.Get(ctx, ids[0])
	require.NoError(t, err)
	require.Equal(t, state, stateOf(t, h, stored))
	require.Nil(t, stored.Pending)
	require.Equal(t, []map[string]any{{"stage": "a"}}, h.f.sets, "the outputs of a ran")
	if state == "a" {
		return
	}
	last := stored.Log[len(stored.Log)-1]
	require.Equal(t, flowrun.ViaChange, last.Via)
	require.Equal(t, "alice", last.By)
}

// TestChanged_ForkedRun pins that a write moves the branch whose
// `change` it satisfies, and that settling then fires the join.
func TestChanged_ForkedRun(t *testing.T) {
	h, run := startFork(t, "join: any", map[string]any{"name": "Ada"})
	ctx := context.Background()
	old := person(map[string]any{"name": "Ada"})
	w := write(h, old, edit(old, map[string]any{"documents": true}))
	w.Holds = holds("manager")
	ch, err := NewOnChange(h.eng).Changed(ctx, w)
	require.NoError(t, err)
	require.Equal(t, []flow.RunID{run.ID}, ch.Settle)
	require.NoError(t, NewOnChange(h.eng).Settle(ctx, run.ID))
	require.Equal(t, "completed", h.f.header(run.ID)["status"], "legal arrived and the any join fired")
}

func TestChanged_StartsOnAPredicate(t *testing.T) {
	tests := []struct {
		name   string
		old    map[string]any
		props  map[string]any
		starts bool
		want   int
	}{
		{name: "comes to hold", old: map[string]any{"status": "new"}, props: map[string]any{"status": "hired"},
			starts: true, want: 1},
		{name: "held already", old: map[string]any{"status": "hired"}, props: map[string]any{"name": "Bea"},
			starts: true},
		{name: "does not hold", old: map[string]any{"status": "new"}, props: map[string]any{"status": "left"},
			starts: true},
		{name: "chain at its bound", old: map[string]any{"status": "new"}, props: map[string]any{"status": "hired"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, "")
			ctx := context.Background()
			old := person(tc.old)
			h.f.addPerson("PER-1", maps.Clone(tc.old))
			w := write(h, old, edit(old, tc.props))
			w.Starts = tc.starts
			ch, err := NewOnChange(h.eng).Changed(ctx, w)
			require.NoError(t, err)
			require.Len(t, ch.Started, tc.want)
			require.Equal(t, ch.Started, ch.Settle)
			for _, id := range ch.Started {
				run, err := h.runs.Get(ctx, id)
				require.NoError(t, err)
				require.Equal(t, "onboarding", run.Flow)
				require.NotNil(t, run.Pending)
				require.Equal(t, "system:onboarding-bot", run.Log[0].By)
			}
		})
	}
}

// TestChanged_PredicateSubjectBusy pins that a predicate start on a
// subject that already carries a run of a per_subject: one flow starts
// nothing and does not fail the write.
func TestChanged_PredicateSubjectBusy(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	old := person(map[string]any{"status": "new"})
	startSample(t, h, maps.Clone(old.Properties))
	ch, err := NewOnChange(h.eng).Changed(ctx, write(h, old, edit(old, map[string]any{"status": "hired"})))
	require.NoError(t, err)
	require.Empty(t, ch.Started)
}

// TestStartCreated_Predicate pins that a create that matches an
// `on: predicate` start starts its run.
func TestStartCreated_Predicate(t *testing.T) {
	h := newHarness(t, `
flows:
  pred:
    subject: person
    start: [{ on: predicate, when: "entity.status == 'hired'" }]
    initial: a
    states:
      a:
        waiting_for: [hr]
        on: { change: { to: b, roles: [hr], when: "entity.documents == true" } }
      b: { final: true }
`)
	ctx := context.Background()
	for _, tc := range []struct {
		status string
		want   int
	}{{"hired", 1}, {"new", 0}} {
		h.f.addPerson("PER-"+tc.status, map[string]any{"status": tc.status})
		subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-" + tc.status})
		require.NoError(t, err)
		ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
		require.NoError(t, err)
		require.Len(t, ids, tc.want, tc.status)
	}
}

// TestEventBranchCap pins that a kept event can name every `change`
// branch a state may declare.
func TestEventBranchCap(t *testing.T) {
	require.GreaterOrEqual(t, flowrun.MaxEventBranches, flowcfg.MaxBranches)
}

// TestChangedProps_TypeMismatch pins that a value read back with another
// Go type but the same JSON value counts as unchanged.
func TestChangedProps_TypeMismatch(t *testing.T) {
	old := person(map[string]any{"n": 1, "tags": []string{"a"}, "x": "y"})
	e := edit(old, map[string]any{"n": float64(1), "tags": []any{"a"}, "x": "z"})
	require.Equal(t, map[string]bool{"x": true}, ChangedProps(old, e))
}
