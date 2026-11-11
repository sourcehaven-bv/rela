package flowengine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/kvflowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const quickFlow = `
flows:
  quick:
    subject: person
    run_as: system:onboarding-bot
    start: [{ on: manual, permission: onboarding.start }]
    initial: a
    states:
      a:
        outputs:
          - set: { properties: { stage: "{{subject.name}}" } }
        on:
          done:
            - { to: b, when: "entity.documents == true" }
            - { to: c }
      b: { final: true }
      c: { final: true, status: failed }
`

// fakes records what the engine asked of its collaborators.
type fakes struct {
	mu       sync.Mutex
	subjects map[string]*entity.Entity
	// hidden lists the subjects run_as cannot read.
	hidden   map[string]bool
	headers  map[flow.RunID]map[string]any
	carries  map[flow.RunID]entity.Ref
	produced map[flow.RunID][]entity.Ref
	sets     []map[string]any
	creates  []map[string]any
	failSet  error
	// failCreate fails every create output.
	failCreate error
	// failEnd fails a header patch that records the end of a run.
	failEnd error
	next    int
}

func newFakes() *fakes {
	return &fakes{
		subjects: map[string]*entity.Entity{},
		hidden:   map[string]bool{},
		headers:  map[flow.RunID]map[string]any{},
		carries:  map[flow.RunID]entity.Ref{},
		produced: map[flow.RunID][]entity.Ref{},
	}
}

func (f *fakes) CreateHeader(_ context.Context, subject entity.Ref, props map[string]any) (flow.RunID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.subjects[subject.ID]; !ok {
		return "", ErrSubjectNotFound
	}
	f.next++
	id := flow.RunID(fmt.Sprintf("RUN-%d", f.next))
	f.headers[id] = props
	f.carries[id] = subject
	return id, nil
}

func (f *fakes) PatchHeader(_ context.Context, id flow.RunID, props map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	h, ok := f.headers[id]
	if !ok {
		return errors.New("no header")
	}
	if _, end := props[metamodel.FlowRunPropCompleted]; end && f.failEnd != nil {
		return f.failEnd
	}
	maps.Copy(h, props)
	return nil
}

func (f *fakes) DeleteHeader(_ context.Context, id flow.RunID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.headers, id)
	return nil
}

func (f *fakes) LinkProduced(_ context.Context, id flow.RunID, created entity.Ref) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.produced[id] = append(f.produced[id], created)
	return nil
}

func (f *fakes) Set(_ context.Context, runAs string, _ flow.RunID, _ string, subject entity.Ref,
	props map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSet != nil {
		return f.failSet
	}
	if runAs != "system:onboarding-bot" {
		return errors.New("wrong writer")
	}
	f.sets = append(f.sets, props)
	if s, ok := f.subjects[subject.ID]; ok {
		maps.Copy(s.Properties, props)
	}
	return nil
}

func (f *fakes) Create(_ context.Context, _ string, typ string, props map[string]any, relate string,
	subject entity.Ref) (entity.Ref, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCreate != nil {
		return entity.Ref{}, f.failCreate
	}
	f.next++
	props["_relate"] = relate + "->" + subject.ID
	props["_type"] = typ
	f.creates = append(f.creates, props)
	return entity.Ref{ID: fmt.Sprintf("TSK-%d", f.next)}, nil
}

func (f *fakes) Subject(_ context.Context, ref entity.Ref) (*entity.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.subjects[ref.ID]
	if !ok {
		return nil, ErrSubjectNotFound
	}
	c := *s
	return &c, nil
}

func (f *fakes) SubjectAs(_ context.Context, _ string, ref entity.Ref) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.subjects[ref.ID]
	if !ok || f.hidden[ref.ID] {
		return map[string]any{}, nil
	}
	return s.Properties, nil
}

func (f *fakes) addPerson(id string, props map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := entity.New(id, "person")
	e.Properties = props
	f.subjects[id] = e
}

func (f *fakes) header(id flow.RunID) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.headers[id]
}

type harness struct {
	eng   *Engine
	f     *fakes
	runs  flowrun.Store
	clock *time.Time
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../flowcfg/testdata/" + name)
	require.NoError(t, err)
	return data
}

func newHarness(t *testing.T, flowsYAML string) harness {
	t.Helper()
	meta, err := metamodel.Parse(readFixture(t, "schema.yaml"))
	require.NoError(t, err)
	policy, err := acl.ParsePolicy(readFixture(t, "acl.yaml"), "acl.yaml")
	require.NoError(t, err)
	if flowsYAML == "" {
		flowsYAML = string(readFixture(t, "flows.yaml"))
	}
	compiled, err := flowcfg.Parse([]byte(flowsYAML), meta, policy)
	require.NoError(t, err)
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	require.NoError(t, err)
	runs, err := kvflowrun.New(state.NewFSKV(rfs), filelock.ProcessPrivate{})
	require.NoError(t, err)
	clock := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	f := newFakes()
	eng, err := New(Deps{
		Flows: compiled, Meta: meta, Runs: runs, Headers: f, Outputs: f, Subjects: f,
		Compile: func(name string, src []byte, version string) (*flowcfg.Flow, error) {
			return flowcfg.ParseFlow(name, src, version, meta, policy)
		},
		Now: func() time.Time { return clock },
	})
	require.NoError(t, err)
	return harness{eng: eng, f: f, runs: runs, clock: &clock}
}

func TestNew_RejectsMissingDeps(t *testing.T) {
	_, err := New(Deps{})
	require.Error(t, err)
}

func TestStart_SampleFlowEndToEnd(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada", "status": "hired"})

	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)

	hdr := h.f.header(id)
	require.Equal(t, "onboarding", hdr["flow"])
	require.Equal(t, "collect", hdr["state"])
	require.Equal(t, "input_required", hdr["status"])
	require.Equal(t, h.clock.Add(30*24*time.Hour), hdr["deadline"])
	require.Equal(t, entity.Ref{ID: "PER-1"}, h.f.carries[id])

	// The initial state's outputs: a set, then a related create whose
	// template read the subject through the run_as gate.
	require.Equal(t, []map[string]any{{"stage": "collecting"}}, h.f.sets)
	require.Len(t, h.f.creates, 1)
	require.Equal(t, "Collect documents for Ada", h.f.creates[0]["title"])
	require.Equal(t, "about->PER-1", h.f.creates[0]["_relate"])
	require.Len(t, h.f.produced[id], 1)

	run, err := h.runs.Get(ctx, id)
	require.NoError(t, err)
	require.Nil(t, run.Pending)
	require.Equal(t, h.clock.Add(7*24*time.Hour), run.Instance.NextDueAt)

	// per_subject: one refuses a second run.
	_, err = h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.ErrorIs(t, err, ErrSubjectBusy)

	// Nothing is due yet.
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Zero(t, n)

	// After a week the stalled timer escalates the run.
	*h.clock = h.clock.Add(7*24*time.Hour + time.Minute)
	n, err = h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "escalated", h.f.header(id)["state"])
	require.Equal(t, "input_required", h.f.header(id)["status"])

	// The run deadline (30d since the run) fails it.
	*h.clock = h.clock.Add(23 * 24 * time.Hour)
	n, err = h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "failed", h.f.header(id)["status"])
	require.Equal(t, "failed", h.f.header(id)["state"])
	require.Equal(t, *h.clock, h.f.header(id)["completed"])
	_, err = h.runs.Get(ctx, id)
	require.ErrorIs(t, err, flowrun.ErrNotFound)

	// The subject is free again.
	_, err = h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
}

func TestStart_DoneBranches(t *testing.T) {
	tests := []struct {
		name   string
		props  map[string]any
		state  string
		status string
	}{
		{"guard holds", map[string]any{"name": "Ada", "documents": true}, "b", "completed"},
		{"falls through", map[string]any{"name": "Bob"}, "c", "failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, quickFlow)
			ctx := context.Background()
			h.f.addPerson("PER-1", tc.props)
			id, err := h.eng.Start(ctx, "quick", entity.Ref{ID: "PER-1"})
			require.NoError(t, err)
			require.Equal(t, tc.state, h.f.header(id)["state"])
			require.Equal(t, tc.status, h.f.header(id)["status"])
			require.Equal(t, tc.props["name"], h.f.sets[0]["stage"])
			_, err = h.runs.Get(ctx, id)
			require.ErrorIs(t, err, flowrun.ErrNotFound)
		})
	}
}

func TestStart_TemplateSeesOnlyWhatRunAsReads(t *testing.T) {
	h := newHarness(t, quickFlow)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Secret"})
	h.f.hidden["PER-1"] = true
	_, err := h.eng.Start(ctx, "quick", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	// The whole placeholder had no value, so the set left stage alone.
	require.Equal(t, []map[string]any{{}}, h.f.sets)
}

func TestStart_Refusals(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	_, err := h.eng.Start(ctx, "nope", entity.Ref{ID: "PER-1"})
	require.ErrorIs(t, err, ErrUnknownFlow)
	_, err = h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-9"})
	require.ErrorIs(t, err, ErrSubjectNotFound)

	h.f.mu.Lock()
	h.f.subjects["TSK-1"] = entity.New("TSK-1", "task")
	h.f.mu.Unlock()
	_, err = h.eng.Start(ctx, "onboarding", entity.Ref{ID: "TSK-1"})
	require.ErrorIs(t, err, ErrSubjectNotFound)
	require.Empty(t, h.f.headers)
}

func TestStart_FailedOutputIsRetriedByDue(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	h.f.failSet = errors.New("refused")

	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	require.NotEmpty(t, id)
	run, err := h.runs.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, &flowrun.Pending{State: "collect", Failures: 1}, run.Pending)
	require.Equal(t, h.clock.Add(minBackoff), run.Instance.NextDueAt)
	require.Empty(t, h.f.creates)

	// The run is backed off: a round before the backoff ends leaves it.
	h.f.failSet = nil
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Zero(t, n)

	// After it, a round finishes the outputs.
	*h.clock = h.clock.Add(minBackoff)
	n, err = h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Len(t, h.f.sets, 1)
	require.Len(t, h.f.creates, 1)
	run, err = h.runs.Get(ctx, id)
	require.NoError(t, err)
	require.Nil(t, run.Pending)

	// A second round writes nothing again.
	n, err = h.eng.Due(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	require.Len(t, h.f.creates, 1)
}

func TestDue_BackoffDoublesAndCaps(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-2", map[string]any{"name": "Bo"})
	h.f.failSet = errors.New("refused")
	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-2"})
	require.Error(t, err)

	want := minBackoff
	for i := 1; i <= 10; i++ {
		run, gerr := h.runs.Get(ctx, id)
		require.NoError(t, gerr)
		require.Equal(t, i, run.Pending.Failures)
		require.Equal(t, h.clock.Add(want), run.Instance.NextDueAt, "failure %d", i)
		*h.clock = run.Instance.NextDueAt
		_, gerr = h.eng.Due(ctx)
		require.NoError(t, gerr)
		want = min(want*2, maxBackoff)
	}

	// A success resets the count.
	h.f.failSet = nil
	run, err := h.runs.Get(ctx, id)
	require.NoError(t, err)
	*h.clock = run.Instance.NextDueAt
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	run, err = h.runs.Get(ctx, id)
	require.NoError(t, err)
	require.Nil(t, run.Pending)
}

func TestDue_FailedCreateIsRetried(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	h.f.failCreate = errors.New("refused")
	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	run, err := h.runs.Get(ctx, id)
	require.NoError(t, err)
	// The set is done; the claim on the create was put back.
	require.Equal(t, 1, run.Pending.Done)
	require.Len(t, h.f.sets, 1)

	h.f.failCreate = nil
	*h.clock = run.Instance.NextDueAt
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Len(t, h.f.sets, 1, "the set is not repeated")
	require.Len(t, h.f.creates, 1)
}

func TestDue_FailedFinishIsRetried(t *testing.T) {
	h := newHarness(t, quickFlow)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	h.f.failEnd = errors.New("refused")
	id, err := h.eng.Start(ctx, "quick", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	run, err := h.runs.Get(ctx, id)
	require.NoError(t, err, "a run whose end was not recorded is kept")
	require.Equal(t, "c", run.Pending.State)
	require.Nil(t, h.f.header(id)["completed"])

	h.f.failEnd = nil
	*h.clock = run.Instance.NextDueAt
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "failed", h.f.header(id)["status"])
	require.Len(t, h.f.sets, 1, "the outputs are not repeated")
	_, err = h.runs.Get(ctx, id)
	require.ErrorIs(t, err, flowrun.ErrNotFound)
}

func TestCloseHeader(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	// A header whose run was never stored.
	orphan, err := h.f.CreateHeader(ctx, entity.Ref{ID: "PER-1"}, map[string]any{"status": "running"})
	require.NoError(t, err)
	require.NoError(t, h.eng.CloseHeader(ctx, orphan))
	require.Equal(t, "cancelled", h.f.header(orphan)["status"])

	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	require.Error(t, h.eng.CloseHeader(ctx, id), "a stored run is cancelled, not closed")
	require.NotEqual(t, "cancelled", h.f.header(id)["status"])
}

func TestCancel(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)

	require.NoError(t, h.eng.Cancel(ctx, id))
	require.Equal(t, "cancelled", h.f.header(id)["status"])
	require.Equal(t, "collect", h.f.header(id)["state"])
	_, err = h.eng.Run(ctx, id)
	require.ErrorIs(t, err, ErrRunNotFound)
	require.ErrorIs(t, h.eng.Cancel(ctx, id), ErrRunNotFound)
}

func TestDue_OlderVersionRunsOnItsSnapshot(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)

	// flows.yaml changes the stalled timer; the running run keeps 7d.
	edited := newHarness(t, replace(t, string(readFixture(t, "flows.yaml")), "period: 7d", "period: 14d"))
	edited.eng.d.Runs = h.runs
	edited.eng.d.Headers, edited.eng.d.Outputs, edited.eng.d.Subjects = h.f, h.f, h.f
	edited.eng.d.Now = func() time.Time { return h.clock.Add(7*24*time.Hour + time.Minute) }
	n, err := edited.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "escalated", h.f.header(id)["state"])
}

func TestDue_StrandedRunIsSkipped(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	id, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)

	h.eng.d.Flows = mustParse(t, replace(t, string(readFixture(t, "flows.yaml")), "period: 7d", "period: 8d"))
	h.eng.d.Compile = func(string, []byte, string) (*flowcfg.Flow, error) {
		return nil, errors.New("schema changed")
	}
	*h.clock = h.clock.Add(8 * 24 * time.Hour)
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	require.Equal(t, "collect", h.f.header(id)["state"])

	// A stranded run can still be cancelled.
	require.NoError(t, h.eng.Cancel(ctx, id))
	require.Equal(t, "cancelled", h.f.header(id)["status"])
}

func replace(t *testing.T, s, old, repl string) string {
	t.Helper()
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + repl + s[i+len(old):]
		}
	}
	t.Fatalf("no %q", old)
	return ""
}

func mustParse(t *testing.T, src string) *flowcfg.Compiled {
	t.Helper()
	meta, err := metamodel.Parse(readFixture(t, "schema.yaml"))
	require.NoError(t, err)
	policy, err := acl.ParsePolicy(readFixture(t, "acl.yaml"), "acl.yaml")
	require.NoError(t, err)
	c, err := flowcfg.Parse([]byte(src), meta, policy)
	require.NoError(t, err)
	return c
}
