package realtimecfg

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// namePattern is the grammar of channel type, message type, role, action and
// trigger names. The channel store uses type names as path segments, so the
// grammar keeps them short, lowercase and free of separators.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// propertyNamePattern is the grammar of message property names, which are
// JSON keys in every message.
var propertyNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

const nameRule = "must match ^[a-z][a-z0-9-]{0,62}$"

// validator collects every problem in one pass, so the operator sees them
// all at once rather than one per edit.
type validator struct {
	meta     *metamodel.Metamodel
	policy   *acl.Policy
	problems []Problem
	warnings []Problem
	perms    map[string]bool
}

func (v *validator) add(key, reason string) {
	v.problems = append(v.problems, Problem{Key: key, Reason: reason})
}

// warn records a part of the file that is left out rather than refused.
func (v *validator) warn(key, reason string) {
	v.warnings = append(v.warnings, Problem{Key: key, Reason: reason})
}

func (v *validator) compile(raw *rawFile) *Compiled {
	v.perms = map[string]bool{}
	if raw.Version != 1 {
		v.add("version", fmt.Sprintf("must be 1, got %d", raw.Version))
	}
	c := &Compiled{
		limits:       v.limits(raw.Limits),
		channelTypes: map[string]*ChannelType{},
		triggers:     map[string]*Trigger{},
	}
	for _, name := range sortedKeys(raw.ChannelTypes) {
		rct := raw.ChannelTypes[name]
		if ct := v.channelType(name, &rct, c.limits); ct != nil {
			c.channelTypes[name] = ct
		}
	}
	for _, name := range sortedKeys(raw.Triggers) {
		rt := raw.Triggers[name]
		if t := v.trigger(name, &rt, raw, c); t != nil {
			c.triggers[name] = t
		}
	}
	c.warnings = v.warnings
	for p := range v.perms {
		c.permissions = append(c.permissions, p)
	}
	sort.Strings(c.permissions)
	return c
}

func (v *validator) limits(raw *rawLimits) Limits {
	l := defaultLimits()
	if raw == nil {
		return l
	}
	l.LogRate = v.lowerRate("limits.post_rate", raw.PostRate, l.LogRate)
	l.EphemeralRate = v.lowerRate("limits.signal_rate", raw.SignalRate, l.EphemeralRate)
	l.MessageBytes = v.lowerInt("limits.message_bytes", raw.MessageBytes, l.MessageBytes)
	l.ChannelsPerEntity = v.lowerInt("limits.channels_per_entity", raw.ChannelsPerEntity, l.ChannelsPerEntity)
	l.PageLimit = v.lowerInt("limits.page_limit", raw.PageLimit, l.PageLimit)
	return l
}

// lowerRate parses s, which may only lower limit. An empty s keeps limit.
func (v *validator) lowerRate(key, s string, limit realtimelimits.Rate) realtimelimits.Rate {
	if s == "" {
		return limit
	}
	r, err := realtimelimits.ParseRate(s)
	if err != nil {
		v.add(key, err.Error())
		return limit
	}
	if !r.AtMost(limit) {
		v.add(key, fmt.Sprintf("%s is above the limit of %s, on average or in one burst; "+
			"realtime.yaml may only lower it", r, limit))
		return limit
	}
	return r
}

func (v *validator) lowerInt(key string, n *int, limit int) int {
	if n == nil {
		return limit
	}
	if *n <= 0 || *n > limit {
		v.add(key, fmt.Sprintf("must be between 1 and %d; realtime.yaml may only lower it", limit))
		return limit
	}
	return *n
}

func (v *validator) channelType(name string, raw *rawChannelType, limits Limits) *ChannelType {
	key := "channel_types." + name
	if !namePattern.MatchString(name) {
		v.add(key, "channel type name "+nameRule)
	}
	ct := &ChannelType{
		Name: name, Label: orDefault(raw.Label, name),
		Roles: map[string]Role{}, MessageTypes: map[string]*MessageType{},
	}
	ct.AttachTo = v.attachTo(key+".attach_to", raw.AttachTo)
	ct.Anchor = v.anchor(key+".anchor", raw.Anchor, ct.AttachTo)
	ct.Cardinality = v.cardinality(key+".cardinality", raw.Cardinality)
	ct.OnEntityDelete = v.onEntityDelete(key+".on_entity_delete", raw.OnEntityDelete)

	for _, rn := range sortedKeys(raw.Roles) {
		rr := raw.Roles[rn]
		if !namePattern.MatchString(rn) {
			v.add(key+".roles."+rn, "role name "+nameRule)
		}
		ct.Roles[rn] = Role{Name: rn, Label: orDefault(rr.Label, rn), Source: v.source(key+".roles."+rn, &rr, ct)}
	}
	read := rawRole{From: "attached"}
	if raw.Read != nil {
		read = *raw.Read
	}
	ct.Read = Role{
		Name: "read", Label: read.Label, Source: v.source(key+".read", &read, ct),
		RequireAnchorRead: true,
	}

	switch {
	case ct.Cardinality == CardinalityMany && (raw.Create == nil || len(raw.Create.Roles) == 0):
		v.add(key+".create", "a cardinality many channel type needs create.roles, or nobody can open a channel")
	case ct.Cardinality == CardinalityOne && raw.Create != nil:
		v.add(key+".create", "only a cardinality many channel type has create; a cardinality one channel opens on first write")
	case raw.Create != nil:
		ct.Create = &Gate{Label: raw.Create.Label, Roles: v.roleRefs(key+".create.roles", raw.Create.Roles, ct)}
	}

	if len(raw.MessageTypes) == 0 {
		v.add(key+".message_types", "a channel type needs at least one message type")
	}
	for _, mn := range sortedKeys(raw.MessageTypes) {
		rm := raw.MessageTypes[mn]
		if mt := v.messageType(key+".message_types."+mn, mn, &rm, ct, limits); mt != nil {
			ct.MessageTypes[mn] = mt
		}
	}
	ct.Lifecycle = v.lifecycle(key+".lifecycle", raw.Lifecycle, ct)
	ct.DetachedRetention = DefaultDetachedRetention()
	if raw.DetachedRetention != "" {
		p := v.period(key+".detached_retention", raw.DetachedRetention)
		if d, fixed := p.Fixed(); !p.IsZero() && fixed && d < realtimelimits.OrphanGrace {
			// A shorter retention could erase a batch the sweep would still
			// put back: an orphan whose entity returns, or a pending batch
			// whose write failed.
			v.add(key+".detached_retention", fmt.Sprintf("must be at least %s", realtimelimits.OrphanGrace))
		} else if !p.IsZero() {
			ct.DetachedRetention = p
		}
	}
	if ct.OnEntityDelete == OnDeleteDelete && ct.Lifecycle.Delete != nil && !ct.Lifecycle.Delete.Retention.IsZero() {
		v.add(key+".on_entity_delete", "on_entity_delete: delete would erase channels that "+
			"lifecycle.delete.retention must keep; use detach or block, or drop the retention")
	}
	ct.MessageOps = v.messageOps(key+".message_ops", raw.MessageOps, ct)
	return ct
}

func (v *validator) attachTo(key string, types []string) []string {
	if len(types) == 0 {
		v.add(key, "name at least one entity type")
		return nil
	}
	out := make([]string, 0, len(types))
	for i, t := range types {
		k := fmt.Sprintf("%s[%d]", key, i)
		switch {
		case !v.meta.HasEntityType(t):
			v.add(k, fmt.Sprintf("unknown entity type %q", t))
		case v.meta.ResolveAlias(t) != t:
			v.add(k, fmt.Sprintf("%q is an alias of %q; name the type itself", t, v.meta.ResolveAlias(t)))
		case slices.Contains(out, t):
			v.add(k, fmt.Sprintf("%q is listed twice", t))
		default:
			out = append(out, t)
		}
	}
	return out
}

func (v *validator) anchor(key, s string, attachTo []string) Anchor {
	switch Anchor(s) {
	case "", AnchorFamily:
		return AnchorFamily
	case AnchorFace:
		for _, t := range attachTo {
			if len(metamodel.FaceOrderOf(v.meta, t)) == 0 {
				v.add(key, fmt.Sprintf("anchor face needs every attach_to type to declare faces; %q declares none", t))
			}
		}
		return AnchorFace
	}
	v.add(key, fmt.Sprintf("must be family or face, got %q", s))
	return AnchorFamily
}

func (v *validator) cardinality(key, s string) Cardinality {
	switch Cardinality(s) {
	case "", CardinalityOne:
		return CardinalityOne
	case CardinalityMany:
		return CardinalityMany
	}
	v.add(key, fmt.Sprintf("must be one or many, got %q", s))
	return CardinalityOne
}

func (v *validator) onEntityDelete(key, s string) OnEntityDelete {
	switch OnEntityDelete(s) {
	case "", OnDeleteDetach:
		return OnDeleteDetach
	case OnDeleteDelete, OnDeleteBlock:
		return OnEntityDelete(s)
	}
	v.add(key, fmt.Sprintf("must be delete, detach or block, got %q", s))
	return OnDeleteDetach
}

// source compiles a role's single source. It reports a role with none or
// with more than one.
func (v *validator) source(key string, r *rawRole, ct *ChannelType) Source {
	set := 0
	for _, b := range []bool{r.From != "", r.Permission != "", r.PrincipalType != "", r.Creator} {
		if b {
			set++
		}
	}
	if set != 1 {
		v.add(key, "name exactly one of from, permission, principal_type or creator")
		return Source{}
	}
	switch {
	case r.Permission != "":
		v.permission(key+".permission", r.Permission)
		return Source{Kind: SourcePermission, Permission: r.Permission}
	case r.PrincipalType != "":
		return Source{Kind: SourcePrincipalType, PrincipalType: r.PrincipalType}
	case r.Creator:
		if ct.Cardinality != CardinalityMany {
			v.add(key+".creator", "only a cardinality many channel has a creator")
		}
		return Source{Kind: SourceCreator}
	case r.From == "attached":
		return Source{Kind: SourceAttached}
	}
	prop, ok := strings.CutPrefix(r.From, "entity.")
	if !ok || prop == "" {
		v.add(key+".from", fmt.Sprintf("must be attached or entity.<property>, got %q", r.From))
		return Source{}
	}
	for _, t := range ct.AttachTo {
		v.identityProperty(key+".from", t, prop)
	}
	return Source{Kind: SourceEntityProperty, Property: prop}
}

// identityProperty checks that entityType has a string property prop that
// can name a principal, and that acl.yaml does not restrict it: a role
// decided by a hidden value would tell its members what the value is.
func (v *validator) identityProperty(key, entityType, prop string) {
	def, _ := v.meta.GetEntityDef(entityType)
	pd, ok := def.Properties[prop]
	if !ok {
		v.add(key, fmt.Sprintf("%s has no property %q", entityType, prop))
		return
	}
	if pd.Type != metamodel.PropertyTypeString {
		v.add(key, fmt.Sprintf("%s.%s is a %s property; a role can match only a string property naming a principal",
			entityType, prop, pd.Type))
		return
	}
	if v.policy.FieldRestricted(entityType, prop) {
		v.add(key, fmt.Sprintf("%s.%s is restricted by a visible: grant in acl.yaml; "+
			"a role decided by it would disclose its value", entityType, prop))
	}
}

// permission records perm for the acl audit and, when acl.yaml exists,
// checks that some role grants it: a gate on a permission nobody holds is
// closed for everyone, which is almost always a typo.
func (v *validator) permission(key, perm string) {
	v.perms[perm] = true
	if v.policy == nil || acl.IsTagPermission(perm) || slices.Contains(acl.BuiltinPermissions(), perm) {
		return
	}
	for _, role := range v.policy.Roles {
		if slices.Contains(role.Permissions, perm) {
			return
		}
	}
	v.add(key, fmt.Sprintf("no role in acl.yaml grants permission %q", perm))
}

// roleRefs checks that every name is a declared role of ct.
func (v *validator) roleRefs(key string, names []string, ct *ChannelType) []string {
	for i, n := range names {
		if _, ok := ct.Roles[n]; !ok {
			v.add(fmt.Sprintf("%s[%d]", key, i), fmt.Sprintf("unknown role %q; declare it under roles", n))
		}
	}
	return slices.Clone(names)
}

// gate compiles roles plus an optional permission. An operation with
// neither would be declared and yet open to nobody.
func (v *validator) gate(key string, roles []string, perm, label, denied string, ct *ChannelType) Gate {
	if len(roles) == 0 && perm == "" {
		v.add(key, "name roles or a permission")
	}
	if perm != "" {
		v.permission(key+".permission", perm)
	}
	return Gate{Label: label, Roles: v.roleRefs(key+".roles", roles, ct), Permission: perm, Denied: denied}
}

func (v *validator) messageType(key, name string, raw *rawMessageType, ct *ChannelType, limits Limits) *MessageType {
	if !namePattern.MatchString(name) {
		v.add(key, "message type name "+nameRule)
	}
	mt := &MessageType{
		Name: name, Label: orDefault(raw.Label, name),
		Properties: map[string]Property{}, Actions: map[string]*Action{},
	}
	switch Persist(raw.Persist) {
	case "", PersistLog:
		mt.Persist = PersistLog
		mt.Rate = v.lowerRate(key+".rate", raw.Rate, limits.LogRate)
		if raw.TTL != "" {
			v.add(key+".ttl", "only an ephemeral message type has a ttl")
		}
	case PersistEphemeral:
		mt.Persist = PersistEphemeral
		mt.Rate = v.lowerRate(key+".rate", raw.Rate, limits.EphemeralRate)
		mt.TTL = v.ttl(key+".ttl", raw.TTL)
		if raw.PromoteTo != nil {
			v.add(key+".promote_to", "an ephemeral message is never stored, so it cannot be promoted")
		}
		if len(raw.Actions) > 0 {
			v.add(key+".actions", "an ephemeral message is never stored, so it cannot carry actions")
		}
	default:
		v.add(key+".persist", fmt.Sprintf("must be log or ephemeral, got %q", raw.Persist))
	}

	if len(raw.Properties) > realtimelimits.MaxMessageProperties {
		v.add(key+".properties", fmt.Sprintf("at most %d properties", realtimelimits.MaxMessageProperties))
	}
	for _, pn := range sortedKeys(raw.Properties) {
		rp := raw.Properties[pn]
		mt.Properties[pn] = v.property(key+".properties."+pn, pn, &rp, limits)
	}
	if raw.When != "" {
		if env, err := messageEnv(mt.Properties); err != nil {
			v.add(key+".when", err.Error())
		} else if prog, err := predicate.Compile(env, raw.When); err != nil {
			v.add(key+".when", "does not compile: "+err.Error())
		} else {
			mt.When = prog
		}
	}
	if len(raw.Post) == 0 {
		v.add(key+".post", "name the roles that may post this message type")
	}
	mt.Post = v.roleRefs(key+".post", raw.Post, ct)
	if mt.Persist == PersistLog {
		if raw.PromoteTo != nil {
			v.add(key+".promote_to", "promote_to belongs on an action: declare an action with its own "+
				"roles or permission and put promote_to there")
		}
		for _, an := range sortedKeys(raw.Actions) {
			ra := raw.Actions[an]
			ak := key + ".actions." + an
			if !namePattern.MatchString(an) {
				v.add(ak, "action name "+nameRule)
			}
			mt.Actions[an] = &Action{
				Name:      an,
				Gate:      v.gate(ak, ra.Roles, ra.Permission, orDefault(ra.Label, an), ra.Denied, ct),
				PromoteTo: v.promote(ak+".promote_to", ra.PromoteTo, mt, ct),
			}
		}
	}
	return mt
}

func (v *validator) ttl(key, s string) time.Duration {
	if s == "" {
		return realtimelimits.MaxEphemeralTTL
	}
	p, err := ParsePeriod(s)
	if err != nil {
		v.add(key, err.Error())
		return realtimelimits.MaxEphemeralTTL
	}
	d, fixed := p.Fixed()
	if !fixed || d > realtimelimits.MaxEphemeralTTL {
		v.add(key, fmt.Sprintf("must be at most %s", realtimelimits.MaxEphemeralTTL))
		return realtimelimits.MaxEphemeralTTL
	}
	return d
}

func (v *validator) property(key, name string, raw *rawProperty, limits Limits) Property {
	if !propertyNamePattern.MatchString(name) {
		v.add(key, "property name must match ^[a-z][a-z0-9_]{0,62}$")
	}
	p := Property{Name: name, Label: orDefault(raw.Label, name), Required: raw.Required, To: slices.Clone(raw.To)}
	switch PropertyType(raw.Type) {
	case PropertyString, PropertyMarkdown, PropertyInteger, PropertyBoolean,
		PropertyDate, PropertyDatetime, PropertyRef:
		p.Type = PropertyType(raw.Type)
	case "":
		v.add(key+".type", "name a type")
	default:
		ct, ok := v.meta.Types[raw.Type]
		if !ok || len(ct.Values) == 0 {
			v.add(key+".type", fmt.Sprintf("unknown type %q; use string, markdown, integer, boolean, date, "+
				"datetime, ref or an enum type declared in schema.yaml", raw.Type))
		} else {
			p.Type, p.Enum, p.Values = PropertyEnum, raw.Type, slices.Clone(ct.Values)
		}
	}
	if raw.Max != nil {
		switch {
		case p.Type != PropertyString && p.Type != PropertyMarkdown:
			v.add(key+".max", "only a string or markdown property has a max")
		case *raw.Max <= 0 || *raw.Max > limits.MessageBytes:
			v.add(key+".max", fmt.Sprintf("must be between 1 and %d", limits.MessageBytes))
		default:
			p.Max = *raw.Max
		}
	}
	switch {
	case p.Type == PropertyRef && len(raw.To) == 0:
		v.add(key+".to", "a ref property names the entity types it may point to")
	case p.Type != PropertyRef && len(raw.To) > 0:
		v.add(key+".to", "only a ref property has to")
	}
	for i, t := range raw.To {
		if !v.meta.HasEntityType(t) {
			v.add(fmt.Sprintf("%s.to[%d]", key, i), fmt.Sprintf("unknown entity type %q", t))
		}
	}
	return p
}

// promote checks a promote_to block: the entity type and its face, each
// mapped entity property and message property and their types, that every
// required entity property is filled, and the relation linking the new
// entity to every attach_to type. The mapping reads message properties
// only, never the anchor entity.
func (v *validator) promote(key string, raw *rawPromote, mt *MessageType, ct *ChannelType) *Promote {
	if raw == nil {
		return nil
	}
	p := &Promote{Type: raw.Type, Face: raw.Face, Map: map[string]string{}}
	def, ok := v.meta.GetEntityDef(raw.Type)
	if !ok {
		v.add(key+".type", fmt.Sprintf("unknown entity type %q", raw.Type))
		return p
	}
	if canon := v.meta.ResolveAlias(raw.Type); canon != raw.Type {
		v.add(key+".type", fmt.Sprintf("%q is an alias of %q; name the type itself", raw.Type, canon))
	}
	if def.IsManualID() {
		v.add(key+".type", fmt.Sprintf("%s uses id_type: manual, so each entity needs a typed id; "+
			"a promote has no id to give it", raw.Type))
	}
	v.promoteFace(key+".face", raw)
	for _, ep := range sortedKeys(raw.Map) {
		mp := raw.Map[ep]
		p.Map[ep] = mp
		v.promoteMapping(key+".map."+ep, raw.Type, ep, mp, def, mt)
	}
	for _, ep := range sortedKeys(def.Properties) {
		pd := def.Properties[ep]
		if !pd.Required || pd.Default != "" || pd.Computed != "" {
			continue
		}
		mp, mapped := raw.Map[ep]
		if !mapped {
			v.add(key+".map", fmt.Sprintf("%s requires property %q; map a required message property to it", raw.Type, ep))
			continue
		}
		if msgProp, ok := mt.Properties[mp]; ok && !msgProp.Required {
			v.add(key+".map."+ep, fmt.Sprintf("%s requires property %q, but message property %s is optional; "+
				"mark it required", raw.Type, ep, mp))
		}
	}
	if raw.Relate != nil {
		p.Relate = raw.Relate.Type
		for _, t := range ct.AttachTo {
			if err := metamodel.NewValidator(v.meta).ValidateRelation(raw.Relate.Type, raw.Type, t); err != nil {
				v.add(key+".relate.type", fmt.Sprintf("cannot link %s to %s: %v", raw.Type, t, err))
			}
		}
	}
	return p
}

// promoteMapping checks that message property mp exists and can fill entity
// property ep of entityType.
func (v *validator) promoteMapping(key, entityType, ep, mp string, def *metamodel.EntityDef, mt *MessageType) {
	pd, ok := def.Properties[ep]
	if !ok {
		v.add(key, fmt.Sprintf("%s has no property %q", entityType, ep))
		return
	}
	if pd.Computed != "" {
		v.add(key, fmt.Sprintf("%s.%s is computed; it cannot be set", entityType, ep))
		return
	}
	msgProp, ok := mt.Properties[mp]
	if !ok {
		v.add(key, fmt.Sprintf("message type %s has no property %q", mt.Name, mp))
		return
	}
	if !promotable(msgProp, pd) {
		v.add(key, fmt.Sprintf("message property %s (%s) cannot fill %s.%s, a %s property",
			mp, msgProp.Type, entityType, ep, pd.Type))
	}
}

// promoteFace checks that a promote_to names a face exactly when its type
// declares faces. A create of a faced type needs one, and taking the first
// declared face would let a presentation order stand in for the operator's
// intent.
func (v *validator) promoteFace(key string, raw *rawPromote) {
	faces := metamodel.FaceOrderOf(v.meta, raw.Type)
	switch {
	case len(faces) == 0 && raw.Face != "":
		v.add(key, raw.Type+" declares no faces")
	case len(faces) > 0 && raw.Face == "":
		v.add(key, fmt.Sprintf("%s declares faces; name the face to create (%s)", raw.Type, strings.Join(faces, ", ")))
	case len(faces) > 0 && !slices.Contains(faces, raw.Face):
		v.add(key, fmt.Sprintf("%s has no face %q; use one of %s", raw.Type, raw.Face, strings.Join(faces, ", ")))
	}
}

// promotable reports whether a message property's value can fill entity
// property pd. Text and a ref (the entity id) fill a string; an enum fills a
// string or the same enum type; every other type fills only its own type.
func promotable(mp Property, pd metamodel.PropertyDef) bool {
	switch mp.Type {
	case PropertyString, PropertyMarkdown, PropertyRef:
		return pd.Type == metamodel.PropertyTypeString
	case PropertyEnum:
		return pd.Type == metamodel.PropertyTypeString || pd.Type == mp.Enum
	case PropertyInteger:
		return pd.Type == metamodel.PropertyTypeInteger
	case PropertyBoolean:
		return pd.Type == metamodel.PropertyTypeBoolean
	case PropertyDate:
		return pd.Type == metamodel.PropertyTypeDate
	case PropertyDatetime:
		return pd.Type == metamodel.PropertyTypeDatetime
	}
	return false
}

func (v *validator) lifecycle(key string, raw *rawLifecycle, ct *ChannelType) Lifecycle {
	if raw == nil {
		return Lifecycle{}
	}
	return Lifecycle{
		Close:   v.lifecycleOp(key+".close", raw.Close, ct, StateClosed),
		Reopen:  v.lifecycleOp(key+".reopen", raw.Reopen, ct, StateOpen),
		Archive: v.lifecycleOp(key+".archive", raw.Archive, ct, StateArchived),
		Delete:  v.lifecycleOp(key+".delete", raw.Delete, ct, ""),
	}
}

// lifecycleOp compiles one operation. target is the state it moves to, or
// empty for delete.
func (v *validator) lifecycleOp(key string, raw *rawLifecycleOp, ct *ChannelType, target State) *LifecycleOp {
	if raw == nil {
		return nil
	}
	op := &LifecycleOp{
		Gate:       v.gate(key, raw.Roles, raw.Permission, "", raw.Denied, ct),
		Conditions: map[string]*predicate.Program{},
	}
	if raw.From != "" {
		op.From = v.state(key+".from", raw.From)
		if op.From != "" && op.From == target {
			v.add(key+".from", fmt.Sprintf("an operation cannot start from the state it moves to (%s)", target))
		}
	}
	if raw.Condition != "" {
		op.RequireAnchorRead = true
		for _, t := range ct.AttachTo {
			if rule, ok := v.compileRule(key+".condition", t, raw.Condition); ok {
				op.Conditions[t] = rule.Program
			}
		}
	}
	if raw.Retention != "" {
		if target != "" {
			v.add(key+".retention", "only delete has a retention")
		} else {
			op.Retention = v.period(key+".retention", raw.Retention)
		}
	}
	if raw.Auto != nil {
		op.Auto = v.auto(key+".auto", raw.Auto, ct, target)
	}
	return op
}

func (v *validator) auto(key string, raw *rawAuto, ct *ChannelType, target State) *Auto {
	if target != StateClosed && target != StateArchived {
		v.add(key, "only close and archive run automatically")
		return nil
	}
	if raw.When == "" && raw.After == "" {
		v.add(key, "name when, after or both")
		return nil
	}
	a := &Auto{When: map[string]Rule{}}
	if raw.After != "" {
		a.After = v.period(key+".after", raw.After)
	}
	if raw.When != "" {
		for _, t := range ct.AttachTo {
			rule, ok := v.compileRule(key+".when", t, raw.When)
			if !ok {
				continue
			}
			if ct.Anchor == AnchorFamily && v.facedAndRestricted(t) {
				v.warn(key+".when", fmt.Sprintf("left out for %s: it declares faces and acl.yaml lets some "+
					"readers see a face other than the default world's, so the rule would judge a face they "+
					"may not see; use a channel type with anchor: face", t))
				continue
			}
			a.When[t] = rule
		}
	}
	return a
}

func (v *validator) messageOps(key string, raw *rawMessageOps, ct *ChannelType) MessageOps {
	var ops MessageOps
	if raw == nil {
		return ops
	}
	if r := raw.EditOwn; r != nil {
		ops.EditOwn = &EditOwn{Gate: v.gate(key+".edit_own", r.Roles, "", "", "", ct)}
		if r.Within != "" {
			ops.EditOwn.Within = v.period(key+".edit_own.within", r.Within)
		}
	}
	if r := raw.DeleteOwn; r != nil {
		ops.DeleteOwn = &DeleteOwn{Gate: v.gate(key+".delete_own", r.Roles, "", "", "", ct)}
		if r.From != "" {
			ops.DeleteOwn.From = v.state(key+".delete_own.from", r.From)
		}
	}
	if r := raw.Remove; r != nil {
		ops.Remove = &Remove{Gate: v.gate(key+".remove", r.Roles, "", "", "", ct), ReasonRequired: r.ReasonRequired}
	}
	return ops
}

func (v *validator) state(key, s string) State {
	switch State(s) {
	case StateOpen, StateClosed, StateArchived:
		return State(s)
	}
	v.add(key, fmt.Sprintf("must be open, closed or archived, got %q", s))
	return ""
}

func (v *validator) period(key, s string) Period {
	p, err := ParsePeriod(s)
	if err != nil {
		v.add(key, err.Error())
	}
	return p
}

func (v *validator) trigger(name string, raw *rawTrigger, file *rawFile, c *Compiled) *Trigger {
	key := "triggers." + name
	if !namePattern.MatchString(name) {
		v.add(key, "trigger name "+nameRule)
	}
	t := &Trigger{Name: name, EntityType: raw.EntityType, Channel: raw.Channel, PostType: raw.Post.Type,
		Props: map[string]string{}}
	switch TriggerOn(raw.On) {
	case TriggerEnter, TriggerLeave, TriggerChange:
		t.On = TriggerOn(raw.On)
	default:
		v.add(key+".on", fmt.Sprintf("must be enter, leave or change, got %q", raw.On))
	}
	switch WhenClosed(orDefault(raw.WhenClosed, string(WhenClosedSkip))) {
	case WhenClosedSkip:
		t.WhenClosed = WhenClosedSkip
	case WhenClosedPost:
		t.WhenClosed = WhenClosedPost
	default:
		v.add(key+".when_closed", fmt.Sprintf("must be skip or post, got %q", raw.WhenClosed))
	}
	if !v.meta.HasEntityType(raw.EntityType) {
		v.add(key+".entity_type", fmt.Sprintf("unknown entity type %q", raw.EntityType))
		return nil
	}
	if canon := v.meta.ResolveAlias(raw.EntityType); canon != raw.EntityType {
		v.add(key+".entity_type", fmt.Sprintf("%q is an alias of %q; name the type itself", raw.EntityType, canon))
		return nil
	}
	if raw.When == "" {
		v.add(key+".when", "a trigger needs a when rule")
	} else if rule, ok := v.compileRule(key+".when", raw.EntityType, raw.When); ok {
		t.When = rule
	}
	ct, ok := file.ChannelTypes[raw.Channel]
	if !ok {
		v.add(key+".channel", fmt.Sprintf("unknown channel type %q", raw.Channel))
		return t
	}
	if ct.Cardinality == string(CardinalityMany) {
		v.add(key+".channel", "a trigger posts into a cardinality one channel type")
	}
	if !slices.Contains(ct.AttachTo, raw.EntityType) {
		v.add(key+".channel", fmt.Sprintf("channel type %s does not attach to %s; a trigger posts into "+
			"the changed entity's own channel", raw.Channel, raw.EntityType))
	}
	mt, ok := ct.MessageTypes[raw.Post.Type]
	if !ok {
		v.add(key+".post.type", fmt.Sprintf("channel type %s has no message type %q", raw.Channel, raw.Post.Type))
		return t
	}
	if Persist(mt.Persist) == PersistEphemeral {
		v.add(key+".post.type", fmt.Sprintf("message type %s is ephemeral; a trigger posts only log messages, "+
			"because a signal would reach only the readers connected to the process that runs triggers", raw.Post.Type))
		return t
	}
	v.triggerProps(key+".post.props", raw.Post.Props, &mt, t)
	if cmt := compiledMessageType(c, raw.Channel, raw.Post.Type); cmt != nil {
		v.triggerMessage(key+".post.props", cmt, t, c.limits)
	}
	if v.facesUnsafe(raw.EntityType, c, raw.Channel) {
		v.warn(key, fmt.Sprintf("left out: %s declares faces and acl.yaml lets some readers see a face other "+
			"than the default world's, so a message about the family would describe a face they may not "+
			"see; use a channel type with anchor: face", raw.EntityType))
		return nil
	}
	return t
}

// compiledMessageType returns the compiled message type, or nil when its
// channel type did not compile.
func compiledMessageType(c *Compiled, channel, name string) *MessageType {
	ct, ok := c.channelTypes[channel]
	if !ok {
		return nil
	}
	return ct.MessageTypes[name]
}

// triggerMessage checks the message a trigger posts as the service will
// when it posts it: each literal within its property's max, the whole
// within message_bytes, and the message type's `when:` holding. A trigger
// whose message the service would refuse could never post.
func (v *validator) triggerMessage(key string, mt *MessageType, t *Trigger, limits Limits) {
	props := make(map[string]any, len(t.Props))
	for _, pn := range sortedKeys(t.Props) {
		val := t.Props[pn]
		props[pn] = val
		if p, ok := mt.Properties[pn]; ok && p.Max > 0 && utf8.RuneCountInString(val) > p.Max {
			v.add(key+"."+pn, fmt.Sprintf("is longer than the property's max of %d characters", p.Max))
		}
	}
	if raw, err := json.Marshal(props); err == nil && len(raw) > limits.MessageBytes {
		v.add(key, fmt.Sprintf("the message is over message_bytes (%d bytes)", limits.MessageBytes))
	}
	if mt.When == nil {
		return
	}
	// A when: on today() would make the file load on one day and fail on
	// the next, so it is refused rather than evaluated at an arbitrary date.
	if slices.Contains(mt.When.Functions(), predicatefns.FuncToday) {
		v.add(key, "the when: condition of message type "+mt.Name+" calls today(), so whether this "+
			"trigger's message meets it would depend on the date")
		return
	}
	fields := make(map[string]predicate.Value, len(mt.Properties))
	for name := range mt.Properties {
		fields[name] = predicate.NewNil()
		if val, ok := t.Props[name]; ok {
			fields[name] = predicate.NewString(val)
		}
	}
	if !mt.Holds(context.Background(), fields, time.Time{}) {
		v.add(key, "the message does not meet the when: condition of message type "+mt.Name)
	}
}

// facesUnsafe reports whether a rule over entityType that acts on a
// family-anchored channel would read a face some channel reader may not
// see: the type declares faces and the policy admits readers through other
// faces (see facesRestricted).
func (v *validator) facesUnsafe(entityType string, c *Compiled, channel string) bool {
	ct, ok := c.channelTypes[channel]
	return ok && ct.Anchor == AnchorFamily && v.facedAndRestricted(entityType)
}

// facedAndRestricted reports whether entityType declares faces and
// acl.yaml restricts which of them some readers see.
func (v *validator) facedAndRestricted(entityType string) bool {
	def, ok := v.meta.GetEntityDef(entityType)
	return ok && len(def.Faces) > 0 && facesRestricted(v.policy, entityType)
}

// triggerProps checks a trigger's literal property values against the
// message type. A value is operator text: there is no substitution, so a
// message never carries a value read from the entity.
func (v *validator) triggerProps(key string, props map[string]string, mt *rawMessageType, t *Trigger) {
	for _, pn := range sortedKeys(props) {
		val := props[pn]
		pk := key + "." + pn
		rp, ok := mt.Properties[pn]
		switch {
		case !ok:
			v.add(pk, fmt.Sprintf("the message type has no property %q", pn))
		case strings.Contains(val, "{{"):
			v.add(pk, "a trigger posts literal text; {{...}} substitution is not supported")
		case rp.Type == string(PropertyString) || rp.Type == string(PropertyMarkdown):
		default:
			if ct, isEnum := v.meta.Types[rp.Type]; isEnum && slices.Contains(ct.Values, val) {
				break
			}
			v.add(pk, fmt.Sprintf("a trigger can set only string, markdown or enum properties to a literal "+
				"value; %q is not valid for type %s", val, rp.Type))
		}
		t.Props[pn] = val
	}
	for _, pn := range sortedKeys(mt.Properties) {
		if _, set := props[pn]; mt.Properties[pn].Required && !set {
			v.add(key, fmt.Sprintf("required property %q is not set", pn))
		}
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// facesRestricted reports whether p may admit some reader to entityType
// through a face other than the default world's prime face: a role reads
// it through a face-named grant (type@face) or reads another world. A
// value derived from the prime face is then not one every reader of the
// family sees. Like [acl.Policy.FieldRestricted] the answer is policy-wide
// and principal-independent.
//
// Nil: p is accepted and restricts nothing (no acl.yaml).
func facesRestricted(p *acl.Policy, entityType string) bool {
	if p == nil {
		return false
	}
	for _, role := range p.Roles {
		if len(role.Worlds) > 0 {
			return true
		}
		for _, key := range role.Read {
			if strings.HasPrefix(key, entityType+"@") {
				return true
			}
		}
	}
	return false
}
