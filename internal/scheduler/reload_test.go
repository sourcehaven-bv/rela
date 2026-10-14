package scheduler

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/subscription"
)

// queuedTasks ticks s and returns the names of the tasks it queued.
func queuedTasks(t *testing.T, s *Scheduler, q *fakeQueue) []string {
	t.Helper()
	s.tick(context.Background())
	var names []string
	for _, job := range q.take() {
		name, _ := job.Payload[payloadTaskName].(string)
		names = append(names, name)
	}
	return names
}

// TestWatchConfig_AppliesEdits pins the reload loop: a task added to either
// file runs on the next tick, a removed task is not queued again, and an edit
// that fails to load keeps the running tasks and is logged once.
func TestWatchConfig_AppliesEdits(t *testing.T) {
	ws := metaWorkspace{newMockWorkspace(t)}
	s, q, c := newTestScheduler(t, ws, t0)
	var logs bytes.Buffer
	s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	s.WatchConfig()

	require.Empty(t, queuedTasks(t, s, q), "no files, no tasks")

	ws.files["project:"+ConfigFile] = []byte(scheduleYAML)
	ws.files["project:"+subscription.FileName] = subscriptionsYAML("1h")
	require.ElementsMatch(t, []string{"t", "subscription:news"}, queuedTasks(t, s, q))
	require.Contains(t, logs.String(), "scheduler config reloaded")

	ws.files["project:"+subscription.FileName] = []byte("subscriptions: {news: {url: x}}")
	c.now = c.now.Add(2 * time.Hour)
	for range 2 {
		s.tick(context.Background())
	}
	require.Equal(t, 1, strings.Count(logs.String(), "scheduler config not fully reloaded"),
		"a broken file is reported once, not every tick")
	require.Len(t, s.config.Load().Tasks, 2, "a broken file keeps the running tasks")
	q.take()

	delete(ws.files, "project:"+subscription.FileName)
	c.now = c.now.Add(2 * time.Hour)
	queued := queuedTasks(t, s, q)
	require.Contains(t, logs.String(), "scheduler config loads again")
	require.NotContains(t, queued, "subscription:news", "a removed subscription is not queued again")
	require.Len(t, s.config.Load().Tasks, 1)
}

// TestWatchConfig_UnchangedKeepsTheConfig pins that a reload of the same
// files does not replace the published config.
func TestWatchConfig_UnchangedKeepsTheConfig(t *testing.T) {
	ws := metaWorkspace{newMockWorkspace(t)}
	ws.files["project:"+ConfigFile] = []byte(scheduleYAML)
	cfg, err := LoadWorkspaceConfig(context.Background(), ws)
	require.NoError(t, err)
	s, _, _ := newTestScheduler(t, ws, t0, cfg.Tasks...)
	s.WatchConfig()
	before := s.config.Load()
	s.tick(context.Background())
	require.Same(t, before, s.config.Load())
}

func TestTaskDiff(t *testing.T) {
	added, removed, changed := taskDiff(
		[]TaskConfig{{Name: "a"}, {Name: "b"}, {Name: "d"}},
		[]TaskConfig{{Name: "b", Script: "changed.lua"}, {Name: "c"}, {Name: "d"}},
	)
	require.Equal(t, []string{"c"}, added)
	require.Equal(t, []string{"a"}, removed)
	require.Equal(t, []string{"b"}, changed)
}

// TestWatchConfig_BrokenSubscriptionsKeepSchedulesLive pins that a broken
// subscriptions.yaml does not freeze schedules.yaml: a schedules edit still
// applies, and the running subscriptions stay.
func TestWatchConfig_BrokenSubscriptionsKeepSchedulesLive(t *testing.T) {
	ws := metaWorkspace{newMockWorkspace(t)}
	ws.files["project:"+ConfigFile] = []byte(scheduleYAML)
	ws.files["project:"+subscription.FileName] = subscriptionsYAML("1h")
	cfg, err := LoadWorkspaceConfig(context.Background(), ws)
	require.NoError(t, err)
	s, _, _ := newTestScheduler(t, ws, t0, cfg.Tasks...)
	s.WatchConfig()

	ws.files["project:"+subscription.FileName] = []byte("subscriptions: {news: {url: x}}")
	ws.files["project:"+ConfigFile] = []byte(strings.Replace(scheduleYAML, "name: t", "name: renamed", 1))
	s.reloadConfig(context.Background())

	names := []string{}
	for _, task := range s.config.Load().Tasks {
		names = append(names, task.Name)
	}
	require.ElementsMatch(t, []string{"renamed", "subscription:news"}, names)
}

// TestMaybePrune_SkipsAnIncompleteConfig pins that run-state, which is shared
// by every config, is not pruned with no tasks or while a file fails to load.
func TestMaybePrune_SkipsAnIncompleteConfig(t *testing.T) {
	s, _, _ := newTestScheduler(t, newMockWorkspace(t), t0)
	tasks := &Config{Tasks: []TaskConfig{{Name: "t"}}}

	s.maybePrune(context.Background(), &Config{}, t0)
	require.True(t, s.lastPrune.IsZero(), "no tasks, no prune")

	s.reloadErr = "broken"
	s.maybePrune(context.Background(), tasks, t0)
	require.True(t, s.lastPrune.IsZero(), "a failing file, no prune")

	s.reloadErr = ""
	s.maybePrune(context.Background(), tasks, t0)
	require.Equal(t, t0, s.lastPrune)
	require.Equal(t, pruneAgeFor(tasks.Tasks), s.pruneAge)
}

// TestStartBackground_RunsWithoutTasks pins that a server started with no
// tasks keeps running, so a file created later can take effect. That the
// reload applies it is pinned by TestWatchConfig_AppliesEdits.
func TestStartBackground_RunsWithoutTasks(t *testing.T) {
	ws := &queueWorkspace{mockWorkspace: newMockWorkspace(t), q: newFakeQueue()}
	ctx, cancel := context.WithCancel(context.Background())
	done := StartBackground(ctx, ws, discardLogger())
	select {
	case <-done:
		t.Fatal("the scheduler stopped although a file could still be added")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	<-done
}
