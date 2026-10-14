package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/subscription"
)

func subscriptionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"article": {Properties: map[string]metamodel.PropertyDef{"guid": {Type: "string"}}},
	}}
}

func subscriptionsYAML(every string) []byte {
	return []byte(`subscriptions:
  news:
    url: https://example.com/feed.xml
    every: ` + every + `
    find: {type: article, match: [guid], values: {guid: "{{item.id}}"}}
    create_if_missing: {}
`)
}

const scheduleYAML = "tasks:\n  - name: t\n    script: t.lua\n    every: 1h\n"

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		schedules     string
		subscriptions []byte
		noMeta        bool
		wantTasks     []string // nil means a nil config
		wantErr       string
		subsErr       bool
	}{
		{name: "neither file", wantTasks: []string{}},
		{name: "schedules only", schedules: scheduleYAML, wantTasks: []string{"t"}},
		{
			name: "both", schedules: scheduleYAML, subscriptions: subscriptionsYAML("1h"),
			wantTasks: []string{"t", "subscription:news"},
		},
		{
			name: "invalid subscriptions keep schedules", schedules: scheduleYAML,
			subscriptions: []byte("subscriptions: {news: {url: x}}"),
			wantTasks:     []string{"t"}, wantErr: "url must be http", subsErr: true,
		},
		{
			name: "interval below minimum", subscriptions: subscriptionsYAML("5m"),
			wantTasks: []string{}, wantErr: "shorter than the 15m0s minimum", subsErr: true,
		},
		{
			name: "bad every", subscriptions: subscriptionsYAML("often"),
			wantTasks: []string{}, wantErr: "every", subsErr: true,
		},
		{name: "daily is allowed", subscriptions: subscriptionsYAML("day"), wantTasks: []string{"subscription:news"}},
		{
			name:          "name taken by schedules.yaml",
			schedules:     "tasks:\n  - name: subscription:news\n    script: t.lua\n    every: 1h\n",
			subscriptions: subscriptionsYAML("1h"),
			wantTasks:     []string{"subscription:news"}, wantErr: "is taken", subsErr: true,
		},
		{
			name: "no metamodel", subscriptions: subscriptionsYAML("1h"), noMeta: true,
			wantTasks: []string{}, wantErr: "no metamodel", subsErr: true,
		},
		{name: "invalid schedules", schedules: "tasks: [{name: x}]", wantErr: "script or template"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := newMockWorkspace(t)
			if tc.schedules != "" {
				ws.files["project:"+ConfigFile] = []byte(tc.schedules)
			}
			if tc.subscriptions != nil {
				ws.files["project:"+subscription.FileName] = tc.subscriptions
			}
			meta := subscriptionMeta()
			if tc.noMeta {
				meta = nil
			}
			cfg, err := LoadConfig(context.Background(), ws.Config(), meta)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
				require.Equal(t, tc.subsErr, errors.Is(err, ErrSubscriptions))
			}
			if tc.wantTasks == nil {
				require.Nil(t, cfg)
				return
			}
			names := []string{}
			for _, task := range cfg.Tasks {
				names = append(names, task.Name)
			}
			require.Equal(t, tc.wantTasks, names)
		})
	}
}

// TestLoadConfig_SubscriptionNotFromYAML pins that schedules.yaml cannot mark
// a task as a subscription: the field is internal.
func TestLoadConfig_SubscriptionNotFromYAML(t *testing.T) {
	t.Parallel()
	_, err := ParseConfig([]byte("tasks:\n  - name: t\n    script: t.lua\n    every: 1h\n    subscription: {}\n"))
	require.ErrorContains(t, err, "subscription")
}

// metaWorkspace adds the schema capability LoadWorkspaceConfig asserts.
type metaWorkspace struct{ *mockWorkspace }

func (metaWorkspace) Meta() *metamodel.Metamodel { return subscriptionMeta() }

func TestLoadWorkspaceConfig_UsesTheSchema(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	ws.files["project:"+subscription.FileName] = subscriptionsYAML("1h")

	_, err := LoadWorkspaceConfig(context.Background(), ws)
	require.ErrorIs(t, err, ErrSubscriptions, "without Meta the subscriptions cannot be checked")

	cfg, err := LoadWorkspaceConfig(context.Background(), metaWorkspace{ws})
	require.NoError(t, err)
	require.Len(t, cfg.Tasks, 1)
}

func subscriptionTask(t *testing.T) TaskConfig {
	t.Helper()
	cfg := &Config{}
	subs, err := subscription.Parse(subscriptionsYAML("1h"), subscriptionMeta())
	require.NoError(t, err)
	require.NoError(t, AddSubscriptions(cfg, subs))
	return cfg.Tasks[0]
}

// TestSubscriptionJob_RunsAsTheSubscription pins the job path: the task is
// queued as a subscription job keyed by its run, and the poll runs under the
// subscription's own principal with its trigger, recording the outcome.
func TestSubscriptionJob_RunsAsTheSubscription(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, subscriptionTask(t))
	var gotUser, gotTrigger, gotName string
	s.subscriptionRunner = func(ctx context.Context, name string, _ subscription.Subscription) error {
		gotUser, gotTrigger, gotName = principal.From(ctx).User, audit.TriggeredByFrom(ctx), name
		return nil
	}

	s.tick(context.Background())
	jobsQueued := q.take()
	require.Len(t, jobsQueued, 1)
	require.Equal(t, SubscriptionKind, jobsQueued[0].Kind)
	require.Equal(t, jobsQueued[0].Payload[payloadRunID], jobsQueued[0].IdempotencyKey)
	require.NoError(t, q.deliver(t, jobsQueued[0]))

	require.Equal(t, "system:subscription:news", gotUser)
	require.Equal(t, "subscription:news", gotTrigger)
	require.Equal(t, "news", gotName)
	ts, _ := taskState(t, ws, "subscription:news")
	require.Nil(t, ts.Active)
	require.True(t, ts.LastRun.Equal(t0))
}

func TestSubscriptionJob_FailureAdvancesTheLadder(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, subscriptionTask(t))
	s.subscriptionRunner = func(context.Context, string, subscription.Subscription) error {
		return errors.New("feed down")
	}
	s.tick(context.Background())
	require.Error(t, q.deliver(t, q.take()[0]))
	ts, _ := taskState(t, ws, "subscription:news")
	require.Equal(t, 1, ts.Failures)
}

// TestSubscriptionJob_RefusesMissingCapabilities pins that a workspace
// without a raw store or a host config fails the run instead of polling with
// a substitute.
func TestSubscriptionJob_RefusesMissingCapabilities(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, subscriptionTask(t))
	s.tick(context.Background())
	err := q.deliver(t, q.take()[0])
	require.ErrorContains(t, err, "no store for subscriptions")

	_, err = (&Scheduler{ws: storeWorkspace{ws}}).subscriptionDeps("news")
	require.ErrorContains(t, err, "no host config")
}

func TestSubscriptionJob_UnknownTask(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestScheduler(t, newMockWorkspace(t), t0)
	err := s.pollSubscription(context.Background(), "subscription:gone")
	require.ErrorContains(t, err, "not configured")
}

// storeWorkspace has a raw store but no host config.
type storeWorkspace struct{ *mockWorkspace }

func (storeWorkspace) Store() store.Store { return memstore.New() }

func (storeWorkspace) ScheduledLuaWriteDeps() lua.WriteDeps { return lua.WriteDeps{} }
