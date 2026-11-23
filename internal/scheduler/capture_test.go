package scheduler

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// captureMeta declares a metric owner and record types for capture tests.
func captureMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{SideStores: metamodel.SideStores{
		Records: map[string]metamodel.RecordTypeDef{
			"measurement": {Owner: "metric", Fields: map[string]metamodel.PropertyDef{
				"value": {Type: metamodel.PropertyTypeDecimal, Scale: new(1), Required: true},
				"note":  {Type: metamodel.PropertyTypeString},
			}},
			"tally": {Owner: "metric", Fields: map[string]metamodel.PropertyDef{
				"n": {Type: metamodel.PropertyTypeInteger},
			}},
			"loose": {Owner: "metric", Fields: map[string]metamodel.PropertyDef{
				"v": {Type: metamodel.PropertyTypeDecimal},
			}},
			"strict": {Owner: "metric", Fields: map[string]metamodel.PropertyDef{
				"v":     {Type: metamodel.PropertyTypeDecimal},
				"other": {Type: metamodel.PropertyTypeString, Required: true},
			}},
			"book": {Owner: "metric", Ledger: &metamodel.LedgerDef{}, Fields: map[string]metamodel.PropertyDef{
				"amount": {Type: metamodel.PropertyTypeDecimal},
			}},
		},
	}}
}

func TestParseConfig_Capture(t *testing.T) {
	t.Parallel()
	cfg, err := ParseConfig([]byte(`
tasks:
  - name: coverage
    every: monday
    run_as: system:metrics
    capture:
      dashboard: ops
      card: Control coverage
      record: measurement
      owner: MET-1
      field: value
`))
	require.NoError(t, err)
	require.Len(t, cfg.Tasks, 1)
	assert.Equal(t, &CaptureConfig{
		Dashboard: "ops", Card: "Control coverage", Record: "measurement", Owner: "MET-1", Field: "value",
	}, cfg.Tasks[0].Capture)
	assert.Equal(t, []string{"coverage"}, cfg.CaptureTasks())
}

func TestParseConfig_CaptureRejectsBadShapes(t *testing.T) {
	t.Parallel()
	const capture = `
    capture: {card: C, record: measurement, owner: MET-1, field: value}`
	tests := []struct {
		name, task, want string
	}{
		{"with script", "script: a.lua\n    run_as: system:m" + capture, "exactly one of script, template or capture"},
		{"without run_as", capture, "capture requires run_as"},
		{"shared scheduler identity", "run_as: system:scheduler" + capture, "not the shared system:scheduler"},
		{"with for_each", "for_each: {entity_type: person}" + capture, "capture and for_each are mutually exclusive"},
		{"with capabilities", "run_as: system:m\n    capabilities: {http: true}" + capture, "takes no capabilities"},
		{"without card", "run_as: system:m\n    capture: {record: r, owner: o, field: f}", "capture.card is required"},
		{"without record", "run_as: system:m\n    capture: {card: c, owner: o, field: f}", "capture.record is required"},
		{"without owner", "run_as: system:m\n    capture: {card: c, record: r, field: f}", "capture.owner is required"},
		{"without field", "run_as: system:m\n    capture: {card: c, record: r, owner: o}", "capture.field is required"},
		{"unknown key", "run_as: system:m\n    capture: {card: c, record: r, owner: o, field: f, x: 1}", "x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseConfig([]byte("tasks:\n  - name: t\n    every: day\n    " + tc.task + "\n"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateCaptures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, record, field, want string
	}{
		{"decimal field", "measurement", "value", ""},
		{"integer field", "tally", "n", ""},
		{"unknown record", "nope", "value", "not a record type"},
		{"ledger", "book", "amount", "is a ledger"},
		{"unknown field", "measurement", "nope", "not a field"},
		{"string field", "measurement", "note", "integer or decimal"},
		{"other required field", "strict", "v", "requires field \"other\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{Tasks: []TaskConfig{{Name: "t", Capture: &CaptureConfig{
				Card: "C", Record: tc.record, Owner: "MET-1", Field: tc.field,
			}}}}
			err := cfg.ValidateMetamodel(captureMeta())
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLoadConfig_CaptureCheckedAgainstSchema(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	ws.files["project:"+ConfigFile] = []byte(`
tasks:
  - name: t
    every: day
    run_as: system:m
    capture: {card: C, record: nope, owner: MET-1, field: value}
`)
	_, err := LoadConfig(context.Background(), ws.Config(), captureMeta())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a record type")
}

func TestFieldValue(t *testing.T) {
	t.Parallel()
	integer := metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}
	scale1 := metamodel.PropertyDef{Type: metamodel.PropertyTypeDecimal, Scale: new(1)}
	scale3 := metamodel.PropertyDef{Type: metamodel.PropertyTypeDecimal, Scale: new(3)}
	noScale := metamodel.PropertyDef{Type: metamodel.PropertyTypeDecimal}
	tests := []struct {
		name  string
		rat   *big.Rat
		field metamodel.PropertyDef
		want  any
	}{
		{"integer exact", big.NewRat(42, 1), integer, int64(42)},
		{"integer half away from zero", big.NewRat(5, 2), integer, int64(3)},
		{"negative integer half", big.NewRat(-5, 2), integer, int64(-3)},
		{"two thirds at scale 1", big.NewRat(200, 3), scale1, "66.7"},
		{"exact 1.249 at scale 1", big.NewRat(1249, 1000), scale1, "1.2"},
		{"scale 3", big.NewRat(1, 3), scale3, "0.333"},
		{"no scale means 2", big.NewRat(1, 8), noScale, "0.13"},
		{"zero", new(big.Rat), scale1, "0.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fieldValue(tc.rat, tc.field)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	_, err := fieldValue(new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(1), 70), big.NewInt(1)), integer)
	require.Error(t, err)
}

// captureWorkspace adds the capture schema to the mock workspace.
type captureWorkspace struct{ *mockWorkspace }

func (captureWorkspace) Meta() *metamodel.Metamodel { return captureMeta() }

// fakeCards answers a fixed figure and records the principal it ran as.
type fakeCards struct {
	mu     sync.Mutex
	figure *big.Rat
	err    error
	calls  []string
	users  []string
}

func (f *fakeCards) CardFigure(ctx context.Context, dashboard, card string) (*big.Rat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, dashboard+"/"+card)
	f.users = append(f.users, principal.From(ctx).User)
	return f.figure, f.err
}

// fakeAppender records appended rows.
type fakeAppender struct {
	mu   sync.Mutex
	typ  string
	rows []lua.RecordRow
	err  error
}

func (f *fakeAppender) AppendRecords(_ context.Context, typ string, rows []lua.RecordRow) ([]lua.AppendedRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.typ = typ
	f.rows = append(f.rows, rows...)
	return []lua.AppendedRecord{{ID: "r1", Owner: rows[0].Owner}}, nil
}

func captureTask() TaskConfig {
	return TaskConfig{
		Name: "coverage", Every: dailySchedule(), RunAs: "system:metrics",
		Capture: &CaptureConfig{Card: "Control coverage", Record: "measurement", Owner: "MET-1", Field: "value"},
	}
}

// newCaptureScheduler builds a scheduler with cards and app as its card
// evaluator and record appender.
func newCaptureScheduler(
	t *testing.T, cards CardEvaluator, app *fakeAppender, now time.Time, tasks ...TaskConfig,
) (*Scheduler, *fakeQueue, captureWorkspace) {
	t.Helper()
	mock := newMockWorkspace(t)
	if app != nil {
		mock.luaDepsProvider = func() lua.WriteDeps { return lua.WriteDeps{Records: app} }
	}
	ws := captureWorkspace{mock}
	s, err := New(&Config{Tasks: tasks}, script.NewEngine(), ws, testBinder(ws), discardLogger(), WithCards(cards))
	require.NoError(t, err)
	s.now = func() time.Time { return now }
	q := newFakeQueue()
	require.NoError(t, s.UseQueue(q))
	return s, q, ws
}

func TestCapture_AppendsTheRoundedFigureAsRunAs(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	cards := &fakeCards{figure: big.NewRat(200, 3)}
	app := &fakeAppender{}
	s, q, ws := newCaptureScheduler(t, cards, app, now, captureTask())

	s.tick(context.Background())
	queued := q.take()
	require.Len(t, queued, 1)
	assert.Equal(t, CaptureKind, queued[0].Kind)
	require.NoError(t, q.deliver(t, queued[0]))

	assert.Equal(t, []string{"/Control coverage"}, cards.calls)
	assert.Equal(t, []string{"system:metrics"}, cards.users)
	assert.Equal(t, "measurement", app.typ)
	require.Len(t, app.rows, 1)
	assert.Equal(t, "MET-1", app.rows[0].Owner)
	assert.Equal(t, map[string]any{"value": "66.7"}, app.rows[0].Fields)
	assert.True(t, app.rows[0].TS.Equal(readtime.Normalize(now)), "row time is the run's read time, got %v", app.rows[0].TS)

	ts, ok := taskState(t, ws, "coverage")
	require.True(t, ok)
	assert.False(t, ts.Failing())
	assert.Nil(t, ts.Active)
}

func TestCapture_FailureStoresNothing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		cards  *fakeCards
		app    *fakeAppender
		task   func(TaskConfig) TaskConfig
		wantIn string
	}{
		{"evaluation fails", &fakeCards{err: errors.New("no card titled")}, &fakeAppender{}, nil, "no card titled"},
		{"append refused", &fakeCards{figure: big.NewRat(1, 1)}, &fakeAppender{err: errors.New("forbidden")}, nil,
			"append to MET-1: forbidden"},
		{"no record service", &fakeCards{figure: big.NewRat(1, 1)}, nil, nil, "no record service"},
		{"record type gone from schema", &fakeCards{figure: big.NewRat(1, 1)}, &fakeAppender{},
			func(t TaskConfig) TaskConfig { t.Capture.Record = "gone"; return t }, "not a record type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			task := captureTask()
			if tc.task != nil {
				task = tc.task(task)
			}
			s, q, ws := newCaptureScheduler(t, tc.cards, tc.app, now, task)
			s.tick(context.Background())
			queued := q.take()
			require.Len(t, queued, 1)
			err := q.deliver(t, queued[0])
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantIn)
			assert.Contains(t, err.Error(), `"coverage"`)
			if tc.app != nil {
				assert.Empty(t, tc.app.rows)
			}
			ts, ok := taskState(t, ws, "coverage")
			require.True(t, ok)
			assert.True(t, ts.Failing(), "a failed capture enters the retry ladder")
		})
	}
}

func TestCapture_UnknownTask(t *testing.T) {
	t.Parallel()
	s, _, _ := newCaptureScheduler(t, &fakeCards{}, &fakeAppender{}, time.Now())
	err := s.capture(context.Background(), "gone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestCapture_NoEvaluatorSkipsCaptureTasks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	script := TaskConfig{Name: "report", Script: "r.lua", Every: dailySchedule()}
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, now, captureTask(), script)

	assert.Equal(t, []TaskConfig{script}, s.config.Load().Tasks, "capture tasks are left out without an evaluator")
	q.mu.Lock()
	_, registered := q.handlers[CaptureKind]
	q.mu.Unlock()
	assert.False(t, registered, "a process without an evaluator takes no capture runs")

	// A reload that adds a capture task leaves it out too.
	ws.files["project:"+ConfigFile] = []byte(`
tasks:
  - name: report
    script: r.lua
    every: day
  - name: coverage
    every: day
    run_as: system:metrics
    capture: {card: C, record: measurement, owner: MET-1, field: value}
`)
	s.WatchConfig()
	s.reloadConfig(context.Background())
	assert.Equal(t, []string{"report"}, taskNames(s.config.Load().Tasks))

	// And a run that reaches it anyway, on a config stored without the
	// filter, fails instead of computing nothing.
	s.config.Store(&Config{Tasks: []TaskConfig{captureTask()}})
	err := s.capture(context.Background(), captureTask().Name)
	require.ErrorContains(t, err, "computes no dashboard cards")
}

func TestCapture_WithEvaluatorRegistersTheKind(t *testing.T) {
	t.Parallel()
	_, q, _ := newCaptureScheduler(t, &fakeCards{}, &fakeAppender{}, time.Now())
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.handlers[CaptureKind]
	assert.True(t, ok)
}

func taskNames(tasks []TaskConfig) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Name)
	}
	return out
}
