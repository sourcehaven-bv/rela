package dataentry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Channel route tests (TKT-ZU7P13) run a real channel service over a file
// channel store on an in-memory FS. Its anchor and ref reads go through the
// gated reader appbuildtest builds with the app's declarative ACL, as
// appbuild wires them; the adapters below mirror appbuild's.

// channelTestRealtime attaches room and threads to tickets and features,
// so a hidden feature is a hidden anchor. Nobody holds chat.read or
// chat.moderate, so nobody reads secret, posts an announce or creates a
// topic.
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
      typing: { persist: ephemeral, ttl: 5s, post: [member] }
      announce:
        properties: { body: { type: string } }
        post: [mod]
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member], from: open }
  threads:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
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
// swap; nil is realtime off.
type testRealtimeConfig struct {
	p *atomic.Pointer[realtimecfg.Compiled]
}

func (c testRealtimeConfig) Load() realtime.Snapshot { return realtime.Snapshot{Config: c.p.Load()} }

// testChannelAccess mirrors appbuild's aclAccessSource under a declarative
// policy. Entity-property roles are not used here, so the query identity
// stays empty.
type testChannelAccess struct{ d *acl.Declarative }

func (s testChannelAccess) For(ctx context.Context) (context.Context, realtime.Access, error) {
	req := acl.FromContext(ctx)
	if req == nil {
		var err error
		if req, err = s.d.ForPrincipal(principal.From(ctx)); err != nil {
			if errors.Is(err, acl.ErrUnstampedPrincipal) {
				return nil, nil, fmt.Errorf("%w: %w", realtime.ErrNoPrincipal, err)
			}
			return nil, nil, err
		}
		ctx = acl.WithRequest(ctx, req)
	}
	return ctx, testRequestAccess{req: req}, nil
}

type testRequestAccess struct{ req *acl.Request }

func (a testRequestAccess) Principal() principal.Principal { return a.req.Principal() }
func (a testRequestAccess) QueryIdentity() predicatefns.QueryIdentity {
	return predicatefns.QueryIdentity{}
}
func (a testRequestAccess) HoldsPermissionFor(ctx context.Context, anchorID, perm string) bool {
	return a.req.HoldsPermissionForEntity(ctx, anchorID, perm)
}
func (a testRequestAccess) CeilingAdmitsUpdate(entityType string) bool {
	return a.req.CeilingPermitsVerb(acl.OpUpdate, entityType)
}

// channelHeaders is the batch header read of the gated reader.
type channelHeaders interface {
	ResolveHeadersErr(ctx context.Context, refs []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error)
}

// countingHeaders counts the header batches a request makes.
type countingHeaders struct {
	channelHeaders
	calls atomic.Int64
}

func (c *countingHeaders) ResolveHeadersErr(
	ctx context.Context, refs []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	c.calls.Add(1)
	return c.channelHeaders.ResolveHeadersErr(ctx, refs)
}

// testAnchors mirrors appbuild's anchorReader.
type testAnchors struct{ rows channelHeaders }

func (r testAnchors) Anchor(ctx context.Context, addr entity.Address, props []string) (realtime.AnchorView, error) {
	ref, named := addr.Named()
	if !named {
		var err error
		if ref, err = entity.ParseRef(addr.ID()); err != nil {
			return realtime.AnchorView{}, realtime.ErrAnchorNotFound
		}
	}
	got, err := r.rows.ResolveHeadersErr(ctx, []entity.Ref{ref})
	if err != nil {
		return realtime.AnchorView{}, err
	}
	h, ok := got[ref]
	if !ok || !h.Served() {
		return realtime.AnchorView{}, realtime.ErrAnchorNotFound
	}
	view := realtime.AnchorView{ID: h.Header.ID, Face: h.Header.Face, Type: h.Header.Type}
	if len(props) > 0 {
		view.Props = map[string]any{}
		for _, p := range props {
			if v, ok := h.Header.Properties[p]; ok {
				view.Props[p] = v
			}
		}
	}
	return view, nil
}

// testRefs mirrors appbuild's refResolver.
type testRefs struct {
	rows channelHeaders
	meta *metamodel.Metamodel
}

func (r testRefs) Resolve(ctx context.Context, ids []string) (map[string]realtime.RefView, error) {
	refs := make([]entity.Ref, 0, len(ids))
	for _, id := range ids {
		if ref, err := entity.ParseRef(id); err == nil && ref.Face.IsImplicit() {
			refs = append(refs, ref)
		}
	}
	got, err := r.rows.ResolveHeadersErr(ctx, refs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]realtime.RefView, len(got))
	for ref, h := range got {
		if h.Served() {
			out[ref.ID] = realtime.RefView{
				ID: ref.ID, Type: h.Header.Type, Title: r.meta.DisplayTitle(ref.ID, h.Header.Type, h.Header.Properties),
			}
		}
	}
	return out, nil
}

// channelService is a running channel service and the handles a test
// turns.
type channelService struct {
	svc     *realtime.Service
	store   channels.Store
	cfg     *atomic.Pointer[realtimecfg.Compiled]
	headers *countingHeaders
}

// newChannelService builds a channel service over st for meta, gated by d,
// with yaml as realtime.yaml.
func newChannelService(
	t *testing.T, meta *metamodel.Metamodel, st store.Store, d *acl.Declarative, yaml string,
) *channelService {
	t.Helper()
	compiled, err := realtimecfg.Parse([]byte(yaml), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	local, err := realtime.NewLocal(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(local.Stop)
	bundle := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = bundle.Close() })
	out := &channelService{
		store:   cs,
		cfg:     &atomic.Pointer[realtimecfg.Compiled]{},
		headers: &countingHeaders{channelHeaders: bundle.GatedReads().Reader},
	}
	out.cfg.Store(compiled)
	out.svc, err = realtime.New(realtime.Deps{
		Store: cs, Config: testRealtimeConfig{p: out.cfg}, Access: testChannelAccess{d: d},
		Anchors: testAnchors{rows: out.headers}, Refs: testRefs{rows: out.headers, meta: meta}, Local: local,
	})
	if err != nil {
		t.Fatal(err)
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
		d := channelPolicy(t, app)
		setTestACL(t, app, d)
		env.ch = newChannelService(t, app.Meta(), app.store, d, channelTestRealtime)
		if err := SetChannels(app, env.ch.svc); err != nil {
			t.Fatal(err)
		}
	})
	return env
}
