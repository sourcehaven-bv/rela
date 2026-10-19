// Package realtime is the channel service: it authorizes, validates, rate
// limits and renders channel operations over a [channels.Store], and holds
// the ephemeral signals that are never stored.
//
// Every gate comes in through a consumer-side interface the wiring site
// implements ([AccessSource], [AnchorReader], [RefResolver],
// [ConfigSource]), so the package depends on neither the ACL nor the entity
// store. Each operation captures its state once: one config snapshot, one
// [Access] over one ACL request, one clock reading.
//
// The ephemeral signals, rate buckets and live change sets are process
// local (see [Local]). A multi-process deployment limits each process on its
// own, and a signal posted to one process is not seen by another.
package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// ErrRealtimeOff reports that the project has no realtime.yaml. The HTTP
// layer answers every channel route with its uniform not-found.
var ErrRealtimeOff = errors.New("realtime: off")

func errNil(name string) error { return fmt.Errorf("realtime: %s is required", name) }

// Deps are the collaborators of a [Service].
type Deps struct {
	Store   channels.Store
	Config  ConfigSource
	Access  AccessSource
	Anchors AnchorReader
	Refs    RefResolver
	// Local is the process-local state, shared by the services built over
	// one store.
	Local *Local
}

// Service is the channel service.
type Service struct {
	store   channels.Store
	cfg     ConfigSource
	access  AccessSource
	anchors AnchorReader
	refs    RefResolver
	local   *Local
	clock   Clock
}

// New returns the service over d.
//
// Nil: every field of d is rejected.
func New(d Deps) (*Service, error) {
	switch {
	case d.Store == nil:
		return nil, errNil("Store")
	case d.Config == nil:
		return nil, errNil("Config")
	case d.Access == nil:
		return nil, errNil("Access")
	case d.Anchors == nil:
		return nil, errNil("Anchors")
	case d.Refs == nil:
		return nil, errNil("Refs")
	case d.Local == nil:
		return nil, errNil("Local")
	}
	return &Service{
		store: d.Store, cfg: d.Config, access: d.Access, anchors: d.Anchors, refs: d.Refs,
		local: d.Local, clock: d.Local.clock,
	}, nil
}

// Subscribe registers a live consumer of channel changes.
func (s *Service) Subscribe() *Subscription { return s.local.Subscribe() }

// Config returns the realtime configuration in force now, or nil when
// realtime is off. It is for describing the API (the OpenAPI document);
// an operation loads its own snapshot.
func (s *Service) Config() *realtimecfg.Compiled { return s.cfg.Load().Config }

// Enabled reports whether realtime is on now. A route uses it to answer
// "off" before it reads the request; an operation still loads its own
// snapshot, which decides.
func (s *Service) Enabled() bool { return s.Config() != nil }

// Affordances are what the caller may do in a channel now.
type Affordances struct {
	// Post lists the message types the caller may post, sorted. A type's
	// `when:` is not evaluated here; it depends on the message.
	Post []string `json:"post"`
	// EditOwn and DeleteOwn report whether the caller passes the gate of
	// the operation. Whether a given message qualifies also depends on its
	// author, the edit window and the channel state.
	EditOwn   bool `json:"edit_own"`
	DeleteOwn bool `json:"delete_own"`
	// Remove reports whether the caller passes the remove gate. Such a
	// reader also sees who removed a message and why.
	Remove bool `json:"remove"`
	// Lifecycle lists the lifecycle operations the caller may run on the
	// channel now, in the order close, reopen, archive, delete. Empty for a
	// cardinality-one channel that holds no message yet.
	Lifecycle []string `json:"lifecycle"`
}

// HeadView is a channel's head: its metadata, the last message, the live
// signals and the caller's affordances.
type HeadView struct {
	Channel   ChannelView     `json:"channel"`
	Last      *MessageView    `json:"last,omitempty"`
	Ephemeral []EphemeralView `json:"ephemeral"`
	Actions   Affordances     `json:"actions"`
}

// Head returns the channel's head.
func (s *Service) Head(ctx context.Context, addr RawAddress) (HeadView, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return HeadView{}, err
	}
	var msgs []channels.Message
	if c.head != nil && c.head.LastSeq > 0 {
		m, gerr := s.store.Get(c.ctx, c.key, c.head.LastSeq)
		if gerr != nil && !errors.Is(gerr, channels.ErrNotFound) {
			return HeadView{}, fmt.Errorf("realtime: read last message: %w", gerr)
		}
		if gerr == nil {
			msgs = append(msgs, m)
		}
	}
	sigs := s.local.liveSignals(c.key)
	r, err := s.renderer(c, msgs, sigs)
	if err != nil {
		return HeadView{}, err
	}
	out := HeadView{Channel: channelView(c.key, c.head), Ephemeral: []EphemeralView{}, Actions: c.affordances()}
	if len(msgs) > 0 {
		v := r.message(msgs[0])
		out.Last = &v
	}
	for _, sg := range sigs {
		out.Ephemeral = append(out.Ephemeral, r.signal(sg))
	}
	return out, nil
}

// affordances answers what the caller may do in c's channel.
func (c *call) affordances() Affordances {
	a := Affordances{Post: []string{}, Lifecycle: []string{}}
	if c.requireCaller() != nil {
		return a
	}
	ops := c.ct.MessageOps
	a.Remove = c.moderator()
	a.Lifecycle = c.lifecycleOps()
	if c.state() == channels.StateOpen {
		for _, name := range sortedKeys(c.ct.MessageTypes) {
			if c.mayPost(c.ct.MessageTypes[name]) {
				a.Post = append(a.Post, name)
			}
		}
		a.EditOwn = ops.EditOwn != nil && c.gate(ops.EditOwn.Gate)
	}
	a.DeleteOwn = ops.DeleteOwn != nil && c.gate(ops.DeleteOwn.Gate)
	return a
}

// renderer builds the renderer for c, resolving every ref of msgs and sigs
// in one batch. A property the store recorded as a ref renders as one
// whatever the configuration now says (see Local.storedRefs).
func (s *Service) renderer(c *call, msgs []channels.Message, sigs []signal) (*renderer, error) {
	r := &renderer{ct: c.ct, moderator: c.moderator()}
	if len(msgs) == 0 && len(sigs) == 0 {
		return r, nil
	}
	stored, err := s.local.storedRefs(c.ctx, s.store, c.snap.Config, c.ct.Name)
	if err != nil {
		return nil, err
	}
	r.stored = stored
	ids := r.refIDs(msgs, sigs)
	if len(ids) == 0 {
		return r, nil
	}
	got, err := s.refs.Resolve(c.ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("realtime: resolve refs: %w", err)
	}
	r.refs = got
	return r, nil
}

// MessagePage is one page of messages. Full is set when the page holds as
// many messages as asked for: there may be more past its cursor.
type MessagePage struct {
	Channel  ChannelView   `json:"channel"`
	Messages []MessageView `json:"messages"`
	Full     bool          `json:"full"`
}

// Messages returns one page of messages. The limit is clamped to the
// configured page limit.
func (s *Service) Messages(ctx context.Context, addr RawAddress, page channels.Page) (MessagePage, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return MessagePage{}, err
	}
	limit := c.snap.Config.Limits().PageLimit
	if page.Limit <= 0 || page.Limit > limit {
		page.Limit = limit
	}
	page, err = page.Normalize()
	if err != nil {
		return MessagePage{}, storeError(err)
	}
	out := MessagePage{Channel: channelView(c.key, c.head), Messages: []MessageView{}}
	if c.head == nil {
		return out, nil
	}
	msgs, err := s.store.Messages(c.ctx, c.key, page)
	if err != nil {
		if errors.Is(err, channels.ErrNotFound) {
			return out, nil
		}
		return MessagePage{}, storeError(err)
	}
	r, err := s.renderer(c, msgs, nil)
	if err != nil {
		return MessagePage{}, err
	}
	for _, m := range msgs {
		out.Messages = append(out.Messages, r.message(m))
	}
	out.Full = len(msgs) >= page.Limit
	return out, nil
}

// PostInput is a message to post.
type PostInput struct {
	Type    string
	Props   map[string]any
	ReplyTo int64
}

// PostResult is a posted message. A log message has Message set. An
// ephemeral signal has Ephemeral, ID and ExpiresAt set, and no seq: it is
// never stored.
type PostResult struct {
	Ephemeral bool
	ID        string
	ExpiresAt time.Time
	Message   *MessageView
}

// Post posts a message. The checks run in this order: read access, a post
// role of the message type, an open channel, the rate limit, the
// properties, the type's `when:`, the refs. A denied caller is not charged
// a token; an invalid message is.
func (s *Service) Post(ctx context.Context, addr RawAddress, in PostInput) (PostResult, error) {
	c, openErr := s.open(ctx, addr)
	if openErr != nil {
		return PostResult{}, openErr
	}
	if err := c.requireCaller(); err != nil {
		return PostResult{}, err
	}
	mt := c.ct.MessageTypes[in.Type]
	if mt == nil {
		return PostResult{}, invalid("channel type %s has no message type %q", c.ct.Name, in.Type)
	}
	if !c.mayPost(mt) {
		return PostResult{}, postDenied(c.ct, mt)
	}
	if err := c.requireOpen(); err != nil {
		return PostResult{}, err
	}
	ephemeral := mt.Persist == realtimecfg.PersistEphemeral
	if err := s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opPostPfx+mt.Name,
		postRate(c.snap.Config.Limits(), mt)); err != nil {
		return PostResult{}, err
	}
	maxBytes := c.snap.Config.Limits().MessageBytes
	if ephemeral {
		maxBytes = min(maxBytes, realtimelimits.MaxEphemeralBytes)
	}
	vp, refs, err := s.checkMessage(c, mt, in.Props, maxBytes)
	if err != nil {
		return PostResult{}, err
	}
	if ephemeral {
		return s.postSignal(c, mt, vp, in.ReplyTo)
	}
	err = s.record(c)
	if err != nil {
		return PostResult{}, err
	}
	m, err := s.store.Append(c.ctx, c.key, channels.NewMessage{
		Type: mt.Name, Author: authorOf(c.p), At: c.now, Props: vp.props, ReplyTo: in.ReplyTo,
	})
	if err != nil {
		return PostResult{}, storeError(err)
	}
	s.local.mark(c.key)
	v := s.rendererWith(c, refs).message(m)
	return PostResult{Message: &v}, nil
}

// rendererWith is a renderer over refs already resolved for the caller.
// It needs no stored refs: it renders only the caller's own write, after a successful
// Service.record, which refuses a config that declares a recorded ref as
// another type, or after a removal, which keeps no properties.
func (s *Service) rendererWith(c *call, refs map[string]RefView) *renderer {
	return &renderer{ct: c.ct, refs: refs, moderator: c.moderator()}
}

// postRate is the rate of mt: its own, or the configured default for its
// persistence.
func postRate(l realtimecfg.Limits, mt *realtimecfg.MessageType) realtimelimits.Rate {
	if mt.Rate.Count > 0 && mt.Rate.Per > 0 {
		return mt.Rate
	}
	if mt.Persist == realtimecfg.PersistEphemeral {
		return l.EphemeralRate
	}
	return l.LogRate
}

func (c *call) requireOpen() error {
	if st := c.state(); st != channels.StateOpen {
		return deny(http.StatusConflict, TypeChannelNotOpen, fmt.Sprintf("the channel is %s", st))
	}
	return nil
}

// checkMessage validates props against mt, evaluates its `when:` and
// resolves its refs in one batch. Every ref must name an entity the caller
// may read, of a type the property allows; any other ref is the same 422.
func (s *Service) checkMessage(
	c *call, mt *realtimecfg.MessageType, props map[string]any, maxBytes int,
) (validProps, map[string]RefView, error) {
	vp, err := validateProps(mt, props, maxBytes)
	if err != nil {
		return validProps{}, nil, err
	}
	if !whenHolds(c.ctx, mt, vp.props, c.now) {
		return validProps{}, nil, forbidden(TypePostDenied,
			"the message does not meet the when: condition of "+mt.Name,
			"channel_types."+c.ct.Name+".message_types."+mt.Name+".when")
	}
	if len(vp.refs) == 0 {
		return vp, nil, nil
	}
	got, err := s.refs.Resolve(c.ctx, vp.refs)
	if err != nil {
		return validProps{}, nil, fmt.Errorf("realtime: resolve refs: %w", err)
	}
	for name, p := range mt.Properties {
		if p.Type != realtimecfg.PropertyRef {
			continue
		}
		id, ok := vp.props[name].(string)
		if !ok {
			continue
		}
		ref, readable := got[id]
		if !readable || (len(p.To) > 0 && !slices.Contains(p.To, ref.Type)) {
			return validProps{}, nil, invalidRef()
		}
	}
	return vp, got, nil
}

// postSignal keeps an ephemeral message in memory.
func (s *Service) postSignal(
	c *call, mt *realtimecfg.MessageType, vp validProps, replyTo int64,
) (PostResult, error) {
	if replyTo != 0 {
		return PostResult{}, invalid("an ephemeral message cannot reply")
	}
	ttl := realtimelimits.MaxEphemeralTTL
	if mt.TTL > 0 {
		ttl = min(mt.TTL, ttl)
	}
	sg := signal{
		id: channels.NewName(), typ: mt.Name, author: authorOf(c.p), identity: bucketIdentity(c.p),
		at: c.now, expires: c.now.Add(ttl), props: vp.props, size: vp.size,
	}
	expired, err := s.local.ephemeral.add(c.key, sg, c.now)
	s.local.mark(expired...)
	if err != nil {
		return PostResult{}, err
	}
	s.local.mark(c.key)
	s.local.scheduleExpiry()
	return PostResult{Ephemeral: true, ID: sg.id, ExpiresAt: sg.expires}, nil
}

// record writes the type record of c's channel type to the store before a
// stored write, so boot and reload can refuse a change of its anchor, its
// cardinality or a ref property. The store records a type once and only
// adds refs after that; a record that conflicts with the held one refuses
// the write with a 409 channel_type_conflict. Local's cache skips the
// store call for a record this process already wrote.
//
// After the record it checks that the live realtime.yaml still gives the
// type the shape it recorded. A reload checks the store's records before
// it applies, so one that ran before this record could not see it;
// refusing here keeps a write from landing under a shape the live config
// no longer has. It compares shapes, not config generations, so a reload
// that leaves this type's shape alone refuses nothing.
func (s *Service) record(c *call) error {
	rec := typeRecord(c.ct)
	if s.local.needsRecord(c.ct.Name, rec) {
		err := s.store.RecordTypes(c.ctx, map[string]channels.TypeRecord{c.ct.Name: rec})
		if errors.Is(err, channels.ErrTypeShapeConflict) {
			s.local.forgetStoredRefs()
			return typeConflict(c.ct.Name)
		}
		if err != nil {
			return fmt.Errorf("realtime: record channel type %s: %w", c.ct.Name, err)
		}
		s.local.noteRecorded(c.ct.Name, rec)
	}
	live := s.cfg.Load().Config
	if live != c.snap.Config {
		ct, ok := live.ChannelType(c.ct.Name)
		if !ok || !typeRecord(ct).Equal(rec) {
			return configChanged()
		}
	}
	return nil
}

// typeRecord is the store record of ct's configured shape.
func typeRecord(ct realtimecfg.ChannelType) channels.TypeRecord {
	shape := ct.Shape()
	return channels.TypeRecord{
		Anchor:      channels.AnchorMode(shape.Anchor),
		Cardinality: channels.Cardinality(shape.Cardinality),
		Refs:        shape.Refs,
		Plain:       ct.PlainProperties(),
	}
}

// CreateInput is a named channel to create.
type CreateInput struct {
	Title string
}

// Create creates a named channel of type typ on the anchor. Only a type
// with cardinality many has named channels.
func (s *Service) Create(
	ctx context.Context, entityType, anchor, typ string, in CreateInput,
) (ChannelView, error) {
	c, openErr := s.begin(ctx)
	if openErr != nil {
		return ChannelView{}, openErr
	}
	if err := c.channelType(entityType, typ); err != nil {
		return ChannelView{}, err
	}
	if c.ct.Cardinality != realtimecfg.CardinalityMany || c.ct.Create == nil {
		return ChannelView{}, deny(http.StatusConflict, TypeNotCreatable,
			fmt.Sprintf("channel type %s has one channel per anchor, created by its first post", typ))
	}
	if err := s.resolveAnchor(c, anchor); err != nil {
		return ChannelView{}, err
	}
	// The caller becomes the creator, so a creator read role admits them.
	if c.ct.Read.Source.Kind != realtimecfg.SourceCreator && !c.can(c.ct.Read, realtimecfg.CapabilityRead) {
		return ChannelView{}, readDenied(c.ct)
	}
	if err := c.requireCaller(); err != nil {
		return ChannelView{}, err
	}
	if !c.gate(*c.ct.Create) {
		return ChannelView{}, gateDenied(TypeCreateDenied, "creating "+typ+" channels",
			"channel_types."+typ+".create", *c.ct.Create, c.ct.Roles)
	}
	if err := s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opCreate, realtimelimits.OpRate); err != nil {
		return ChannelView{}, err
	}
	if err := s.record(c); err != nil {
		return ChannelView{}, err
	}
	// The store counts the channels of this type on the anchor and creates
	// in one step, so concurrent creates cannot pass channels_per_entity.
	ch, err := s.store.CreateMany(c.ctx, c.key.Anchor, channels.NewChannel{
		Type: typ, Title: in.Title, By: authorOf(c.p), At: c.now,
		Max: c.snap.Config.Limits().ChannelsPerEntity,
	})
	if err != nil {
		return ChannelView{}, storeError(err)
	}
	s.local.mark(ch.Key)
	return channelView(ch.Key, &ch), nil
}

// TypeChannels are the channels of one type on an anchor that the caller
// may read.
type TypeChannels struct {
	Type        string                  `json:"type"`
	Label       string                  `json:"label,omitempty"`
	Cardinality realtimecfg.Cardinality `json:"cardinality"`
	// Create reports whether the caller may create a channel of the type
	// here.
	Create   bool          `json:"create"`
	Channels []ChannelView `json:"channels"`
}

// AnchorChannels are the channel types and channels of one anchor.
type AnchorChannels struct {
	Anchor string         `json:"anchor"`
	Types  []TypeChannels `json:"types"`
}

// ListAnchor lists the channels on the anchor that the caller may read,
// grouped by type, in one anchor read and one channel list. A bare id lists
// the family channels; `ID@face` lists the family channels and those of
// that face. An empty state lists every state.
func (s *Service) ListAnchor(
	ctx context.Context, entityType, anchor string, state channels.State,
) (AnchorChannels, error) {
	c, err := s.begin(ctx)
	if err != nil {
		return AnchorChannels{}, err
	}
	addr, err := entity.ParseAddress(anchor)
	if err != nil {
		return AnchorChannels{}, ErrAnchorNotFound
	}
	_, named := addr.Named()
	var types []realtimecfg.ChannelType
	var props []string
	for _, ct := range c.snap.Config.ChannelTypes() {
		if entityType != "" && !slices.Contains(ct.AttachTo, entityType) {
			continue
		}
		if ct.Anchor == realtimecfg.AnchorFace && !named {
			continue
		}
		types = append(types, ct)
		props = append(props, roleProps(ct)...)
	}
	slices.Sort(props)
	view, err := s.anchors.Anchor(c.ctx, addr, slices.Compact(props))
	if err != nil {
		if errors.Is(err, ErrAnchorNotFound) {
			return AnchorChannels{}, ErrAnchorNotFound
		}
		return AnchorChannels{}, fmt.Errorf("realtime: read anchor: %w", err)
	}
	if entityType != "" && view.Type != entityType {
		return AnchorChannels{}, ErrAnchorNotFound
	}
	c.anchor = view
	stored, err := s.store.ListEntity(c.ctx, view.ID)
	if err != nil {
		return AnchorChannels{}, fmt.Errorf("realtime: list channels: %w", err)
	}
	out := AnchorChannels{Anchor: addr.String(), Types: []TypeChannels{}}
	for _, ct := range types {
		if !slices.Contains(ct.AttachTo, view.Type) {
			continue
		}
		if tc, ok := c.listType(ct, stored, state); ok {
			out.Types = append(out.Types, tc)
		}
	}
	return out, nil
}

// listType collects the readable channels of ct from stored. A type whose
// read role the caller fails is left out; for a creator read role, each
// channel is checked on its own.
func (c *call) listType(
	ct realtimecfg.ChannelType, stored []channels.Channel, state channels.State,
) (TypeChannels, bool) {
	c.ct, c.head = ct, nil
	anchor := anchorFor(ct, c.anchor)
	c.key = channels.Key{Anchor: anchor, Type: ct.Name}
	creator := ct.Read.Source.Kind == realtimecfg.SourceCreator
	if !creator && !c.can(ct.Read, realtimecfg.CapabilityRead) {
		return TypeChannels{}, false
	}
	tc := TypeChannels{Type: ct.Name, Label: ct.Label, Cardinality: ct.Cardinality, Channels: []ChannelView{}}
	for i := range stored {
		ch := stored[i]
		if ch.Key.Type != ct.Name || ch.Key.Anchor != anchor || (state != "" && ch.State != state) {
			continue
		}
		if creator {
			c.head = &ch
			if !c.can(ct.Read, realtimecfg.CapabilityRead) {
				continue
			}
		}
		tc.Channels = append(tc.Channels, channelView(ch.Key, &ch))
	}
	c.head = nil
	tc.Create = ct.Cardinality == realtimecfg.CardinalityMany && ct.Create != nil &&
		c.requireCaller() == nil && c.gate(*ct.Create)
	return tc, true
}
