package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

type forEachWorkspace struct {
	*mockWorkspace
	ids       []string
	dropped   int
	principal map[string]string
	template  string
	subject   string
}

func (w *forEachWorkspace) RunScheduledTemplate(_ context.Context, template, subject string) error {
	w.template, w.subject = template, subject
	return nil
}

func (w *forEachWorkspace) ScheduledForEachEntities(
	context.Context, string, []string, int,
) (ids []string, dropped int, err error) {
	return append([]string(nil), w.ids...), w.dropped, nil
}

func (w *forEachWorkspace) ScheduledForEachPrincipal(_ context.Context, id string) (string, error) {
	return w.principal[id], nil
}

func TestExpandJobPostsOneBoundedRetryChildPerSubject(t *testing.T) {
	t.Parallel()
	q := newFakeQueue()
	ws := &forEachWorkspace{
		mockWorkspace: newMockWorkspace(t),
		ids:           []string{"P-2", "P-1"},
		principal:     map[string]string{"P-1": "P-1", "P-2": "P-2"},
	}
	s := New(&Config{Tasks: []TaskConfig{{
		Name: "digest", Script: "digest.lua", Every: Schedule{kind: dayKind, set: true},
		ForEach: &ForEachConfig{EntityType: "person"},
	}}}, nil, ws, discardLogger())
	require.NoError(t, s.UseQueue(q))

	err := s.runExpandJob(context.Background(), jobs.Job{Payload: map[string]any{
		payloadTaskName: "digest", payloadOccurrence: "2026-08-25",
	}})
	require.NoError(t, err)
	got := q.jobs()
	require.Len(t, got, 2)
	require.Equal(t, "P-1", got[0].Payload[payloadSubject], "children are deterministic")
	require.Equal(t, jobs.RetryBounded, got[0].Retry)
	require.Equal(t, "digest/2026-08-25/P-1", got[0].IdempotencyKey)
	require.NotContains(t, got[0].Payload, payloadScript)
	require.NotContains(t, got[0].Payload, payloadRunAs)
}

func TestChildJobRunsAsSelectedPrincipal(t *testing.T) {
	t.Parallel()
	ws := &forEachWorkspace{
		mockWorkspace: newMockWorkspace(t), principal: map[string]string{"P-1": "P-1"},
	}
	s := New(&Config{Tasks: []TaskConfig{{
		Name: "digest", Script: "digest.lua", Every: Schedule{kind: dayKind, set: true},
		ForEach: &ForEachConfig{EntityType: "person"},
	}}}, nil, ws, discardLogger())
	var ran TaskConfig
	s.engineRunner = func(_ context.Context, task TaskConfig) error { ran = task; return nil }
	err := s.runChildJob(context.Background(), jobs.Job{Payload: map[string]any{
		payloadTaskName: "digest", payloadOccurrence: "2026-08-25", payloadSubject: "P-1",
	}})
	require.NoError(t, err)
	require.Equal(t, "P-1", ran.RunAs)
}

func TestChildJobRunsTemplateAtRecipientBoundary(t *testing.T) {
	t.Parallel()
	ws := &forEachWorkspace{mockWorkspace: newMockWorkspace(t), principal: map[string]string{"P-1": "P-1"}}
	s := New(&Config{Tasks: []TaskConfig{{
		Name: "digest", Template: "overdue", Every: Schedule{kind: dayKind, set: true},
		ForEach: &ForEachConfig{EntityType: "person"},
	}}}, nil, ws, discardLogger())
	err := s.runChildJob(context.Background(), jobs.Job{Payload: map[string]any{
		payloadTaskName: "digest", payloadOccurrence: "2026-08-25", payloadSubject: "P-1",
	}})
	require.NoError(t, err)
	require.Equal(t, "overdue", ws.template)
	require.Equal(t, "P-1", ws.subject)
}

func TestEnqueueForEachUsesExpansionOccurrence(t *testing.T) {
	t.Parallel()
	q := newFakeQueue()
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.Local)
	task := TaskConfig{
		Name: "digest", Script: "digest.lua", Every: Schedule{kind: dayKind, set: true},
		ForEach: &ForEachConfig{EntityType: "person"},
	}
	s := newQueuedScheduler(t, q, now, task)
	require.NoError(t, s.enqueueTask(context.Background(), task))
	got := q.jobs()
	require.Len(t, got, 1)
	require.Equal(t, ExpandKind, got[0].Kind)
	require.Equal(t, "2026-08-25", got[0].Payload[payloadOccurrence])
	require.Equal(t, "digest/2026-08-25", got[0].IdempotencyKey)
}
