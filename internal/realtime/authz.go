package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// RawAddress names a channel as a request does, before anything is
// resolved: the entity type in the URL (empty when the route names none),
// the anchor as "ID" or "ID@face", the channel type and, for a type with
// cardinality many, the channel name.
type RawAddress struct {
	EntityType string
	Anchor     string
	Type       string
	Name       string
}

// call is the state one operation captures once at its start: the config
// snapshot, the access answers, the caller, the time and, once resolved,
// the channel type, the anchor and the channel.
type call struct {
	ctx  context.Context //nolint:containedctx // one operation's context, derived by AccessSource.For
	snap Snapshot
	acc  Access
	p    principal.Principal
	now  time.Time

	ct realtimecfg.ChannelType
	// entityType is the entity type the route names, empty when it names
	// none. The anchor must be of that type.
	entityType string
	anchor     AnchorView
	key        channels.Key
	// head is the stored channel, nil for a cardinality-one channel that
	// holds no message yet.
	head *channels.Channel

	// promoter is the service's [Promoter]; promoteChecks caches its
	// [Promoter.CheckCreate] answer per promote target.
	promoter      Promoter
	promoteChecks map[string]error
	// actionGates caches the gate answer per message type and action.
	actionGates map[string]bool
	// ops caches the message operation gates; see [call.opGates].
	ops *opGates
}

// begin captures the per-operation state: one config snapshot, one Access
// over one ACL request, one clock reading. With realtime off it answers
// [ErrRealtimeOff] before it asks anything about the caller. A caller the
// ACL cannot evaluate ([ErrNoPrincipal]) is a 403 no_principal; any other
// failure to open the access is a server error.
func (s *Service) begin(ctx context.Context) (*call, error) {
	snap := s.cfg.Load()
	if snap.Config == nil {
		return nil, ErrRealtimeOff
	}
	actx, acc, err := s.access.For(ctx)
	if errors.Is(err, ErrNoPrincipal) {
		return nil, noPrincipal()
	}
	if err != nil {
		return nil, fmt.Errorf("realtime: open access: %w", err)
	}
	return &call{
		ctx: actx, snap: snap, acc: acc, p: acc.Principal(), now: s.clock(), promoter: s.promoter,
	}, nil
}

func noPrincipal() *DenialError {
	return forbidden(TypeNoPrincipal, "the request carries no identity the ACL can evaluate", "")
}

// requireCaller refuses a write from a caller with no identity: channel
// writes record their author.
func (c *call) requireCaller() error {
	if c.p.IsZero() || c.p.User == "" || c.p.User == principal.Unknown {
		return noPrincipal()
	}
	return nil
}

// channelType resolves the channel type named typ for a route on
// entityType. An unknown type, or a type that does not attach to
// entityType, is 404 unknown_channel_type, answered before the anchor is
// read: the configuration is not secret.
func (c *call) channelType(entityType, typ string) error {
	ct, ok := c.snap.Config.ChannelType(typ)
	if !ok || (entityType != "" && !slices.Contains(ct.AttachTo, entityType)) {
		return unknownChannelType(typ)
	}
	c.ct, c.entityType = ct, entityType
	return nil
}

// resolveAnchor reads the anchor of c.ct and sets c.anchor and the anchor
// part of c.key. A face-anchored type needs `ID@face` (422 face_required,
// before the read). A miss, or an entity of another type than the route
// names, is [ErrAnchorNotFound], as the entity GET under that plural
// answers 404. A readable entity whose type the channel type does not
// attach to is 404 unknown_channel_type; only a route that names no entity
// type (MCP) can reach it.
func (s *Service) resolveAnchor(c *call, raw string) error {
	addr, err := entity.ParseAddress(raw)
	if err != nil {
		return ErrAnchorNotFound
	}
	_, named := addr.Named()
	if c.ct.Anchor == realtimecfg.AnchorFace && !named {
		return deny(http.StatusUnprocessableEntity, TypeFaceRequired,
			fmt.Sprintf("channel type %s anchors to a face; address the entity as ID@face", c.ct.Name))
	}
	view, err := s.anchors.Anchor(c.ctx, addr, anchorProps(c.ct))
	if err != nil {
		if errors.Is(err, ErrAnchorNotFound) {
			return ErrAnchorNotFound
		}
		return fmt.Errorf("realtime: read anchor: %w", err)
	}
	if c.entityType != "" && view.Type != c.entityType {
		return ErrAnchorNotFound
	}
	if !slices.Contains(c.ct.AttachTo, view.Type) {
		// Only a route that names no entity type gets here: the caller may
		// read the entity, so naming its type tells them nothing new.
		return deny(http.StatusNotFound, TypeUnknownChannelType,
			fmt.Sprintf("channel type %s does not attach to %s entities", c.ct.Name, view.Type))
	}
	c.anchor = view
	c.key = channels.Key{Anchor: anchorFor(c.ct, view), Type: c.ct.Name}
	return nil
}

// anchorFor is the channel anchor of view under ct: the family for a
// family type, the admitted face for a face type.
func anchorFor(ct realtimecfg.ChannelType, view AnchorView) channels.Anchor {
	if ct.Anchor == realtimecfg.AnchorFace {
		return channels.Anchor{ID: view.ID, Face: view.Face}
	}
	return channels.Anchor{ID: view.ID}
}

// roleProps lists the anchor properties the roles of ct read.
func roleProps(ct realtimecfg.ChannelType) []string {
	var out []string
	add := func(r realtimecfg.Role) {
		if r.Source.Kind == realtimecfg.SourceEntityProperty && !slices.Contains(out, r.Source.Property) {
			out = append(out, r.Source.Property)
		}
	}
	add(ct.Read)
	for _, r := range ct.Roles {
		add(r)
	}
	slices.Sort(out)
	return out
}

// open resolves addr through the read check: channel type, anchor, read
// role, then the channel itself. The order is the point: a caller who may
// not read the channel learns nothing about its name, and the anchor is
// read only for a type that exists.
func (s *Service) open(ctx context.Context, addr RawAddress) (*call, error) {
	c, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.channelType(addr.EntityType, addr.Type); err != nil {
		return nil, err
	}
	if err := s.resolveAnchor(c, addr.Anchor); err != nil {
		return nil, err
	}
	if c.ct.Read.Source.Kind == realtimecfg.SourceCreator {
		// The creator source needs the channel; a miss reads as a denial,
		// so the read check still comes before the name is confirmed.
		if err := s.loadHead(c, addr.Name); err != nil {
			if d, ok := errors.AsType[*DenialError](err); ok && d.Type == TypeChannelNotFound {
				return nil, readDenied(c.ct)
			}
			return nil, err
		}
	}
	if !c.can(c.ct.Read, realtimecfg.CapabilityRead) {
		return nil, readDenied(c.ct)
	}
	if c.head == nil {
		if err := s.loadHead(c, addr.Name); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func readDenied(ct realtimecfg.ChannelType) *DenialError {
	return forbidden(TypeReadDenied,
		fmt.Sprintf("reading %s channels needs the %s role", ct.Name, roleLabel(ct.Read)),
		"channel_types."+ct.Name+".read")
}

// loadHead sets c.key's name and reads the channel. A many type needs a
// name and a stored channel (404 channel_not_found). A one type takes no
// name, and a missing channel reads as an empty open one.
func (s *Service) loadHead(c *call, name string) error {
	if c.ct.Cardinality == realtimecfg.CardinalityMany {
		if name == "" {
			return channelNotFound()
		}
	} else if name != "" {
		return channelNotFound()
	}
	c.key.Name = name
	if err := c.key.Validate(); err != nil {
		return channelNotFound()
	}
	h, err := s.store.Head(c.ctx, c.key)
	switch {
	case err == nil:
		c.head = &h
	case errors.Is(err, channels.ErrNotFound) && c.ct.Cardinality != realtimecfg.CardinalityMany:
		c.head = nil
	case errors.Is(err, channels.ErrNotFound):
		return channelNotFound()
	default:
		return fmt.Errorf("realtime: read channel: %w", err)
	}
	return nil
}

// state is the channel's lifecycle state; a channel not yet stored is open.
func (c *call) state() channels.State {
	if c.head == nil {
		return channels.StateOpen
	}
	return c.head.State
}

// can reports whether role admits the caller for capability on the anchor.
// The anchor was read through the gated reader, so every role already
// carries the read check its RequireAnchorRead asks for.
func (c *call) can(role realtimecfg.Role, capability realtimecfg.Capability) bool {
	src := role.Source
	ok := false
	switch src.Kind {
	case realtimecfg.SourcePermission:
		ok = c.acc.HoldsPermissionFor(c.ctx, c.anchor.ID, src.Permission)
	case realtimecfg.SourceAttached:
		ok = true
	case realtimecfg.SourceEntityProperty:
		ok = !c.snap.Degraded && c.propertyNamesCaller(src.Property)
	case realtimecfg.SourcePrincipalType:
		ok = src.PrincipalType != "" && c.p.PrincipalType() == src.PrincipalType
	case realtimecfg.SourceCreator:
		ok = c.head != nil && sameAuthor(c.head.CreatedBy, c.p)
	}
	if ok && src.NeedsCeilingCheck(capability) {
		ok = capability == realtimecfg.CapabilityRead || c.acc.CeilingAdmitsUpdate(c.anchor.Type)
	}
	return ok
}

// propertyNamesCaller reports whether the anchor's property names the
// caller, by the rule is_current_user and has_current_user apply. A
// property the caller may not see is absent from the anchor view, and
// matches nobody.
func (c *call) propertyNamesCaller(prop string) bool {
	fns := predicatefns.CurrentUserBindings(c.acc.QueryIdentity().ID())
	var name string
	var arg predicate.Value
	switch v := c.anchor.Props[prop].(type) {
	case string:
		name, arg = predicatefns.FuncIsCurrentUser, predicate.NewString(v)
	case []string:
		name, arg = predicatefns.FuncHasCurrentUser, stringList(v)
	case []any:
		elems := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				elems = append(elems, s)
			}
		}
		name, arg = predicatefns.FuncHasCurrentUser, stringList(elems)
	default:
		return false
	}
	got, err := fns[name](c.ctx, []predicate.Value{arg})
	if err != nil {
		return false
	}
	b, ok := got.(predicate.Bool)
	return ok && b.Bool()
}

// conditionHolds reports whether prog, a lifecycle `condition:` compiled
// for the anchor's type, holds for the anchor's redacted record. A missing
// program or record, or an evaluation error, does not hold: a condition
// fails closed. An evaluation error is logged with rule, the condition's
// key in realtime.yaml, because the caller sees only the refusal.
func (c *call) conditionHolds(prog *predicate.Program, rule string) bool {
	if prog == nil || c.anchor.Record == nil {
		return false
	}
	b := predicate.NewBindings()
	err := b.SetVar("entity", c.anchor.Record)
	if err == nil {
		err = predicatefns.Bind(b, c.now)
	}
	var v predicate.Value
	if err == nil {
		v, err = prog.Eval(c.ctx, b)
	}
	if err != nil {
		slog.WarnContext(c.ctx, "realtime: lifecycle condition failed to evaluate; refusing",
			"rule", rule, "anchor", c.anchor.ID, "error", err)
		return false
	}
	got, ok := v.(predicate.Bool)
	return ok && got.Bool()
}

func stringList(ss []string) predicate.Value {
	elems := make([]predicate.Value, len(ss))
	for i, s := range ss {
		elems[i] = predicate.NewString(s)
	}
	return predicate.NewList(elems)
}

// sameAuthor reports whether a was written by p: the same user in the same
// organization. The tool is ignored, so a user's browser and agent count
// as one author.
func sameAuthor(a channels.Author, p principal.Principal) bool {
	return a.User != "" && a.User == p.User && a.OrgID == p.OrgID()
}

// authorOf is the author record of p.
func authorOf(p principal.Principal) channels.Author {
	return channels.Author{User: p.User, Tool: p.Tool, PrincipalType: p.PrincipalType(), OrgID: p.OrgID()}
}

// gate reports whether g admits the caller: any of its roles admits them
// for update, or they hold its permission and the client ceiling admits
// update. A gate is always a write.
func (c *call) gate(g realtimecfg.Gate) bool {
	for _, name := range g.Roles {
		if r, ok := c.ct.Roles[name]; ok && c.can(r, realtimecfg.CapabilityUpdate) {
			return true
		}
	}
	return g.Permission != "" &&
		c.acc.HoldsPermissionFor(c.ctx, c.anchor.ID, g.Permission) &&
		c.acc.CeilingAdmitsUpdate(c.anchor.Type)
}

// gateDenied is the 403 for a refused gate: the operator's `denied:`
// message when set, else a title naming the roles and permission.
func gateDenied(typ, what, rule string, g realtimecfg.Gate, roles map[string]realtimecfg.Role) *DenialError {
	title := g.Denied
	if title == "" {
		title = fmt.Sprintf("%s needs %s", what, describeGate(g, roles))
	}
	return forbidden(typ, title, rule)
}

// describeGate names who a gate admits, for a refusal.
func describeGate(g realtimecfg.Gate, roles map[string]realtimecfg.Role) string {
	var parts []string
	for _, name := range g.Roles {
		parts = append(parts, "the "+roleLabel(roles[name])+" role")
	}
	if g.Permission != "" {
		parts = append(parts, "the "+g.Permission+" permission")
	}
	if len(parts) == 0 {
		return "a role no one holds"
	}
	return strings.Join(parts, " or ")
}

func roleLabel(r realtimecfg.Role) string {
	if r.Label != "" {
		return r.Label
	}
	if r.Name != "" {
		return r.Name
	}
	return "read"
}

// moderator reports whether the caller passes the remove gate of c's
// channel type: they may remove any message, and see who removed one and
// why. Every renderer and the delete path decide it here.
func (c *call) moderator() bool {
	ops := c.ct.MessageOps
	return ops.Remove != nil && c.requireCaller() == nil && c.gate(ops.Remove.Gate)
}

// mayPost reports whether the caller holds a post role of mt.
func (c *call) mayPost(mt *realtimecfg.MessageType) bool {
	for _, name := range mt.Post {
		if r, ok := c.ct.Roles[name]; ok && c.can(r, realtimecfg.CapabilityUpdate) {
			return true
		}
	}
	return false
}

func postDenied(ct realtimecfg.ChannelType, mt *realtimecfg.MessageType) *DenialError {
	return gateDenied(TypePostDenied, "posting "+mt.Name+" messages",
		"channel_types."+ct.Name+".message_types."+mt.Name+".post",
		realtimecfg.Gate{Roles: mt.Post}, ct.Roles)
}
