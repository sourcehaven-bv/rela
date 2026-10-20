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
}

// ChannelHead is the `GET` answer for one channel: its metadata, the last
// message, the live ephemeral messages and the caller's actions. It is the
// shape a live subscription follows; after a frame, fetch
// `messages?after_rev=` with the rev you hold.
type ChannelHead struct {
	Channel   Channel            `json:"channel"`
	Last      *Message           `json:"last,omitempty"`
	Ephemeral []EphemeralMessage `json:"ephemeral"`
	Actions   ChannelActions     `json:"_actions"`
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

// Message is one stored message. A removed message keeps only seq, rev,
// reply_to, reply_count and removed. Props holds the properties the
// message type declares; a ref property is a [RefProp]. Actions lists,
// sorted, what the reader may do with the message now: the message
// type's actions (`POST messages/{seq}/_action/{name}`) and the
// operations edit (`PATCH messages/{seq}`), delete (the author's
// `DELETE messages/{seq}`), remove (a moderator's DELETE with a reason)
// and release (`DELETE messages/{seq}/_action`). Edit is left out while a
// ref property renders as restricted. Every operation is authorized
// again.
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
	Actions     []string        `json:"_actions,omitempty"`
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
// every property of the message.
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
type ChannelSchema struct {
	ChannelTypes []ChannelTypeSchema `json:"channel_types"`
}

// ChannelTypeSchema describes one channel type. Anchor is "family" or
// "face"; CreateLabel is the label of create, set only for a cardinality
// many type that names one. Lifecycle lists the declared operations in
// the order close, reopen, archive, delete.
type ChannelTypeSchema struct {
	Name                 string                     `json:"name"`
	Label                string                     `json:"label"`
	AttachTo             []string                   `json:"attach_to"`
	Anchor               string                     `json:"anchor"`
	Cardinality          string                     `json:"cardinality"`
	CreateLabel          string                     `json:"create_label,omitempty"`
	RemoveReasonRequired bool                       `json:"remove_reason_required"`
	Lifecycle            []ChannelLifecycleSchema   `json:"lifecycle"`
	MessageTypes         []ChannelMessageTypeSchema `json:"message_types"`
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
