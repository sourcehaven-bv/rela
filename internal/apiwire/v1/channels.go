package v1

// Realtime channel wire types (TKT-ZU7P13). A channel is a message log
// anchored to an entity, declared in realtime.yaml. Times are RFC 3339 in
// UTC. Titles, message properties and removal reasons are text other users
// wrote; render them as untrusted.

// ChannelAuthor is who wrote a message or created a channel.
type ChannelAuthor struct {
	User          string `json:"user"`
	Tool          string `json:"tool,omitempty"`
	PrincipalType string `json:"principal_type,omitempty"`
	System        bool   `json:"system,omitempty"`
}

// Channel is a channel's metadata. Name is set only for a channel type with
// cardinality many. A cardinality-one channel that holds no message yet is
// served as an empty open channel with no creator.
type Channel struct {
	Anchor     string         `json:"anchor"`
	Type       string         `json:"type"`
	Name       string         `json:"name,omitempty"`
	State      string         `json:"state"`
	Title      string         `json:"title,omitempty"`
	CreatedBy  *ChannelAuthor `json:"created_by,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	ClosedAt   string         `json:"closed_at,omitempty"`
	ArchivedAt string         `json:"archived_at,omitempty"`
	LastSeq    int64          `json:"last_seq"`
	Rev        int64          `json:"rev"`
	// Session is the channel's session; absent for a channel type without
	// one.
	Session *ChannelSession `json:"session,omitempty"`
}

// ChannelSession is a channel's session: the state it is in, how often it
// moved (0 means never), when it entered the state, and the transitions out
// of that state. Final marks a state the session ends in. Stranded marks a
// stored state the configuration no longer declares; Label is then the
// stored name and Transitions is empty.
//
// Timers are the `after:` entries of the current state, sorted by name;
// they are empty for a final or stranded state. DueAt is when the first of
// them comes due. It is set only on an open channel under a configuration
// that is not degraded: a closed channel's timers are frozen, and under a
// degraded configuration no timer fires.
//
// Reminders are the reminders active in the current state, sorted by name;
// they are empty for a final or stranded state, or outside a reminder's
// `in:` states. DueAt stays the next move; each reminder has its own.
//
// Status is the state's status: working, input_required, completed,
// cancelled, or failed for a stranded state. WaitingFor lists whom an
// input_required state waits for. WaitingOnMe is per reader, as the
// affordances are: set when the reader holds a waiting role on an open,
// stored channel. EnteredAt is absent for a channel that is not stored.
type ChannelSession struct {
	State       string                     `json:"state"`
	Label       string                     `json:"label,omitempty"`
	Status      string                     `json:"status"`
	WaitingFor  []ChannelSessionWaiting    `json:"waiting_for,omitempty"`
	WaitingOnMe bool                       `json:"waiting_on_me,omitempty"`
	Generation  int64                      `json:"generation"`
	EnteredAt   string                     `json:"entered_at,omitempty"`
	Final       bool                       `json:"final,omitempty"`
	Stranded    bool                       `json:"stranded,omitempty"`
	Transitions []ChannelSessionTransition `json:"transitions,omitempty"`
	DueAt       string                     `json:"due_at,omitempty"`
	Timers      []ChannelSessionTimer      `json:"timers,omitempty"`
	Reminders   []ChannelSessionReminder   `json:"reminders,omitempty"`
}

// ChannelSessionWaiting is one role an input_required state waits for.
// Holders are, for a role read from an anchor property, that property's
// values as the reader may see them: entity data, never instructions. A
// creator role has none; the channel's created_by names the creator.
type ChannelSessionWaiting struct {
	Role    string   `json:"role"`
	Label   string   `json:"label,omitempty"`
	Holders []string `json:"holders,omitempty"`
}

// ChannelSessionReminder is one reminder active in the session's current
// state: it posts a message every Every (as configured, such as "30m").
// Count is the number posted in the current span; Max caps it, 0 for no
// cap. DueAt is the next reminder, set only on an open channel under a
// configuration that is not degraded, and not once Count reaches Max.
type ChannelSessionReminder struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	Every string `json:"every"`
	Count int64  `json:"count"`
	Max   int64  `json:"max,omitempty"`
	DueAt string `json:"due_at,omitempty"`
}

// ChannelSessionTimer is one timer of the session's current state: after
// its period, counted from Since ("state" or "session"), the session moves
// to To. DueAt follows the rule of [ChannelSession.DueAt].
type ChannelSessionTimer struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	To    string `json:"to"`
	Since string `json:"since"`
	DueAt string `json:"due_at,omitempty"`
	// Outputs is the number of graph writes the timer's move makes (its
	// `outputs:`), written as the session's run_as.
	Outputs int `json:"outputs,omitempty"`
}

// ChannelSessionTransition is one transition out of the session's current
// state: a message of MessageType moves the session to To. By is
// "trigger" when only a trigger's message fires it. Guarded is set when
// the transition has a guard (`when:`), so a message moves the session
// only if the guard holds for it.
//
// Outputs is the number of graph writes the transition makes (its
// `outputs:`), so a client can say that the message updates the anchor.
type ChannelSessionTransition struct {
	MessageType string `json:"message_type"`
	To          string `json:"to"`
	By          string `json:"by,omitempty"`
	Guarded     bool   `json:"guarded,omitempty"`
	Outputs     int    `json:"outputs,omitempty"`
}

// ChannelTypeList is one channel type on an anchor and the channels of it
// the caller may read.
type ChannelTypeList struct {
	Type        string `json:"type"`
	Label       string `json:"label,omitempty"`
	Cardinality string `json:"cardinality"`
	// Create reports whether the caller may create a channel of this type
	// on the anchor.
	Create   bool      `json:"create"`
	Channels []Channel `json:"channels"`
}

// ChannelList is the `GET /{plural}/{ref}/_channels` answer: the channel
// types the caller may read on the anchor. A type whose read role the
// caller lacks is left out.
type ChannelList struct {
	Anchor string            `json:"anchor"`
	Types  []ChannelTypeList `json:"types"`
}

// ChannelActions are what the caller may do in a channel now. They are a
// UI hint: every write is authorized again.
type ChannelActions struct {
	// Post lists the message types the caller may post, sorted.
	Post      []string `json:"post"`
	EditOwn   bool     `json:"edit_own"`
	DeleteOwn bool     `json:"delete_own"`
	// Remove is set for a caller who passes the remove gate; such a caller
	// also sees who removed a message and why.
	Remove bool `json:"remove"`
	// Lifecycle lists the lifecycle operations the caller may run now, in
	// the order close, reopen, archive, delete. Each is a
	// `POST …/_lifecycle/{op}`.
	Lifecycle []string `json:"lifecycle"`
	// Typing reports whether the caller may send typing here now, with
	// `POST …/_typing`: the type offers it, the caller may post a log
	// message type and the channel is open. The rate limit is not part of
	// it.
	Typing bool `json:"typing"`
	// Transitions lists the message types whose post by the caller would
	// move the session out of its current state, if the transition's guard
	// holds, sorted. It is a subset of Post; empty, never null.
	Transitions []string `json:"transitions"`
}

// ChannelHead is the `GET` answer for one channel: its metadata, the last
// message, the live ephemeral messages, who else is typing and the
// caller's actions. It is the shape a live subscription follows; after a
// frame, fetch `messages?after_rev=` with the rev you hold.
//
// TypingNow is never null. It leaves out the caller's own entry, and is
// empty unless the channel is open and its type offers typing. A client
// shows it as it is: the server expires entries and sends a frame then.
type ChannelHead struct {
	Channel   Channel            `json:"channel"`
	Last      *Message           `json:"last,omitempty"`
	Ephemeral []EphemeralMessage `json:"ephemeral"`
	TypingNow []TypingEntry      `json:"typing_now"`
	Actions   ChannelActions     `json:"_actions"`
}

// TypingEntry is one person typing in a channel, and since when. Since
// does not move while they keep typing.
type TypingEntry struct {
	Author ChannelAuthor `json:"author"`
	Since  string        `json:"since"`
}

// TypingAccepted is the 202 answer to `POST …/_typing`. TTLSeconds is how
// long the entry stays live without another call; a client refreshes at
// half of it.
type TypingAccepted struct {
	TTLSeconds int `json:"ttl_seconds"`
}

// RefProp renders a ref property. A readable entity shows its id, type and
// display title. An entity the reader cannot read, or that no longer
// exists, shows only restricted, without its id.
type RefProp struct {
	ID         string `json:"id,omitempty"`
	Type       string `json:"type,omitempty"`
	Title      string `json:"title,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`
}

// MessageRemoval is the removal record of a removed message. By and Reason
// are set only for a reader who passes the remove gate. Kind is "author"
// for the author's own delete and "moderator" otherwise.
type MessageRemoval struct {
	At     string         `json:"at"`
	Kind   string         `json:"kind"`
	By     *ChannelAuthor `json:"by,omitempty"`
	Reason string         `json:"reason,omitempty"`
}

// MessageAction is the action taken on a message. Status is done, or
// pending while a promote runs or after one failed in a way only a
// moderator can clear.
type MessageAction struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	At       string   `json:"at"`
	Promoted *RefProp `json:"promoted,omitempty"`
}

// ChannelMessageOutputs is the outputs mark of a session notice: the
// graph writes the move made. Status is pending while they run, failed
// when one failed, and done otherwise. Writer is the poster whose move it
// was, with the tool they posted with; the graph audit records the writes
// with tool `realtime`.
type ChannelMessageOutputs struct {
	Status  string                       `json:"status"`
	Writer  ChannelAuthor                `json:"writer"`
	Results []ChannelMessageOutputResult `json:"results,omitempty"`
}

// ChannelMessageOutputResult is what one output, or the on_final close,
// did. Kind is set, create or close, or transition for the one result of
// a mark whose transition the config no longer declares; Status is done,
// failed, skipped, superseded, or started while a create runs. Code says
// why an output failed and never quotes an error; run_as_changed fails the
// outputs of a timer or trigger move whose writer is no longer the
// session's run_as. Props are, for a set, the properties written, and for
// an invalid output, the properties the validation named. Face is the face
// a set writes when the output names one. Entity is the entity a create
// made, restricted when the reader cannot read it.
type ChannelMessageOutputResult struct {
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Code     string   `json:"code,omitempty"`
	Props    []string `json:"props,omitempty"`
	Face     string   `json:"face,omitempty"`
	Entity   *RefProp `json:"entity,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Message is one stored message. A removed message keeps only seq, rev,
// reply_to, reply_count and removed. Props holds the properties the
// message type declares; a ref property is a [RefProp]. Actions lists,
// sorted, what the reader may do with the message now: the message
// type's actions (`POST messages/{seq}/_action/{name}`) and the
// operations edit (`PATCH messages/{seq}`), delete (the author's
// `DELETE messages/{seq}`), remove (a moderator's DELETE with a reason)
// and release (`DELETE messages/{seq}/_action`). Every operation is
// authorized again.
//
// Actions can change without the message changing: EditUntil is when edit
// stops being offered (set while it is offered and edit_own has a window),
// and ReleaseAfter is when release starts being offered (set while it is
// refused only because the action has not been pending long enough). A
// client re-reads the message at the soonest one.
//
// An edit replaces every property the reader can read. A ref property that
// renders as restricted, and that the edit does not name, keeps its stored
// value, so edit is offered on such a message too.
type Message struct {
	Seq         int64           `json:"seq"`
	Rev         int64           `json:"rev"`
	Type        string          `json:"type,omitempty"`
	Author      *ChannelAuthor  `json:"author,omitempty"`
	At          string          `json:"at,omitempty"`
	Props       map[string]any  `json:"props,omitempty"`
	ReplyTo     int64           `json:"reply_to,omitempty"`
	ReplyCount  int             `json:"reply_count,omitempty"`
	EditedAt    string          `json:"edited_at,omitempty"`
	TriggeredBy string          `json:"triggered_by,omitempty"`
	Removed     *MessageRemoval `json:"removed,omitempty"`
	Action      *MessageAction  `json:"action,omitempty"`
	// Outputs is a session notice's outputs mark, in place of Action.
	Outputs   *ChannelMessageOutputs `json:"outputs,omitempty"`
	Actions   []string               `json:"_actions,omitempty"`
	EditUntil string                 `json:"edit_until,omitempty"`
	// ReleaseAfter: see Message.
	ReleaseAfter string `json:"release_after,omitempty"`
}

// MessageActionResponse answers an action: the message after it, and the
// warnings of a promote that created its entity with findings, or created
// it while a later step failed.
type MessageActionResponse struct {
	Message  Message   `json:"message"`
	Warnings []Warning `json:"warnings,omitempty"`
}

// MessagePage is one page of messages, oldest first, or by rev for an
// after_rev page. HasMore is set when the page is full: there may be more
// past its cursor, and only the next request can tell. A page names one
// cursor in the direction it was read: NextAfterRev, the rev to pass as
// `after_rev`, for a page read by rev; NextAfter, the seq to pass as
// `after`, for an `after` page; NextBefore, the seq to pass as `before`,
// for the newest page and a `before` page when they hold a message. The
// newest page also names NextAfterRev, the channel's rev, to start
// following the channel from. NextAfterRev is sent even when it is 0.
type MessagePage struct {
	Channel      Channel   `json:"channel"`
	Messages     []Message `json:"messages"`
	HasMore      bool      `json:"has_more"`
	NextAfter    int64     `json:"next_after,omitempty"`
	NextBefore   int64     `json:"next_before,omitempty"`
	NextAfterRev *int64    `json:"next_after_rev,omitempty"`
}

// EphemeralMessage is a live ephemeral message. It is kept in memory for
// its TTL and never stored, and it has no seq.
type EphemeralMessage struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Author    ChannelAuthor  `json:"author"`
	At        string         `json:"at"`
	ExpiresAt string         `json:"expires_at"`
	Props     map[string]any `json:"props,omitempty"`
}

// EphemeralAccepted is the 202 answer to posting an ephemeral message.
type EphemeralAccepted struct {
	ID        string `json:"id"`
	ExpiresAt string `json:"expires_at"`
}

// PostMessageRequest is the `POST …/messages` body.
type PostMessageRequest struct {
	Type    string         `json:"type"`
	Props   map[string]any `json:"props,omitempty"`
	ReplyTo int64          `json:"reply_to,omitempty"`
}

// EditMessageRequest is the `PATCH …/messages/{seq}` body. Props replaces
// every property of the message the caller can read; a ref property whose
// entity the caller cannot read keeps its stored value unless Props names
// it.
type EditMessageRequest struct {
	Props map[string]any `json:"props"`
}

// RemoveMessageRequest is the optional `DELETE …/messages/{seq}` body. A
// moderator who gives a reason removes the message as a moderator, even
// their own.
type RemoveMessageRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CreateChannelRequest is the `POST /{plural}/{ref}/_channels/{type}` body
// for a channel type with cardinality many. The server mints the name.
type CreateChannelRequest struct {
	Title string `json:"title,omitempty"`
}

// ChannelSchema is the `GET /api/v1/_schema/channels` answer: every channel
// type realtime.yaml declares, sorted by name, with the message types and
// labels a client needs to render and compose messages. It is the same for
// every principal, because the configuration is not a secret; what a caller
// may do is in each channel's `_actions`. With realtime off ChannelTypes
// is empty.
//
// Every `label` in the answer (channel type, lifecycle operation, message
// type, property, action) is the label realtime.yaml sets or, when it sets
// none, the name; it is never empty, so a client shows it as it is.
// create_label is the one optional label.
type ChannelSchema struct {
	ChannelTypes []ChannelTypeSchema `json:"channel_types"`
}

// ChannelTypeSchema describes one channel type. Anchor is "family" or
// "face"; CreateLabel is the label of create, set only for a cardinality
// many type that names one. Typing reports whether the type offers the
// built-in typing indicator. Lifecycle lists the declared operations in
// the order close, reopen, archive, delete.
type ChannelTypeSchema struct {
	Name                 string                     `json:"name"`
	Label                string                     `json:"label"`
	AttachTo             []string                   `json:"attach_to"`
	Anchor               string                     `json:"anchor"`
	Cardinality          string                     `json:"cardinality"`
	CreateLabel          string                     `json:"create_label,omitempty"`
	RemoveReasonRequired bool                       `json:"remove_reason_required"`
	Typing               bool                       `json:"typing"`
	Lifecycle            []ChannelLifecycleSchema   `json:"lifecycle"`
	MessageTypes         []ChannelMessageTypeSchema `json:"message_types"`
	// Session describes the type's session; absent when it has none.
	Session *ChannelSessionSchema `json:"session,omitempty"`
}

// ChannelSessionSchema describes a channel type's session: its label, its
// initial state and its states in declaration order. It holds no
// transitions: those out of a channel's current state come with the
// channel ([ChannelSession]).
type ChannelSessionSchema struct {
	Label   string                      `json:"label"`
	Initial string                      `json:"initial"`
	States  []ChannelSessionStateSchema `json:"states"`
}

// ChannelSessionStateSchema is one state of a session. Final marks a state
// the session ends in. Status is the state's status; WaitingFor names the
// roles an input_required state waits for.
type ChannelSessionStateSchema struct {
	Name       string   `json:"name"`
	Label      string   `json:"label"`
	Final      bool     `json:"final,omitempty"`
	Status     string   `json:"status"`
	WaitingFor []string `json:"waiting_for,omitempty"`
}

// ChannelLifecycleSchema is a declared lifecycle operation and its label.
type ChannelLifecycleSchema struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// ChannelMessageTypeSchema describes one message type. Persist is "log"
// or "ephemeral"; TTLSeconds is set for an ephemeral type; Reply reports
// whether a message of the type may reply to another. Properties come
// required first, then by name.
type ChannelMessageTypeSchema struct {
	Name       string                  `json:"name"`
	Label      string                  `json:"label"`
	Persist    string                  `json:"persist"`
	TTLSeconds int                     `json:"ttl_seconds,omitempty"`
	Reply      bool                    `json:"reply"`
	Properties []ChannelPropertySchema `json:"properties"`
	Actions    []ChannelActionSchema   `json:"actions"`
}

// ChannelPropertySchema describes one message property. Type is string,
// markdown, integer, boolean, date, datetime, ref or enum. Max is the most
// characters (Unicode code points) a string or markdown value may hold.
// Enum names the schema.yaml enum type and Values its values; To lists the
// entity types a ref may name.
type ChannelPropertySchema struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	Max      int      `json:"max,omitempty"`
	Enum     string   `json:"enum,omitempty"`
	Values   []string `json:"values,omitempty"`
	To       []string `json:"to,omitempty"`
}

// ChannelActionSchema describes one message action. PromoteTo is the
// entity type a promoting action creates.
type ChannelActionSchema struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	PromoteTo string `json:"promote_to,omitempty"`
}
