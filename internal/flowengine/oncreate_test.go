package flowengine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// createFlows has two flows on person that start on create. twice has two
// create starts; both hold for a hired person, and it gets one run.
const createFlows = `
flows:
  once:
    subject: person
    run_as: system:onboarding-bot
    start:
      - on: create
        when: "entity.status == 'hired'"
    initial: a
    states:
      a:
        outputs:
          - set: { properties: { stage: started } }
        waiting_for: [hr]
        on: { change: { to: b, roles: [hr] } }
      b: { final: true }
  twice:
    subject: person
    per_subject: many
    start:
      - on: create
      - on: create
        when: "entity.status == 'hired'"
      - on: manual
        permission: onboarding.start
    initial: a
    states:
      a:
        waiting_for: [hr]
        on: { change: { to: b, roles: [hr] } }
      b: { final: true }
  tasks:
    subject: task
    start: [{ on: create }]
    initial: a
    states:
      a:
        waiting_for: [hr]
        on: { change: { to: b, roles: [hr] } }
      b: { final: true }
`

// errTest is an output failure a test injects.
var errTest = errors.New("output failed")

func TestStartCreated(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]any
		want  []string // flows with a run
	}{
		{name: "when holds", props: map[string]any{"name": "Ada", "status": "hired"}, want: []string{"once", "twice"}},
		{name: "when fails", props: map[string]any{"name": "Bob", "status": "new"}, want: []string{"twice"}},
		{name: "when unset", props: map[string]any{"name": "Cy"}, want: []string{"twice"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, createFlows)
			ctx := context.Background()
			h.f.addPerson("PER-1", tc.props)
			subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
			require.NoError(t, err)

			ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
			require.NoError(t, err)
			require.Len(t, ids, len(tc.want))
			var got []string
			for _, id := range ids {
				run, err := h.runs.Get(ctx, id)
				require.NoError(t, err)
				got = append(got, run.Flow)
				require.NotNil(t, run.Pending, "the run is stored unsettled; Settle or a due round settles it")
				require.Equal(t, flowrun.ViaStart, run.Log[0].Via)
			}
			require.Equal(t, tc.want, got)
			require.Empty(t, h.f.sets, "nothing is settled inside the create")
		})
	}
}

// TestStartCreated_ByRunAs pins who starts a run on create: the flow's
// run_as, not the principal that created the subject.
func TestStartCreated_ByRunAs(t *testing.T) {
	h := newHarness(t, createFlows)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"status": "hired"})
	subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)
	byFlow := map[string]string{}
	for _, id := range ids {
		run, err := h.runs.Get(ctx, id)
		require.NoError(t, err)
		byFlow[run.Flow] = run.Log[0].By
	}
	require.Equal(t, map[string]string{"once": "system:onboarding-bot", "twice": ""}, byFlow)
}

// TestStartCreated_SubjectBusySkips pins that a subject already carrying a
// run of a per_subject: one flow gets no second run, and the create is not
// failed for it.
func TestStartCreated_SubjectBusySkips(t *testing.T) {
	h := newHarness(t, createFlows)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"status": "hired"})
	subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	first, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)
	require.Len(t, first, 2)

	again, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)
	require.Len(t, again, 1, "once is per_subject: one and busy; twice is per_subject: many")
	run, err := h.runs.Get(ctx, again[0])
	require.NoError(t, err)
	require.Equal(t, "twice", run.Flow)
}

// TestStartCreated_GuardErrorSkips pins that a `when:` that fails to
// evaluate starts nothing rather than failing the create.
func TestStartCreated_GuardErrorSkips(t *testing.T) {
	h := newHarness(t, `
flows:
  bad:
    subject: person
    start:
      - on: create
        when: "entity.started > today()"
    initial: a
    states:
      a:
        waiting_for: [hr]
        on: { change: { to: b, roles: [hr] } }
      b: { final: true }
`)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"started": "not a date"})
	subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)
	require.Empty(t, ids)
}

// TestSettle pins that Settle writes the initial outputs of a run started
// on create, and is a no-op for a settled or a missing run.
func TestSettle(t *testing.T) {
	h := newHarness(t, createFlows)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"status": "hired"})
	subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)

	for _, id := range ids {
		require.NoError(t, NewOnCreate(h.eng).Settle(ctx, id))
		run, err := h.runs.Get(ctx, id)
		require.NoError(t, err)
		require.Nil(t, run.Pending)
	}
	require.Equal(t, []map[string]any{{"stage": "started"}}, h.f.sets)

	// Again: nothing to settle, so the outputs are not written twice.
	for _, id := range ids {
		require.NoError(t, NewOnCreate(h.eng).Settle(ctx, id))
	}
	require.Len(t, h.f.sets, 1)
	require.NoError(t, NewOnCreate(h.eng).Settle(ctx, flow.RunID("RUN-404")))
}

// TestSettle_FailureBacksOff pins that a run that fails to settle keeps
// its mark and is backed off, so the flow worker settles it later.
func TestSettle_FailureBacksOff(t *testing.T) {
	h := newHarness(t, createFlows)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"status": "hired"})
	subj, err := h.f.Subject(ctx, entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	ids, err := NewOnCreate(h.eng).StartCreated(ctx, h.runs, subj, 0)
	require.NoError(t, err)
	h.f.failSet = errTest
	var once flow.RunID
	for _, id := range ids {
		run, getErr := h.runs.Get(ctx, id)
		require.NoError(t, getErr)
		if run.Flow == "once" {
			once = id
		}
	}
	require.ErrorIs(t, NewOnCreate(h.eng).Settle(ctx, once), errTest)
	run, err := h.runs.Get(ctx, once)
	require.NoError(t, err)
	require.NotNil(t, run.Pending)
	require.Equal(t, h.clock.Add(minBackoff), run.Instance.NextDueAt)

	h.f.failSet = nil
	*h.clock = h.clock.Add(minBackoff)
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n, "the due round settles both runs: once after its backoff, twice never settled")
	require.Len(t, h.f.sets, 1)
}
