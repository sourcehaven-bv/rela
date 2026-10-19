package appbuild

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// ChannelService returns the channel service of s. A package function
// because [Services] is at its plimsoll cap.
//
// Nil: s is accepted and returns nil, as does a Services built by
// [NewFromCollaborators] without [Collaborators.Channels].
func ChannelService(s *Services) *realtime.Service {
	if s == nil {
		return nil
	}
	return s.channelService
}

// PolicyChannelService returns a channel service of s that applies the
// project's acl.yaml, for a wiring that assembled s with an injected ACL.
// Stdio `rela mcp` does that (NopACL), so its entity tools stay open as
// before, while its channel tools apply acl.yaml as `rela lease` does. That
// is not a security boundary for a local user, who can edit acl.yaml; it
// keeps an agent identity inside the limits the project set for it.
//
// Without acl.yaml it returns [ChannelService]. With one it builds a
// service over the same channel store, realtime config and process-local
// state, gated by a Declarative built from the policy. A policy that does
// not load, validate or compile is an error, never an open fallback.
//
// The returned bind is the request-edge step for that service: it binds
// the principal on ctx against the service's own policy, which is not the
// one s's readers use. The caller runs it before every channel call.
//
// Nil: s is accepted and returns a nil service, as does an s without
// channels.
func PolicyChannelService(s *Services) (*realtime.Service, func(context.Context) (context.Context, error), error) {
	svc, d, err := policyChannelService(s)
	if err != nil {
		return nil, nil, err
	}
	return svc, identityBinderFor(d), nil
}

// policyChannelService is [PolicyChannelService] without the binder: it
// returns the service and the policy the service evaluates, nil for none.
func policyChannelService(s *Services) (*realtime.Service, *acl.Declarative, error) {
	if s == nil || s.channelService == nil || s.base == nil {
		return ChannelService(s), nil, nil
	}
	policy, err := loadACLPolicy(s.base.cfg)
	if err != nil {
		return nil, nil, err
	}
	if policy == nil {
		return s.channelService, s.ACLDeclarative(), nil
	}
	c, err := compileDeclarative(policy, s.base.meta, s.Store())
	if err != nil {
		return nil, nil, err
	}
	svc, _, err := newRealtimeService(realtimeInputs{
		meta: s.base.meta, worlds: s.base.worlds, config: realtimeConfigSource{h: s.base.realtime},
		store: s.Store(), channels: s.channels, local: s.realtimeLocal,
		declarative: c.declarative, redactor: c.fields.redactor, writes: c.acl,
	})
	if err != nil {
		return nil, nil, err
	}
	return svc, c.declarative, nil
}

// ACLAssignsNoRole reports whether the project has an acl.yaml whose
// assignments name no role for user. A local process uses it to warn that
// its identity reaches the channels with the "everyone" role only. It looks
// at assignments alone: a role a user entity or a membership grants is not
// seen, so the answer is a hint for a log line, not an access decision. An
// acl.yaml that does not load reports false; [PolicyChannelService] reports
// that error.
//
// Nil: s is accepted and reports false.
func ACLAssignsNoRole(s *Services, user string) bool {
	if s == nil || s.base == nil {
		return false
	}
	policy, err := loadACLPolicy(s.base.cfg)
	if err != nil || policy == nil {
		return false
	}
	_, ok := policy.Assignments[user]
	return !ok
}

// buildRealtimeService builds the channel service over cs for one assembly.
// The service is per assembly, so its access checks follow the current ACL;
// local is the process-local state successive assemblies share (see
// [SharedBase.ForReassembly]), or nil for a first assembly, which makes it.
func buildRealtimeService(
	base *SharedBase, st store.Store, cs channels.Store, local *realtime.Local,
	d *acl.Declarative, redactor visibility.FieldRedactor, writes acl.ACL,
) (*realtime.Service, *realtime.Local, error) {
	return newRealtimeService(realtimeInputs{
		meta: base.meta, worlds: base.worlds, config: realtimeConfigSource{h: base.realtime},
		store: st, channels: cs, local: local, declarative: d, redactor: redactor, writes: writes,
	})
}

// realtimeInputs are what [newRealtimeService] builds a channel service
// from. Assembly and [NewFromCollaborators] both fill it.
type realtimeInputs struct {
	meta        *metamodel.Metamodel
	worlds      worlds.Compiled
	config      realtime.ConfigSource
	store       store.Store
	channels    channels.Store
	local       *realtime.Local
	declarative *acl.Declarative
	redactor    visibility.FieldRedactor
	writes      acl.ACL
}

// newRealtimeService builds the channel service from in, and makes the
// process-local state when in.local is nil.
//
// Reads of anchors and refs go through the gated, redacted reader
// [scriptReads] builds from the same inputs as the entity read paths, so a
// channel route answers not-found exactly where an entity GET does.
func newRealtimeService(in realtimeInputs) (*realtime.Service, *realtime.Local, error) {
	local := in.local
	if local == nil {
		var err error
		if local, err = realtime.NewLocal(time.Now); err != nil {
			return nil, nil, err
		}
	}
	rows, _ := scriptReads(in.store, in.declarative, in.redactor, in.worlds)
	svc, err := realtime.New(realtime.Deps{
		Store:   in.channels,
		Config:  in.config,
		Access:  aclAccessSource{d: in.declarative, writes: in.writes},
		Anchors: anchorReader{rows: rows, meta: in.meta},
		Refs:    refResolver{rows: rows, meta: in.meta},
		Local:   local,
	})
	if err != nil {
		return nil, nil, err
	}
	return svc, local, nil
}

// headerReader is the batch header read of the gated reader.
type headerReader interface {
	ResolveHeadersErr(ctx context.Context, refs []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error)
}

// realtimeConfigSource adapts the holder to [realtime.ConfigSource].
type realtimeConfigSource struct{ h *RealtimeConfig }

func (s realtimeConfigSource) Load() realtime.Snapshot {
	snap := s.h.Load()
	return realtime.Snapshot{Config: snap.Config, Degraded: snap.Degraded}
}

// aclAccessSource opens one [realtime.Access] per operation. Under a
// declarative policy it reads the acl.Request the edge bound on ctx and
// fails closed without one: no Request is [realtime.ErrNoPrincipal], and a
// foreign or mismatched one is an error. It never resolves the principal
// itself, so the gated reads of that operation evaluate the edge's request.
// Without a policy (NopACL, ReadOnlyACL) every permission is held, and the
// write ACL decides whether anything may be written.
type aclAccessSource struct {
	d      *acl.Declarative
	writes acl.ACL
}

func (s aclAccessSource) For(ctx context.Context) (context.Context, realtime.Access, error) {
	q, _ := queryIdentityFor(ctx)
	if s.d == nil {
		writable := !access.ModeOf(s.writes).RefusesWrites()
		return ctx, openAccess{p: principal.From(ctx), q: q, writable: writable}, nil
	}
	req, err := s.d.RequestFrom(ctx)
	if errors.Is(err, acl.ErrNoIdentity) {
		return nil, nil, fmt.Errorf("%w: %w", realtime.ErrNoPrincipal, err)
	}
	if err != nil {
		return nil, nil, err
	}
	return ctx, requestAccess{req: req, q: q}, nil
}

// requestAccess answers from one acl.Request.
type requestAccess struct {
	req *acl.Request
	q   predicatefns.QueryIdentity
}

func (a requestAccess) Principal() principal.Principal            { return a.req.Principal() }
func (a requestAccess) QueryIdentity() predicatefns.QueryIdentity { return a.q }

func (a requestAccess) HoldsPermissionFor(ctx context.Context, anchorID, perm string) bool {
	return a.req.HoldsPermissionForEntity(ctx, anchorID, perm)
}

func (a requestAccess) CeilingAdmitsUpdate(entityType string) bool {
	return acl.CeilingPermitsVerb(a.req, acl.OpUpdate, entityType)
}

// openAccess answers without a policy: every permission is held and there
// is no client ceiling. writable is false under ReadOnlyACL, which refuses
// every write.
type openAccess struct {
	p        principal.Principal
	q        predicatefns.QueryIdentity
	writable bool
}

func (a openAccess) Principal() principal.Principal                          { return a.p }
func (a openAccess) QueryIdentity() predicatefns.QueryIdentity               { return a.q }
func (a openAccess) HoldsPermissionFor(context.Context, string, string) bool { return true }
func (a openAccess) CeilingAdmitsUpdate(string) bool                         { return a.writable }

// anchorReader reads a channel anchor through one batch header read: the
// row gate, the readable faces and field redaction all apply, and a bare id
// serves the face the world ranks first among the readable ones.
//
// It also binds the record a lifecycle condition reads, from the same
// redacted properties, so a hidden property reads as absent there too. The
// record is bound even when no property is requested: a condition may read
// only the id, the type or the clock.
type anchorReader struct {
	rows headerReader
	meta *metamodel.Metamodel
}

func (r anchorReader) Anchor(ctx context.Context, addr entity.Address, props []string) (realtime.AnchorView, error) {
	ref, named := addr.Named()
	if !named {
		// A bare id parses to the implicit face, the form ResolveHeaders
		// reads as the whole family.
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
		view.Props = make(map[string]any, len(props))
		for _, p := range props {
			if v, ok := h.Header.Properties[p]; ok {
				view.Props[p] = v
			}
		}
	}
	if def, ok := r.meta.GetEntityDef(view.Type); ok {
		view.Record = predicatefns.EntityRecord(r.meta, def, view.ID, view.Type, view.Props)
	}
	return view, nil
}

// refResolver resolves ref properties through one batch header read. The
// headers are already redacted, so a title built from a hidden property
// falls back to the id.
type refResolver struct {
	rows headerReader
	meta *metamodel.Metamodel
}

func (r refResolver) Resolve(ctx context.Context, ids []string) (map[string]realtime.RefView, error) {
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
		if !h.Served() {
			continue
		}
		out[ref.ID] = realtime.RefView{
			ID: ref.ID, Type: h.Header.Type,
			Title: r.meta.DisplayTitle(ref.ID, h.Header.Type, h.Header.Properties),
		}
	}
	return out, nil
}

// typeRecordReader is the part of a channel store a reload checks.
type typeRecordReader interface {
	TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error)
}

// recordSources are the channel stores of the live assemblies of one
// holder. Reload holds a new config against the type records of each.
type recordSources struct {
	mu   sync.Mutex
	next int
	srcs map[int]typeRecordReader
}

// watchTypeRecords makes Reload check src's type records until the
// returned stop is called.
//
// Nil: r and src are accepted; either makes it a no-op.
func (r *RealtimeConfig) watchTypeRecords(src typeRecordReader) func() {
	if r == nil || src == nil {
		return func() {}
	}
	r.records.mu.Lock()
	defer r.records.mu.Unlock()
	if r.records.srcs == nil {
		r.records.srcs = map[int]typeRecordReader{}
	}
	id := r.records.next
	r.records.next++
	r.records.srcs[id] = src
	return func() {
		r.records.mu.Lock()
		defer r.records.mu.Unlock()
		delete(r.records.srcs, id)
	}
}

// checkTypeRecords refuses next when it changes the anchor or cardinality
// of a channel type a watched store has recorded, or declares one of its
// recorded refs as another property type.
func (r *RealtimeConfig) checkTypeRecords(ctx context.Context, next *realtimecfg.Compiled) error {
	if next == nil {
		return nil
	}
	r.records.mu.Lock()
	srcs := make([]typeRecordReader, 0, len(r.records.srcs))
	for _, s := range r.records.srcs {
		srcs = append(srcs, s)
	}
	r.records.mu.Unlock()
	for _, src := range srcs {
		if err := checkRecords(ctx, src, next); err != nil {
			return err
		}
	}
	return nil
}

// checkRecords holds cfg against the type records of src: a changed anchor
// or cardinality of a recorded type, or a recorded ref cfg declares as
// another property type, is a [realtimecfg.ShapeMismatch]. Boot and reload
// both report through it.
func checkRecords(ctx context.Context, src typeRecordReader, cfg *realtimecfg.Compiled) error {
	recs, err := src.TypeRecords(ctx)
	if err != nil {
		return fmt.Errorf("channels: read the recorded channel types: %w", err)
	}
	stored := make(map[string]realtimecfg.TypeShape, len(recs))
	for typ, rec := range recs {
		stored[typ] = realtimecfg.TypeShape{
			Anchor:      realtimecfg.Anchor(rec.Anchor),
			Cardinality: realtimecfg.Cardinality(rec.Cardinality),
			Refs:        rec.Refs,
		}
	}
	return realtimecfg.ShapeMismatchError(realtimecfg.ShapeMismatches(cfg, stored))
}
