package realtimecfg

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Compiled is a validated realtime.yaml. It is never mutated after [Parse]
// returns it, so one value may be shared by every reader. Its fields are
// unexported and every accessor returns a copy, so a caller cannot change
// what another reader sees. A copy costs a few small maps; take it once per
// operation.
//
// Nil: accepted by every method — a nil *Compiled is the project without a
// realtime.yaml, where realtime is off.
type Compiled struct {
	limits       Limits
	channelTypes map[string]*ChannelType
	triggers     map[string]*Trigger
	permissions  []string
	warnings     []Problem
	// waiting is the inbox index; see [Compiled.Waiting].
	waiting WaitingIndex
	// mentionUserType is the person entity type; see
	// [Compiled.MentionUserType].
	mentionUserType string
}

// MentionUserType is the entity type a mention names a person by: the
// ACL's user_entity_type when acl.yaml also sets principal_property, so
// the web app stamps a person entity id as the principal; else empty, and
// a person is mentioned by login. It is the one predicate that picks the
// mention form. A nil Compiled has none.
func (c *Compiled) MentionUserType() string {
	if c == nil {
		return ""
	}
	return c.mentionUserType
}

// Limits are the effective caps: the built-in ones from [realtimelimits],
// lowered where realtime.yaml says so.
type Limits struct {
	LogRate           realtimelimits.Rate
	EphemeralRate     realtimelimits.Rate
	MessageBytes      int
	ChannelsPerEntity int
	PageLimit         int
}

// Limits returns the effective caps. A nil Compiled has the built-in ones.
func (c *Compiled) Limits() Limits {
	if c == nil {
		return defaultLimits()
	}
	return c.limits
}

// ChannelType returns a copy of the channel type called name.
func (c *Compiled) ChannelType(name string) (ChannelType, bool) {
	if c == nil {
		return ChannelType{}, false
	}
	ct, ok := c.channelTypes[name]
	if !ok {
		return ChannelType{}, false
	}
	return ct.clone(), true
}

// ChannelTypes returns a copy of every channel type, sorted by name.
func (c *Compiled) ChannelTypes() []ChannelType {
	if c == nil {
		return nil
	}
	out := make([]ChannelType, 0, len(c.channelTypes))
	for _, ct := range sortedValues(c.channelTypes) {
		out = append(out, ct.clone())
	}
	return out
}

// Warnings returns the parts of realtime.yaml that loaded but were left
// out, such as a trigger the policy makes unsafe, each with its reason. The
// rest of the file is in force; the caller logs them.
func (c *Compiled) Warnings() []Problem {
	if c == nil {
		return nil
	}
	return slices.Clone(c.warnings)
}

// Triggers returns a copy of every trigger, sorted by name.
func (c *Compiled) Triggers() []Trigger {
	if c == nil {
		return nil
	}
	out := make([]Trigger, 0, len(c.triggers))
	for _, t := range sortedValues(c.triggers) {
		out = append(out, t.clone())
	}
	return out
}

// UsedPermissions returns every permission realtime.yaml names, sorted and
// without duplicates. It satisfies the dead-permission check's consumer
// interface in `rela acl audit`.
func (c *Compiled) UsedPermissions() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.permissions)
}

func sortedValues[V any](m map[string]*V) []*V {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*V, 0, len(names))
	for _, name := range names {
		out = append(out, m[name])
	}
	return out
}

// Anchor says what a channel is attached to.
type Anchor string

const (
	// AnchorFamily attaches one channel to an entity id across its faces.
	// It is gated on the face the reader can see.
	AnchorFamily Anchor = "family"
	// AnchorFace gives each face of an entity its own channel.
	AnchorFace Anchor = "face"
)

// Cardinality says how many channels of a type one entity has.
type Cardinality string

const (
	// CardinalityOne is one implicit channel per entity, created on first
	// write.
	CardinalityOne Cardinality = "one"
	// CardinalityMany lets the create roles open named channels.
	CardinalityMany Cardinality = "many"
)

// OnEntityDelete says what happens to an entity's channels when the entity
// is deleted.
type OnEntityDelete string

const (
	// OnDeleteDetach keeps the channels, detached from the deleted entity.
	OnDeleteDetach OnEntityDelete = "detach"
	// OnDeleteDelete deletes the channels with the entity.
	OnDeleteDelete OnEntityDelete = "delete"
	// OnDeleteBlock refuses to delete an entity that has channels.
	OnDeleteBlock OnEntityDelete = "block"
)

// State is a channel's lifecycle state.
type State string

// Channel states.
const (
	StateOpen     State = "open"
	StateClosed   State = "closed"
	StateArchived State = "archived"
)

// ChannelType is one entry of `channel_types:`.
type ChannelType struct {
	Name     string
	Label    string
	AttachTo []string
	Anchor   Anchor
	// Cardinality decides whether Create is set: only a "many" type has
	// create roles.
	Cardinality Cardinality
	Create      *Gate
	// Read decides who receives the channel. Receiving a message type
	// needs nothing more than this. Its RequireAnchorRead is always set:
	// whatever its source, Read admits only a principal who can also read
	// the anchor entity.
	Read           Role
	Roles          map[string]Role
	MessageTypes   map[string]*MessageType
	Lifecycle      Lifecycle
	MessageOps     MessageOps
	OnEntityDelete OnEntityDelete
	// DetachedRetention is how long a detached channel of this type is
	// kept before the sweep erases it: after a detach or block delete,
	// and after the sweep detaches an orphan. It is never zero; the
	// default is [realtimelimits.DetachedRetention]. A channel under
	// lifecycle.delete.retention is kept until that ends too.
	DetachedRetention Period
	// Typing is `typing:`: the channel offers the built-in typing
	// indicator. Such a type declares at least one log message type and no
	// message type named [TypingName].
	Typing bool
	// session is the compiled `session:`, or nil. Copies of the type share
	// it; it is immutable (see [ChannelType.Session]).
	session *Session
}

// DefaultDetachedRetention is the [ChannelType.DetachedRetention] of a type
// that does not set `detached_retention:`, and of a type the config no
// longer declares.
func DefaultDetachedRetention() Period {
	return Period{fixed: realtimelimits.DetachedRetention, text: "30d"}
}

// Role is a named set of principals, defined by exactly one source.
//
// When RequireAnchorRead is set, the role admits a principal only if the
// principal matches Source AND can read the anchor entity (the
// [SourceAttached] check). The channel's Read role always has it set, so no
// source lets a principal who cannot read the entity receive its channel:
// a role sourced from a permission, an entity property, a principal type or
// the channel's creator would otherwise reveal the entity's channel to
// someone the ACL hides the entity from.
type Role struct {
	Name              string
	Label             string
	Source            Source
	RequireAnchorRead bool
}

// SourceKind is the closed set of ways a role can match a principal. There
// is no deny source: a role only ever admits.
type SourceKind int

// Role sources.
const (
	// SourcePermission matches a principal holding Permission on the anchor
	// entity (acl.Request.HoldsPermissionForEntity).
	SourcePermission SourceKind = iota + 1
	// SourceAttached matches a principal who can read the anchor entity.
	SourceAttached
	// SourceEntityProperty matches a principal named by the anchor's
	// Property.
	SourceEntityProperty
	// SourcePrincipalType matches the principal's verified principal_type.
	SourcePrincipalType
	// SourceCreator matches the principal who created the channel.
	SourceCreator
)

// Source is the compiled form of a role's `from:`, `permission:`,
// `principal_type:` or `creator:` key.
type Source struct {
	Kind          SourceKind
	Permission    string
	Property      string
	PrincipalType string
}

// Capability is the base capability an operation needs on the anchor
// entity under the request's client ceiling.
type Capability int

// Base capabilities.
const (
	// CapabilityRead is what reading a channel needs: read of the anchor.
	CapabilityRead Capability = iota + 1
	// CapabilityUpdate is what every other operation needs (create, post,
	// edit, delete, remove, lifecycle and actions): update of the anchor's
	// type.
	CapabilityUpdate
)

// NeedsCeilingCheck reports whether the caller must also check that the
// request's client ceiling (client_baselines and scope_grants in acl.yaml)
// admits capability on the anchor before the source admits a principal.
//
// Only an attached source on a read needs no extra check: "can read the
// anchor" is decided by acl.Request, which applies the ceiling's read axis.
// Every other case needs it:
//   - entity property, principal type and creator sources match on identity
//     alone and consult no ACL at all;
//   - a permission source is decided by acl.Request, but the ceiling narrows
//     a permission only through its permission axis (permissions,
//     deny_permissions), never through read, update or deny_write, so a
//     client baseline with deny_write would not stop it;
//   - an attached source on a write proves read, not update.
func (s Source) NeedsCeilingCheck(capability Capability) bool {
	return s.Kind != SourceAttached || capability != CapabilityRead
}

// Gate is who may perform an operation: any listed role, or a holder of
// Permission. Denied is the operator's message for a refusal.
//
// A gate is a write: every way through it also needs the request's client
// ceiling to admit [CapabilityUpdate] on the anchor (Permission is narrowed
// by the ceiling's permission axis only, like a [SourcePermission] role).
//
// When RequireAnchorRead is set, a principal must also be able to read the
// anchor entity, whichever role or permission admits it. A lifecycle
// operation with a `condition:` sets it: the allow-or-deny answer is one bit
// about the entity, which a principal who cannot read it must not learn.
type Gate struct {
	Label             string
	Roles             []string
	Permission        string
	Denied            string
	RequireAnchorRead bool
}

// Rule is a compiled program over the anchor entity, with the properties it
// reads. Triggers and auto rules evaluate it; Fields lists every property
// the program depends on, because the program may read nothing else.
type Rule struct {
	Program *predicate.Program
	Fields  []string
	// Source is the rule as realtime.yaml spells it. The trigger engine
	// keys what it has seen by it, so an edited rule starts over.
	Source string
}

// Lifecycle holds the declared lifecycle operations. A nil operation is not
// offered.
type Lifecycle struct {
	Close   *LifecycleOp
	Reopen  *LifecycleOp
	Archive *LifecycleOp
	Delete  *LifecycleOp
}

// Lifecycle operation names, as `lifecycle:` keys them and the HTTP route
// names them.
const (
	LifecycleClose   = "close"
	LifecycleReopen  = "reopen"
	LifecycleArchive = "archive"
	LifecycleDelete  = "delete"
)

// Message operation names. A message's `_actions` lists them next to the
// names of its actions, so no action may take one (see [MessageOpNames]).
const (
	// MessageOpEdit edits the reader's own message (edit_own).
	MessageOpEdit = "edit"
	// MessageOpDelete deletes the reader's own message without a reason
	// (delete_own).
	MessageOpDelete = "delete"
	// MessageOpRemove removes any message with a reason (remove).
	MessageOpRemove = "remove"
	// MessageOpRelease releases a message's pending action.
	MessageOpRelease = "release"
)

// MessageOpNames returns every message operation name, sorted. They are
// reserved: an action may not take one.
func MessageOpNames() []string {
	return []string{MessageOpDelete, MessageOpEdit, MessageOpRelease, MessageOpRemove}
}

// LifecycleOpNames returns every lifecycle operation name, in the order a
// listing of operations uses.
func LifecycleOpNames() []string {
	return []string{LifecycleClose, LifecycleReopen, LifecycleArchive, LifecycleDelete}
}

// Op returns the operation named name.
//
// Nil: returned for an operation the channel type does not declare and for
// a name that is no lifecycle operation.
func (l Lifecycle) Op(name string) *LifecycleOp {
	switch name {
	case LifecycleClose:
		return l.Close
	case LifecycleReopen:
		return l.Reopen
	case LifecycleArchive:
		return l.Archive
	case LifecycleDelete:
		return l.Delete
	}
	return nil
}

// LifecycleOp is one lifecycle operation.
type LifecycleOp struct {
	Gate
	// Conditions is the compiled `condition:`, one program per attach_to
	// type. Empty when the operation has no condition.
	Conditions map[string]*predicate.Program
	// From is the state the operation starts from; empty means any state
	// the operation applies to.
	From      State
	Auto      *Auto
	Retention Period
}

// Auto runs a lifecycle operation without a request: when When holds for
// the anchor entity, or After has passed since the previous state change.
type Auto struct {
	// When holds one rule per attach_to type; empty when only After is set.
	When  map[string]Rule
	After Period
}

// MessageOps holds the declared message operations. A nil operation is not
// offered.
type MessageOps struct {
	EditOwn   *EditOwn
	DeleteOwn *DeleteOwn
	Remove    *Remove
}

// EditOwn lets an author edit their own message, within Within of posting
// when set.
type EditOwn struct {
	Gate
	Within Period
}

// DeleteOwn lets an author delete their own message while the channel is in
// From, when set.
type DeleteOwn struct {
	Gate
	From State
}

// Remove lets a moderator remove any message.
type Remove struct {
	Gate
	ReasonRequired bool
}

// Persist is where a message type's messages are kept.
type Persist string

const (
	// PersistLog appends messages to the channel's log.
	PersistLog Persist = "log"
	// PersistEphemeral keeps signals in memory only, for TTL.
	PersistEphemeral Persist = "ephemeral"
)

// MessageType is one entry of a channel type's `message_types:`.
type MessageType struct {
	Name       string
	Label      string
	Persist    Persist
	TTL        time.Duration
	Rate       realtimelimits.Rate
	Properties map[string]Property
	// When is the compiled `when:` over the `message` record, or nil.
	When *predicate.Program
	Post []string
	// Actions are the buttons on a message. Promoting a message to an
	// entity is always an action, so its gate is explicit.
	Actions map[string]*Action
	// In is the session states in which the type may be posted: the states
	// as listed in `in:`, or, when derived from session.default_in, in the
	// order of session.states. Nil means every state; see
	// [MessageType.PostableIn] and [ChannelType.InKey].
	In []string
	// Choices is `choices:`, or nil.
	Choices *Choices
	// DecisionButton is `decision_button:`: the options of the type's
	// decision buttons in the web app, or nil when the file sets none.
	// The load already merged them into each preset button of the type
	// (see [TransitionButton.Options]), so they matter on their own only
	// for the type's button.
	DecisionButton *ButtonOptions
	// derived reports that In was derived from session.default_in; set
	// only by the loader.
	derived bool
}

// Choices is `choices:` on a message type, the question type. The web app
// draws one button per item of the question's From list. A click posts a
// message of type Answer, with the item in its Into property, as a reply
// to the question. Other adds a button that opens the composer with
// Answer selected, for a free-text answer.
//
// The load checks that From is a list property of the question type, that
// Answer is another log type of the same channel type, and that Into is a
// string property of Answer that fits every item. The server does not
// check that an answer is one of the options: Other posts free text into
// the same property.
type Choices struct {
	From   string
	Answer string
	Into   string
	Other  bool
}

// InKey is the key path of the rule that decided mt.In: session.default_in
// when the load derived it, else the type's `in:`. A MessageType built by
// hand gets the `in:` path.
func (ct *ChannelType) InKey(mt *MessageType) string {
	if mt.derived {
		return "channel_types." + ct.Name + ".session.default_in"
	}
	return ct.messageTypeKey(mt.Name) + ".in"
}

// PostableIn reports whether a message of type mt may be posted while the
// session is in state: true when mt has no `in:` or lists state.
func (mt *MessageType) PostableIn(state string) bool {
	return mt.In == nil || slices.Contains(mt.In, state)
}

// Holds reports whether a message whose `message` record has fields meets
// mt's `when:`, evaluated at now. It holds without a `when:`, and fails
// closed when the condition does not evaluate to a boolean. The channel
// service and the load check of a trigger's literal props both call it, so
// the two cannot drift apart.
func (mt *MessageType) Holds(ctx context.Context, fields map[string]predicate.Value, now time.Time) bool {
	if mt.When == nil {
		return true
	}
	b := predicate.NewBindings()
	if err := b.SetVar("message", predicate.NewRecord(fields)); err != nil {
		return false
	}
	if err := predicatefns.Bind(b, now); err != nil {
		return false
	}
	v, err := mt.When.Eval(ctx, b)
	if err != nil {
		return false
	}
	got, ok := v.(predicate.Bool)
	return ok && got.Bool()
}

// PropertyType is the type of a message property.
type PropertyType string

// Message property types. PropertyEnum takes its values from an enum type
// declared in schema.yaml.
const (
	PropertyString   PropertyType = "string"
	PropertyMarkdown PropertyType = "markdown"
	PropertyInteger  PropertyType = "integer"
	PropertyBoolean  PropertyType = "boolean"
	PropertyDate     PropertyType = "date"
	PropertyDatetime PropertyType = "datetime"
	PropertyRef      PropertyType = "ref"
	PropertyEnum     PropertyType = "enum"
)

// Property is one message property.
type Property struct {
	Name     string
	Label    string
	Type     PropertyType
	Enum     string
	Values   []string
	Required bool
	Max      int
	To       []string
	// NoNotify is set by `notify: false` on a ref property: a value that
	// names a person entity is then no mention target.
	NoNotify bool
	// List is `list: true`: the value is a list of strings. Only a string
	// property may be a list. Max is then the most code points of one
	// item, and is always set.
	List bool
	// MaxItems is the most items of a list; 0 when List is not set.
	MaxItems int
}

// TypeName is the property's type as a reason names it: "list of string"
// for a list, else the type.
func (p Property) TypeName() string {
	if p.List {
		return "list of " + string(p.Type)
	}
	return string(p.Type)
}

// Promote turns a message into an entity: Map names, per entity property,
// the message property it is copied from, and Relate, when set, is the
// relation type linking the new entity to the anchor. Face is the face the
// new entity is created at; it is set exactly when Type declares faces, so
// a promote never picks a face by rank.
type Promote struct {
	Type   string
	Face   string
	Map    map[string]string
	Relate string
}

// Action is a button on a message.
type Action struct {
	Name string
	Gate
	PromoteTo *Promote
}

// TriggerOn is the change a trigger reacts to.
type TriggerOn string

// Trigger events.
const (
	TriggerEnter  TriggerOn = "enter"
	TriggerLeave  TriggerOn = "leave"
	TriggerChange TriggerOn = "change"
)

// WhenClosed is what a trigger does when its channel is closed or
// archived.
type WhenClosed string

// Trigger behaviors for a channel that is not open.
const (
	// WhenClosedSkip posts nothing and records the change as seen.
	WhenClosedSkip WhenClosed = "skip"
	// WhenClosedPost posts all the same.
	WhenClosedPost WhenClosed = "post"
)

// Trigger posts a message into an entity's channel when the entity starts
// matching (enter), stops matching (leave), or changes a property its rule
// reads while matching (change).
type Trigger struct {
	Name       string
	EntityType string
	On         TriggerOn
	When       Rule
	Channel    string
	PostType   string
	WhenClosed WhenClosed
	// Props are literal values: a trigger posts operator text and a
	// reference to the entity, never a value read from it.
	Props map[string]string
}

func defaultLimits() Limits {
	return Limits{
		LogRate:           realtimelimits.DefaultLogRate,
		EphemeralRate:     realtimelimits.DefaultEphemeralRate,
		MessageBytes:      realtimelimits.MaxMessageBytes,
		ChannelsPerEntity: realtimelimits.MaxChannelsPerAnchor,
		PageLimit:         realtimelimits.MaxPageLimit,
	}
}
