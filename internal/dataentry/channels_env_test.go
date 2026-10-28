package dataentry

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Channel route tests (TKT-ZU7P13) run a real channel service over a file
// channel store on an in-memory FS. appbuildtest builds it with the
// production constructor, so its anchor and ref reads go through the gated
// reader the app's declarative ACL builds, as in a deployment.

// channelTestRealtime attaches room and threads to tickets and features,
// so a hidden feature is a hidden anchor. Nobody holds chat.read or
// chat.moderate, so nobody reads secret, posts an announce or creates a
// topic. A room text offers ack, and accept, which promotes; the
// collaborator wiring has no promoter, so accept is never offered. chat
// and briefing offer the built-in typing indicator; nobody may post to
// briefing, so nobody may type there.
const channelTestRealtime = `
version: 1
channel_types:
  room:
    label: Room
    attach_to: [ticket, feature]
    read: { from: attached }
    roles:
      member: { from: attached }
      mod:    { permission: chat.moderate }
    message_types:
      text:
        properties:
          body:  { type: string, required: true }
          about: { type: ref, to: [ticket, feature] }
        post: [member]
        actions:
          ack: { roles: [member] }
          accept: { roles: [member], promote_to: { type: ticket, map: { title: body } } }
      typing: { persist: ephemeral, ttl: 5s, post: [member] }
      announce:
        properties: { body: { type: string } }
        post: [mod]
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member], from: open }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [mod] }
      delete: { roles: [member], retention: 7d }
  threads:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    lifecycle:
      delete: { roles: [member] }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  topics:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [mod] }
    roles:
      member: { from: attached }
      mod:    { permission: chat.moderate }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  incident:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      contained: { properties: { note: { type: string } }, post: [member] }
      note:      { properties: { body: { type: string } }, post: [member] }
    lifecycle:
      close: { roles: [member] }
    session:
      initial: detected
      states:
        detected:
          label: Detected
          on: { contained: { to: contained } }
          after: { no-containment: { period: 24h, to: contained, label: Not contained within a day } }
        contained: { label: Contained, final: true }
  chat:
    attach_to: [ticket, feature]
    typing: true
    read: { from: attached }
    roles:
      member: { from: attached }
      mod:    { permission: chat.moderate }
    message_types:
      text:   { properties: { body: { type: string, required: true } }, post: [member] }
      status: { persist: ephemeral, ttl: 30s, post: [member] }
      notice: { properties: { body: { type: string } }, post: [mod] }
    lifecycle:
      close: { roles: [member] }
  briefing:
    attach_to: [ticket]
    typing: true
    read: { from: attached }
    roles:
      mod: { permission: chat.moderate }
    message_types:
      note: { properties: { body: { type: string } }, post: [mod] }
  secret:
    attach_to: [ticket]
    read: { permission: chat.read }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

// testRealtimeConfig is a realtime.ConfigSource over a pointer a test can
// swap; nil is realtime off. degraded, when set, marks the snapshot
// degraded while it holds true.
type testRealtimeConfig struct {
	p        *atomic.Pointer[realtimecfg.Compiled]
	degraded *atomic.Bool
}

func (c testRealtimeConfig) Load() realtime.Snapshot {
	return realtime.Snapshot{Config: c.p.Load(), Degraded: c.degraded != nil && c.degraded.Load()}
}

// channelService is a running channel service and the handles a test
// turns.
type channelService struct {
	svc   *realtime.Service
	store channels.Store
	cfg   *atomic.Pointer[realtimecfg.Compiled]
	meta  *metamodel.Metamodel
	// degraded marks the configuration degraded while it holds true.
	degraded atomic.Bool
	// outputs writes session outputs; no test configuration declares any
	// unless it swaps in outputsRealtime.
	outputs *fakeOutputs
}

// fakeOutputs is an OutputWriter that allows every output and writes
// nothing. A create answers with FEAT-1, which only dave reads. A set on
// TKT-1 fails as invalid on status.
type fakeOutputs struct{}

func (fakeOutputs) Poster(context.Context) (realtime.OutputPoster, error) {
	return realtime.OutputPoster{}, nil
}

func (fakeOutputs) Check(context.Context, realtime.OutputTarget) error { return nil }

func (fakeOutputs) Write(ctx context.Context, r realtime.OutputRequest) (realtime.OutputWritten, error) {
	if r.Target.Kind == realtimecfg.OutputSet && r.Target.AnchorID == "TKT-1" {
		return realtime.OutputWritten{}, &realtime.OutputError{
			Code: realtime.OutputCodeInvalid, Props: []string{"status"}, WroteNothing: true,
		}
	}
	if r.Target.Kind != realtimecfg.OutputCreate {
		return realtime.OutputWritten{}, nil
	}
	if r.Started != nil {
		if err := r.Started(ctx); err != nil {
			return realtime.OutputWritten{}, err
		}
	}
	return realtime.OutputWritten{Entity: &channels.Promoted{ID: "FEAT-1", Type: "feature"}}, nil
}

// outputsRealtime is channelTestRealtime with outputs on the incident
// session: contained sets the status and creates a feature, and
// contained is no longer final, as a note then moves it to reviewed.
var outputsRealtime = strings.Replace(channelTestRealtime,
	`          on: { contained: { to: contained } }
          after: { no-containment: { period: 24h, to: contained, label: Not contained within a day } }
        contained: { label: Contained, final: true }`,
	`          on:
            contained:
              to: contained
              outputs:
                - set: { properties: { status: contained } }
                - create: { type: feature, properties: { title: Follow-up } }
        contained:
          label: Contained
          on:
            note:
              to: reviewed
              outputs:
                - set: { properties: { status: reviewed } }
        reviewed: { label: Reviewed, final: true }`, 1)

// parse compiles yaml as realtime.yaml against the service's schema.
func (c *channelService) parse(tb testing.TB, yaml string) *realtimecfg.Compiled {
	tb.Helper()
	compiled, err := realtimecfg.Parse([]byte(yaml), c.meta, nil)
	if err != nil {
		tb.Fatal(err)
	}
	return compiled
}

// newChannelService builds a channel service over st for meta, gated by d,
// with yaml as realtime.yaml, over a fresh file channel store.
func newChannelService(
	t *testing.T, meta *metamodel.Metamodel, st store.Store, d *acl.Declarative, yaml string,
) *channelService {
	t.Helper()
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	return newChannelServiceOver(t, meta, st, d, yaml, cs)
}

// newChannelServiceOver is newChannelService over the channel store cs.
func newChannelServiceOver(
	t *testing.T, meta *metamodel.Metamodel, st store.Store, d *acl.Declarative, yaml string, cs channels.Store,
) *channelService {
	t.Helper()
	return newChannelServiceLocal(t, meta, st, d, yaml, cs, nil)
}

// newChannelServiceLocal is newChannelServiceOver over the process-local
// state local. Nil: a new one on the real clock.
func newChannelServiceLocal(
	tb testing.TB, meta *metamodel.Metamodel, st store.Store, d *acl.Declarative, yaml string, cs channels.Store,
	local *realtime.Local,
) *channelService {
	tb.Helper()
	out := &channelService{store: cs, cfg: &atomic.Pointer[realtimecfg.Compiled]{}, meta: meta, outputs: &fakeOutputs{}}
	out.cfg.Store(out.parse(tb, yaml))
	bundle := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d),
		appbuildtest.WithChannelsLocal(cs, testRealtimeConfig{p: out.cfg, degraded: &out.degraded}, local),
		appbuildtest.WithChannelOutputs(out.outputs))
	tb.Cleanup(func() { _ = bundle.Close() })
	if out.svc = appbuild.ChannelService(bundle); out.svc == nil {
		tb.Fatal("appbuildtest built no channel service")
	}
	return out
}

// channelPolicy: alice and bob are editors through their membership of the
// editors group (read and update tickets); carol reads tickets; dave reads
// and updates tickets and features. Only dave reads features.
func channelPolicy(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	return mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}},
			"viewer": {Read: []string{"ticket"}},
			"admin":  {Read: []string{"ticket", "feature"}, Update: []string{"ticket", "feature"}},
		},
		Assignments: map[string]string{"editors": "editor", "carol": "viewer", "dave": "admin"},
	}, app.store)
}

// channelEnv is a running app with channels wired.
type channelEnv struct {
	*liveEnv
	ch *channelService
}

// newChannelEnv seeds TKT-1, TKT-2 and FEAT-1, makes alice and bob members
// of editors, applies channelPolicy and wires a channel service.
func newChannelEnv(t *testing.T) *channelEnv {
	t.Helper()
	return newChannelEnvWith(t, channelPolicy, nil)
}

// newChannelEnvWith is newChannelEnv with policy as the ACL. wired, when
// set, runs after the channel service is installed and before the live
// worker starts.
func newChannelEnvWith(
	t *testing.T, policy func(*testing.T, *App) *acl.Declarative, wired func(*App),
) *channelEnv {
	t.Helper()
	return newChannelEnvLocal(t, policy, wired, nil)
}

// newChannelEnvLocal is newChannelEnvWith over the process-local channel
// state local, such as one on a test clock. Nil: a new one on the real
// clock.
func newChannelEnvLocal(
	t *testing.T, policy func(*testing.T, *App) *acl.Declarative, wired func(*App), local *realtime.Local,
) *channelEnv {
	t.Helper()
	env := &channelEnv{}
	env.liveEnv = newLiveEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "two"}})
		seedEntity(app, feature("FEAT-1", "hidden"))
		for _, id := range []string{"alice", "bob", "editors"} {
			seedEntity(app, &entity.Entity{ID: id, Type: "person", Properties: map[string]any{"title": id}})
		}
		seedRelation(app, entity.NewRelation("alice", "member-of", "editors"))
		seedRelation(app, entity.NewRelation("bob", "member-of", "editors"))
		d := policy(t, app)
		setTestACL(t, app, d)
		cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
		if err != nil {
			t.Fatal(err)
		}
		env.ch = newChannelServiceLocal(t, app.Meta(), app.store, d, channelTestRealtime, cs, local)
		if err := SetChannels(app, env.ch.svc); err != nil {
			t.Fatal(err)
		}
		if wired != nil {
			wired(app)
		}
	})
	return env
}
