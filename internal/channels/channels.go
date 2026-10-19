// Package channels holds the realtime channels attached to entities: ordered
// logs of typed messages that live beside the graph rather than in it.
//
// # Why this is not in the graph, and not comments
//
// A channel message is conversation about an entity, not a fact in the
// operator's domain model, so it stays out of the store, the audit log, the
// version sweep and the search index, for the reason the comments package
// gives. It does not reuse the comments backends either: the comments file
// backend keeps a whole thread in one document capped at 500 comments, and a
// chat log needs ordered paging over an unbounded history.
//
// # What this package holds
//
// The value types, the sentinel errors, the store-boundary caps, key
// validation and the [Store] contract that every backend implements:
// filechannels (fs, memory, desktop), sqlitechannels and pgchannels. The
// service that authorizes and delivers messages is built on top of [Store]
// and is not part of this package's storage layer.
//
// # The contract every backend keeps
//
// channelstest.RunAll pins it once for all backends:
//
//   - seq is gapless and strictly increasing per channel, even under
//     concurrent appends from several processes.
//   - Every mutation bumps the channel's rev and stamps it on each message
//     it changed, so a reader paging by rev sees edits, removals, reply
//     counts and action changes, not only new messages.
//   - Removing a message erases its content from storage: only the seq, the
//     thread link and the removal record remain. Editing replaces the
//     properties and keeps no history.
//   - The per-entity audit trail never holds message content, and outlives
//     the channels it describes.
//   - A rename re-keys every channel of the entity atomically. A tombstone
//     moves an entity's channels out of reach without deleting them, so an
//     entity that later reuses the id does not inherit them.
package channels

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Sentinel errors. Backends wrap them, so callers test with errors.Is.
var (
	// ErrNotFound reports a channel or message that does not exist. A
	// cardinality-one channel that was never written to is not found: it
	// exists implicitly, and the caller decides what an empty one looks like.
	ErrNotFound = errors.New("channels: not found")

	// ErrInvalidKey reports a malformed key: a bad entity id, face, type or
	// name.
	ErrInvalidKey = errors.New("channels: invalid key")

	// ErrInvalidMessage reports a message that breaks a store-boundary rule
	// other than the size caps: a bad type name, a missing author or time.
	ErrInvalidMessage = errors.New("channels: invalid message")

	// ErrTooLarge reports a value over one of the caps below.
	ErrTooLarge = errors.New("channels: value too large")

	// ErrTooManyChannels reports an anchor that already holds as many
	// channels of the type as allowed: [NewChannel.Max], at most
	// [MaxChannelsPerType].
	ErrTooManyChannels = errors.New("channels: too many channels")

	// ErrTypeShapeConflict reports a type record that differs from the one
	// the store holds for the type: another anchor mode or cardinality, or
	// a property the store recorded as a ref that the new record declares
	// as something else. Nothing was recorded. See [TypeRecord].
	ErrTypeShapeConflict = errors.New("channels: channel type is recorded with another shape")

	// ErrConflict reports a compare-and-set that lost: the lifecycle state
	// or the action status was not the expected one.
	ErrConflict = errors.New("channels: state changed concurrently")

	// ErrRemoved reports an operation on a message that was removed.
	ErrRemoved = errors.New("channels: message was removed")

	// ErrInvalidReply reports a reply to a message that does not exist, was
	// removed, or is itself a reply. Threads are one level deep.
	ErrInvalidReply = errors.New("channels: invalid reply target")

	// ErrInvalidPage reports a page request that names more than one cursor.
	ErrInvalidPage = errors.New("channels: invalid page request")

	// ErrRekeyConflict reports a rename whose target id already holds
	// channels. Nothing moved; the caller tombstones the occupant and retries.
	ErrRekeyConflict = errors.New("channels: target id already has channels")

	// ErrStaleAnchor reports an append to a channel whose recorded
	// incarnation is not the one the caller gave: the channel belongs to an
	// earlier entity that used the same id. Nothing was written; the caller
	// tombstones the leftover and retries. See [Channel.Incarnation].
	ErrStaleAnchor = errors.New("channels: channel belongs to an earlier entity with this id")

	// ErrTypeInUse reports a type record that cannot be dropped because a
	// channel of the type still exists, live or tombstoned.
	ErrTypeInUse = errors.New("channels: channel type still has channels")
)

// Caps enforced at the store boundary, so no backend and no caller above it
// can exceed them.
const (
	// MaxPropsBytes caps the JSON encoding of one message's properties.
	MaxPropsBytes = 16 * 1024
	// MaxProps caps the number of top-level properties of one message.
	MaxProps = 32
	// MaxTitleRunes caps a channel title.
	MaxTitleRunes = 200
	// MaxReasonRunes caps a removal reason.
	MaxReasonRunes = 500
	// MaxChannelsPerType caps the named channels one anchor holds of one
	// type, counted over every lifecycle state.
	MaxChannelsPerType = 200
	// MaxPageLimit caps one page of messages. A larger limit is clamped.
	MaxPageLimit = 200
	// DefaultPageLimit is the page size when the request names none.
	DefaultPageLimit = 50
	// MaxTextBytes caps the free-text fields that are not content: the
	// schema hash, the triggered-by label and the incarnation.
	MaxTextBytes = 200
	// MaxAnchorIDBytes and MaxFaceBytes cap the anchor of a channel. The
	// file backend writes both into one directory name of at most 255
	// bytes, base32-encoded (8 characters per 5 bytes) and followed by a
	// tombstone stamp, so every backend refuses a longer anchor alike.
	MaxAnchorIDBytes = 100
	MaxFaceBytes     = 32
	// MaxPromotedTypeBytes caps the entity type an action names.
	MaxPromotedTypeBytes = 64
)

// namePattern is the grammar of channel type, message type and action
// names. It is shared with realtime.yaml, so a name the config accepts is a
// name the store accepts.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// channelNamePattern is the grammar of a named channel's name. Names are
// minted by [NewName], never chosen by a user; the title carries the label.
var channelNamePattern = regexp.MustCompile(`^[0-9a-z]{1,32}$`)

// windowsReserved are device names Windows refuses as a path segment. A
// type is a path segment in the file backend, so every backend refuses
// them alike.
var windowsReserved = map[string]bool{
	"con": true, "nul": true, "prn": true, "aux": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidName reports whether s is a valid channel type, message type or
// action name.
func ValidName(s string) bool { return namePattern.MatchString(s) && !windowsReserved[s] }

// State is a channel's lifecycle state.
type State string

// Lifecycle states. A channel starts open.
const (
	StateOpen     State = "open"
	StateClosed   State = "closed"
	StateArchived State = "archived"
)

// Valid reports whether s is one of the lifecycle states.
func (s State) Valid() bool {
	return s == StateOpen || s == StateClosed || s == StateArchived
}

// Anchor is the entity a channel hangs off. Face is empty for a channel
// anchored to the whole entity family, and names the face for a channel
// type declared `anchor: face`.
type Anchor struct {
	ID   string      `json:"id"`
	Face entity.Face `json:"face,omitempty"`
}

// Validate checks the id and face grammar and length.
func (a Anchor) Validate() error {
	if err := entity.ValidateID(a.ID); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	if len(a.ID) > MaxAnchorIDBytes || len(a.Face) > MaxFaceBytes {
		return fmt.Errorf("%w: anchor id over %d bytes or face over %d bytes",
			ErrInvalidKey, MaxAnchorIDBytes, MaxFaceBytes)
	}
	if a.Face != "" {
		if _, err := entity.ParseFace(string(a.Face)); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidKey, err)
		}
	}
	return nil
}

// Key identifies one channel: its anchor, its type and, for a type with
// cardinality many, its name. Name is empty for a cardinality-one channel.
type Key struct {
	Anchor
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Validate checks every part of the key.
func (k Key) Validate() error {
	if err := k.Anchor.Validate(); err != nil {
		return err
	}
	if !ValidName(k.Type) {
		return fmt.Errorf("%w: type %q", ErrInvalidKey, k.Type)
	}
	if k.Name != "" && !channelNamePattern.MatchString(k.Name) {
		return fmt.Errorf("%w: name %q", ErrInvalidKey, k.Name)
	}
	return nil
}

// String renders the key as `id[@face]/type[/name]`, for logs and for
// triggered-by labels.
func (k Key) String() string {
	var b strings.Builder
	b.WriteString(entity.FormatStateRef(k.ID, k.Face))
	b.WriteString("/")
	b.WriteString(k.Type)
	if k.Name != "" {
		b.WriteString("/")
		b.WriteString(k.Name)
	}
	return b.String()
}

// Author records who wrote a message or acted on one. It is for display and
// ownership checks only; the principal type grants nothing. OrgID is the
// organization of a verified principal, so an ownership check compares the
// user and the organization: two organizations may share a user name.
type Author struct {
	User          string `json:"user"                     yaml:"user"`
	Tool          string `json:"tool,omitempty"           yaml:"tool,omitempty"`
	PrincipalType string `json:"principal_type,omitempty" yaml:"principal_type,omitempty"`
	OrgID         string `json:"org_id,omitempty"         yaml:"org_id,omitempty"`
}

// IsZero reports whether no user is recorded.
func (a Author) IsZero() bool { return a.User == "" }

// Removal records who removed a message, when and why. It replaces the
// message's content. Kind says whether the author withdrew the message or a
// moderator removed it. The author is erased with the content, so Kind is
// the only record of which it was.
type Removal struct {
	By     Author      `json:"by"`
	At     time.Time   `json:"at"`
	Reason string      `json:"reason,omitempty"`
	Kind   RemovalKind `json:"kind,omitempty"`
}

// RemovalKind says who removed a message.
type RemovalKind string

// Removal kinds. The empty kind reads as a moderator's removal.
const (
	RemovalModerator RemovalKind = "moderator"
	RemovalAuthor    RemovalKind = "author"
)

// ActionStatus is where a message action stands.
type ActionStatus string

// Action statuses. The zero value means no action was taken.
const (
	ActionNone    ActionStatus = ""
	ActionPending ActionStatus = "pending"
	ActionDone    ActionStatus = "done"
)

// Promoted names the entity an action created from a message.
type Promoted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Action is the one action taken on a message. Pending marks an action whose
// side effect is under way; Done marks one that completed.
type Action struct {
	Name      string       `json:"name"`
	Status    ActionStatus `json:"status"`
	By        Author       `json:"by"`
	At        time.Time    `json:"at"`
	PendingAt time.Time    `json:"pending_at,omitzero"`
	Promoted  *Promoted    `json:"promoted,omitempty"`
}

// Message is one stored message. A removed message keeps only Seq, Rev,
// ReplyTo, ReplyCount and Removed.
type Message struct {
	Seq         int64          `json:"seq"`
	Rev         int64          `json:"rev"`
	Type        string         `json:"type,omitempty"`
	Author      Author         `json:"author,omitzero"`
	At          time.Time      `json:"at,omitzero"`
	Props       map[string]any `json:"props,omitempty"`
	ReplyTo     int64          `json:"reply_to,omitempty"`
	ReplyCount  int            `json:"reply_count,omitempty"`
	EditedAt    time.Time      `json:"edited_at,omitzero"`
	Removed     *Removal       `json:"removed,omitempty"`
	Action      *Action        `json:"action,omitempty"`
	SchemaHash  string         `json:"schema_hash,omitempty"`
	TriggeredBy string         `json:"triggered_by,omitempty"`
}

// ActionStatus returns the status of the message's action, or [ActionNone].
func (m Message) ActionStatus() ActionStatus {
	if m.Action == nil {
		return ActionNone
	}
	return m.Action.Status
}

// NewMessage is what a caller appends. The store assigns Seq and Rev.
//
// Incarnation identifies the anchor's lifetime (see [Channel.Incarnation]).
// The append records it when it creates an implicit channel, and refuses
// with [ErrStaleAnchor] when the channel exists with another one.
type NewMessage struct {
	Incarnation string
	Type        string
	Author      Author
	At          time.Time
	Props       map[string]any
	ReplyTo     int64
	SchemaHash  string
	TriggeredBy string
}

// Channel is a channel's metadata. LastSeq and Rev are the head a poller
// compares against.
//
// Incarnation is an opaque token, chosen by the caller, that identifies the
// lifetime of the anchor the channel was created on. It is recorded once,
// when the channel is created, and survives a rekey, because a rename keeps
// the entity. Its job is to fail closed on id reuse. The hooks that
// tombstone a deleted entity's channels run after the commit and can fail;
// a channel they missed still carries the old incarnation, so a caller that
// compares it with the live entity's (see [CheckIncarnation]) treats the
// channel as nonexistent rather than handing it to the new entity.
//
// The same check covers two creates racing on one id. When two faces of a
// new entity are created at once, each create hook sees two faces and
// tombstones only its own face, so channels anchored to the whole family of
// the deleted entity survive both hooks. They carry the deleted entity's
// incarnation and so stay unreachable.
//
// The token must be stable for the anchor's life and differ between two
// lives of one id: for a family anchor, an identity of the family; for a
// face anchor, one of that face. The store only compares it for equality.
type Channel struct {
	Key         Key       `json:"key"`
	Incarnation string    `json:"incarnation,omitempty"`
	State       State     `json:"state"`
	Title       string    `json:"title,omitempty"`
	CreatedBy   Author    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	ClosedAt    time.Time `json:"closed_at,omitzero"`
	ArchivedAt  time.Time `json:"archived_at,omitzero"`
	// ReopenedAt is when the channel was last reopened; zero when it was
	// never reopened.
	ReopenedAt time.Time `json:"reopened_at,omitzero"`
	LastSeq    int64     `json:"last_seq"`
	Rev        int64     `json:"rev"`
}

// CheckIncarnation reports a channel whose incarnation is not live, the
// incarnation of the anchor as it exists now, as [ErrNotFound]: it was left
// by an earlier entity with the same id. A caller runs it on every channel
// it reads before serving it.
func CheckIncarnation(c Channel, live string) error {
	if c.Incarnation != live {
		return fmt.Errorf("%w: channel %s belongs to an earlier entity", ErrNotFound, c.Key)
	}
	return nil
}

// CheckAppendIncarnation refuses an append to an existing channel whose
// recorded incarnation is not the caller's, with [ErrStaleAnchor].
func CheckAppendIncarnation(k Key, recorded, given string) error {
	if recorded != given {
		return fmt.Errorf("%w: %s", ErrStaleAnchor, k)
	}
	return nil
}

// NewChannel is what CreateMany creates: a named channel of Type with an
// optional Title, recorded with the anchor's Incarnation.
type NewChannel struct {
	Type        string
	Title       string
	Incarnation string
	By          Author
	At          time.Time
	// Max caps the named channels of Type the anchor may hold, counting
	// this one. Zero, or a value over [MaxChannelsPerType], means
	// [MaxChannelsPerType].
	Max int
}

// Cap is the most named channels of the type the anchor may hold: Max,
// bounded by [MaxChannelsPerType].
func (nc NewChannel) Cap() int {
	if nc.Max <= 0 || nc.Max > MaxChannelsPerType {
		return MaxChannelsPerType
	}
	return nc.Max
}

// StateSince returns when the channel entered its current state: the close
// or archive time, or for an open channel the reopen time, or the creation
// time when it was never reopened.
func (c Channel) StateSince() time.Time {
	switch c.State {
	case StateClosed:
		return c.ClosedAt
	case StateArchived:
		return c.ArchivedAt
	default:
		if !c.ReopenedAt.IsZero() {
			return c.ReopenedAt
		}
		return c.CreatedAt
	}
}

// Transition is a lifecycle change: the state to move to, who moves it and
// when. TriggeredBy names the rule that made the change when no person
// asked for it, such as an `auto` step; it is empty for a request. The
// store copies it to the audit record.
type Transition struct {
	To          State
	By          Author
	At          time.Time
	TriggeredBy string
}

// ApplyTransition sets c's state and timestamps for t. Reopening clears
// both timestamps and records the reopen time; closing clears the archive
// time. Archiving an open channel closes it in the same instant, so an
// archived channel always has a close time.
func ApplyTransition(c *Channel, t Transition) {
	c.State = t.To
	switch t.To {
	case StateOpen:
		c.ClosedAt, c.ArchivedAt, c.ReopenedAt = time.Time{}, time.Time{}, t.At
	case StateClosed:
		c.ClosedAt, c.ArchivedAt = t.At, time.Time{}
	case StateArchived:
		if c.ClosedAt.IsZero() {
			c.ClosedAt = t.At
		}
		c.ArchivedAt = t.At
	}
}

// Page selects messages. At most one cursor is set:
//
//   - AfterSeq: messages with a higher seq, oldest first.
//   - BeforeSeq: the newest messages with a lower seq, oldest first.
//   - AfterRev: messages changed after that rev, ordered by rev. This is
//     the cursor that sees edits and removals.
//   - none: the newest messages, oldest first.
//
// Limit is clamped to [1, MaxPageLimit]; zero means [DefaultPageLimit].
type Page struct {
	AfterSeq  int64
	BeforeSeq int64
	AfterRev  int64
	Limit     int
}

// Normalize validates p and returns it with the limit clamped.
func (p Page) Normalize() (Page, error) {
	set := 0
	for _, v := range []int64{p.AfterSeq, p.BeforeSeq, p.AfterRev} {
		if v < 0 {
			return p, fmt.Errorf("%w: negative cursor", ErrInvalidPage)
		}
		if v > 0 {
			set++
		}
	}
	if set > 1 {
		return p, fmt.Errorf("%w: name at most one cursor", ErrInvalidPage)
	}
	switch {
	case p.Limit <= 0:
		p.Limit = DefaultPageLimit
	case p.Limit > MaxPageLimit:
		p.Limit = MaxPageLimit
	}
	return p, nil
}

// PendingAction is a message action left pending, as listed for the sweep
// that reverts or completes it.
type PendingAction struct {
	Key       Key       `json:"key"`
	Seq       int64     `json:"seq"`
	Name      string    `json:"name"`
	PendingAt time.Time `json:"pending_at"`
}

// AuditOp names what an audit record records.
type AuditOp string

// Audit operations. Appends and edits are deliberately not audited.
const (
	AuditCreate      AuditOp = "create"
	AuditLifecycle   AuditOp = "lifecycle"
	AuditRemove      AuditOp = "remove"
	AuditAction      AuditOp = "action"
	AuditDelete      AuditOp = "delete"
	AuditRekey       AuditOp = "rekey"
	AuditTombstone   AuditOp = "tombstone"
	AuditUntombstone AuditOp = "untombstone"
	// AuditPurge records that the channels of a tombstoned batch were
	// erased. It is appended to the batch's own filed trail, never to the
	// live trail of the id, which may belong to a new entity by then.
	AuditPurge AuditOp = "purge"
)

// AuditReleased is the [AuditRecord.Status] of an action record that
// records a pending action put back to none.
const AuditReleased = "released"

// AuditRecord is one entry in an entity's channel audit trail. It never
// holds message content: no properties, no removal reason, no title.
//
// Type and Name are empty for the records that concern the whole entity
// (rekey, and a tombstone or untombstone of every face). From and To carry the old and new
// state of a lifecycle change and the old and new id of a rekey. Status is
// set on an action record that is not a completion, such as
// [AuditReleased]. TriggeredBy is copied from [Transition.TriggeredBy].
type AuditRecord struct {
	At       time.Time   `json:"at"`
	Op       AuditOp     `json:"op"`
	Face     entity.Face `json:"face,omitempty"`
	Type     string      `json:"type,omitempty"`
	Name     string      `json:"name,omitempty"`
	Seq      int64       `json:"seq,omitempty"`
	By       Author      `json:"by,omitzero"`
	From     string      `json:"from,omitempty"`
	To       string      `json:"to,omitempty"`
	Action   string      `json:"action,omitempty"`
	Promoted *Promoted   `json:"promoted,omitempty"`
	Status   string      `json:"status,omitempty"`
	// TriggeredBy is set on records of changes no person asked for.
	TriggeredBy string `json:"triggered_by,omitempty"`
	// Tomb is the stamp of the batch a purge record is about. A purge
	// record of a channel also carries the channel's last seq in Seq.
	Tomb string `json:"tomb,omitempty"`
}

// KeyAudit returns the start of an audit record about the channel k.
func KeyAudit(k Key, op AuditOp, at time.Time) AuditRecord {
	return AuditRecord{At: at, Op: op, Face: k.Face, Type: k.Type, Name: k.Name}
}

// NormTime returns t in UTC at microsecond precision, the precision every
// backend stores. Normalizing at the boundary is what makes a time read back
// equal to the time written, on every backend.
func NormTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC().Truncate(time.Microsecond)
}

// PrepareMessage validates m against the store-boundary rules and returns
// it with its time normalized and its properties encoded. The returned
// props map is the decoded encoding, so every backend returns the same
// shapes (numbers as float64) that a later read returns.
func PrepareMessage(m NewMessage) (NewMessage, []byte, error) {
	if !ValidName(m.Type) {
		return m, nil, fmt.Errorf("%w: message type %q", ErrInvalidMessage, m.Type)
	}
	if err := checkAuthor(m.Author); err != nil {
		return m, nil, err
	}
	if m.At.IsZero() {
		return m, nil, fmt.Errorf("%w: missing time", ErrInvalidMessage)
	}
	if m.ReplyTo < 0 {
		return m, nil, fmt.Errorf("%w: negative reply_to", ErrInvalidReply)
	}
	if len(m.SchemaHash) > MaxTextBytes || len(m.TriggeredBy) > MaxTextBytes || len(m.Incarnation) > MaxTextBytes {
		return m, nil, fmt.Errorf("%w: schema hash, triggered_by or incarnation over %d bytes",
			ErrTooLarge, MaxTextBytes)
	}
	props, raw, err := PrepareProps(m.Props)
	if err != nil {
		return m, nil, err
	}
	m.Props = props
	m.At = NormTime(m.At)
	return m, raw, nil
}

// PrepareProps applies the property caps and returns the decoded encoding
// and the encoding itself. Nil props encode as nil.
func PrepareProps(props map[string]any) (decoded map[string]any, raw []byte, err error) {
	if len(props) == 0 {
		return nil, nil, nil
	}
	if len(props) > MaxProps {
		return nil, nil, fmt.Errorf("%w: %d properties, at most %d", ErrTooLarge, len(props), MaxProps)
	}
	raw, err = json.Marshal(props)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: properties do not encode: %w", ErrInvalidMessage, err)
	}
	if len(raw) > MaxPropsBytes {
		return nil, nil, fmt.Errorf("%w: properties are %d bytes, at most %d", ErrTooLarge, len(raw), MaxPropsBytes)
	}
	if err = json.Unmarshal(raw, &decoded); err != nil { // coverage-ignore: invariant: bytes just encoded from a map
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidMessage, err)
	}
	return decoded, raw, nil
}

// PrepareRemoval validates r and returns it with its time normalized.
func PrepareRemoval(r Removal) (Removal, error) {
	if err := checkAuthor(r.By); err != nil {
		return r, err
	}
	if r.At.IsZero() {
		return r, fmt.Errorf("%w: removal has no time", ErrInvalidMessage)
	}
	if utf8.RuneCountInString(r.Reason) > MaxReasonRunes {
		return r, fmt.Errorf("%w: reason over %d characters", ErrTooLarge, MaxReasonRunes)
	}
	switch r.Kind {
	case "", RemovalModerator, RemovalAuthor:
	default:
		return r, fmt.Errorf("%w: removal kind %q", ErrInvalidMessage, r.Kind)
	}
	r.At = NormTime(r.At)
	return r, nil
}

// PrepareAction validates the next action of a SetAction call, and returns
// it with its times normalized. A next action with [ActionNone] clears the
// action: only its By and At are kept, for the audit record of a release
// (see [CheckRelease]).
func PrepareAction(name string, next Action) (Action, error) {
	if !ValidName(name) {
		return next, fmt.Errorf("%w: action name %q", ErrInvalidMessage, name)
	}
	switch next.Status {
	case ActionNone:
		return Action{By: next.By, At: NormTime(next.At)}, nil
	case ActionPending, ActionDone:
	default:
		return next, fmt.Errorf("%w: action status %q", ErrInvalidMessage, next.Status)
	}
	if err := checkAuthor(next.By); err != nil {
		return next, err
	}
	if next.At.IsZero() {
		return next, fmt.Errorf("%w: action has no time", ErrInvalidMessage)
	}
	if next.Promoted != nil {
		if err := entity.ValidateID(next.Promoted.ID); err != nil {
			return next, fmt.Errorf("%w: promoted id: %w", ErrInvalidMessage, err)
		}
		if err := checkPromotedType(next.Promoted.Type); err != nil {
			return next, err
		}
	}
	next.Name = name
	next.At = NormTime(next.At)
	next.PendingAt = NormTime(next.PendingAt)
	if next.Status == ActionPending && next.PendingAt.IsZero() {
		next.PendingAt = next.At
	}
	return next, nil
}

// CheckActionCAS reports whether a message whose current action is cur may
// move to an action named name: the current status must be expect, and a
// current action other than none must carry the same name.
func CheckActionCAS(cur *Action, name string, expect ActionStatus) error {
	status := ActionNone
	if cur != nil {
		status = cur.Status
	}
	if status != expect {
		return fmt.Errorf("%w: action is %q, expected %q", ErrConflict, status, expect)
	}
	if cur != nil && cur.Name != name {
		return fmt.Errorf("%w: message carries action %q, not %q", ErrConflict, cur.Name, name)
	}
	return nil
}

// CheckRelease refuses a SetAction from [ActionPending] to [ActionNone]
// that names no author or time. That step is a release, and its audit
// record says who released the action and when.
func CheckRelease(expect ActionStatus, next Action) error {
	if expect != ActionPending || next.Status != ActionNone {
		return nil
	}
	if err := checkAuthor(next.By); err != nil {
		return fmt.Errorf("release: %w", err)
	}
	if next.At.IsZero() {
		return fmt.Errorf("%w: release has no time", ErrInvalidMessage)
	}
	return nil
}

// ActionAudit returns the audit record a SetAction writes, and whether it
// writes one. A completion ([ActionDone]) and a release (pending to none)
// are recorded; taking an action pending is not.
func ActionAudit(k Key, seq int64, name string, expect ActionStatus, next Action) (AuditRecord, bool) {
	release := next.Status == ActionNone && expect == ActionPending
	if next.Status != ActionDone && !release {
		return AuditRecord{}, false
	}
	rec := KeyAudit(k, AuditAction, next.At)
	rec.Seq, rec.By, rec.Action, rec.Promoted = seq, next.By, name, next.Promoted
	if release {
		rec.Status = AuditReleased
	}
	return rec, true
}

// checkPromotedType refuses an empty, oversized or unprintable entity type
// in an action's promoted record. Entity type names have no grammar beyond
// that, so this is a sanity cap rather than a schema check.
func checkPromotedType(typ string) error {
	if typ == "" || len(typ) > MaxPromotedTypeBytes || !utf8.ValidString(typ) {
		return fmt.Errorf("%w: promoted type must be 1 to %d bytes of UTF-8", ErrInvalidMessage, MaxPromotedTypeBytes)
	}
	for _, r := range typ {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: promoted type holds a control character", ErrInvalidMessage)
		}
	}
	return nil
}

// PrepareChannel validates the inputs of a CreateMany call and returns
// them with the time normalized.
func PrepareChannel(anchor Anchor, nc NewChannel) (NewChannel, error) {
	if err := (Key{Anchor: anchor, Type: nc.Type}).Validate(); err != nil {
		return nc, err
	}
	if utf8.RuneCountInString(nc.Title) > MaxTitleRunes {
		return nc, fmt.Errorf("%w: title over %d characters", ErrTooLarge, MaxTitleRunes)
	}
	if len(nc.Incarnation) > MaxTextBytes {
		return nc, fmt.Errorf("%w: incarnation over %d bytes", ErrTooLarge, MaxTextBytes)
	}
	if err := checkAuthor(nc.By); err != nil {
		return nc, err
	}
	if nc.At.IsZero() {
		return nc, fmt.Errorf("%w: missing time", ErrInvalidMessage)
	}
	nc.At = NormTime(nc.At)
	return nc, nil
}

// PrepareTransition validates a lifecycle change.
func PrepareTransition(expect State, t Transition) (Transition, error) {
	if !expect.Valid() || !t.To.Valid() {
		return t, fmt.Errorf("%w: lifecycle %q to %q", ErrInvalidMessage, expect, t.To)
	}
	if err := checkAuthor(t.By); err != nil {
		return t, err
	}
	if t.At.IsZero() {
		return t, fmt.Errorf("%w: missing time", ErrInvalidMessage)
	}
	if len(t.TriggeredBy) > MaxTextBytes {
		return t, fmt.Errorf("%w: triggered_by over %d bytes", ErrTooLarge, MaxTextBytes)
	}
	t.At = NormTime(t.At)
	return t, nil
}

// Deletion is a channel delete: the state and rev the caller read, who
// deletes and when. The delete compares both, so a lifecycle change since
// the read (which may have moved the close time a retention counts from)
// or a new message refuses it, rather than deleting a channel the caller
// did not judge.
type Deletion struct {
	Expect State
	Rev    int64
	By     Author
	At     time.Time
}

// PrepareDeletion validates d and returns it with its time normalized.
func PrepareDeletion(d Deletion) (Deletion, error) {
	if !d.Expect.Valid() || d.Rev < 0 {
		return d, fmt.Errorf("%w: delete expecting %q at rev %d", ErrInvalidMessage, d.Expect, d.Rev)
	}
	if err := checkAuthor(d.By); err != nil {
		return d, err
	}
	if d.At.IsZero() {
		return d, fmt.Errorf("%w: missing time", ErrInvalidMessage)
	}
	d.At = NormTime(d.At)
	return d, nil
}

// CheckDeletion reports a channel whose state or rev is not the one d
// expects as [ErrConflict].
func CheckDeletion(k Key, state State, rev int64, d Deletion) error {
	if state != d.Expect || rev != d.Rev {
		return fmt.Errorf("%w: %s is %s at rev %d, expected %s at rev %d",
			ErrConflict, k, state, rev, d.Expect, d.Rev)
	}
	return nil
}

// CheckRekey validates the two ids of a rename.
func CheckRekey(oldID, newID string) error {
	if err := entity.ValidateID(oldID); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	if err := entity.ValidateID(newID); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	if oldID == newID {
		return fmt.Errorf("%w: rename to the same id", ErrInvalidKey)
	}
	return nil
}

// checkAuthor refuses a write with no user, for the reason the comments
// package refuses one: an unattributed message makes every own-message
// permission meaningless.
func checkAuthor(a Author) error {
	if strings.TrimSpace(a.User) == "" {
		return fmt.Errorf("%w: no author", ErrInvalidMessage)
	}
	if len(a.User) > MaxTextBytes || len(a.Tool) > MaxTextBytes || len(a.PrincipalType) > MaxTextBytes ||
		len(a.OrgID) > MaxTextBytes {

		return fmt.Errorf("%w: author field over %d bytes", ErrTooLarge, MaxTextBytes)
	}
	return nil
}

// A minted name is nameBits random bits written in base nameBase, padded
// to nameLen digits, the length of the largest such number.
const (
	nameBits = 64
	nameBase = 36
	nameLen  = 13
)

// NewName mints a name for a cardinality-many channel: 13 base36 digits
// from 64 random bits. A backend retries on the rare collision.
func NewName() string {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), nameBits))
	if err != nil { // coverage-ignore: crypto/rand does not fail on supported platforms
		panic(fmt.Sprintf("channels: read random: %v", err))
	}
	s := n.Text(nameBase)
	return strings.Repeat("0", nameLen-len(s)) + s
}

// TombstoneStamp returns the label a tombstone files channels under: the
// time, then random bits so two tombstones in one instant never collide.
func TombstoneStamp(at time.Time) string {
	return NormTime(at).Format(stampTimeLayout) + "-" + NewName()[7:]
}

// KeyLess orders keys by entity id, face, type and name: the order every
// list method returns.
func KeyLess(a, b Key) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	if a.Face != b.Face {
		return a.Face < b.Face
	}
	if a.Type != b.Type {
		return a.Type < b.Type
	}
	return a.Name < b.Name
}

// SortChannels sorts cs into [KeyLess] order.
func SortChannels(cs []Channel) {
	sort.Slice(cs, func(i, j int) bool { return KeyLess(cs[i].Key, cs[j].Key) })
}

// SortKeys sorts ks into [KeyLess] order.
func SortKeys(ks []Key) {
	sort.Slice(ks, func(i, j int) bool { return KeyLess(ks[i], ks[j]) })
}
