package realtime

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// The bounds of an inbox page.
const (
	// DefaultInboxLimit is the page size when the request names none.
	DefaultInboxLimit = 50
	// MaxInboxLimit is the largest page a request may ask for.
	MaxInboxLimit = 200
)

// inboxBounds are the ceilings of one inbox read: the most anchors the
// holder queries merge, and the most candidate channels one listing may
// return. readKeys is the most keys one marker read names.
type inboxBounds struct {
	anchors, rows, readKeys int
}

func defaultInboxBounds() inboxBounds {
	return inboxBounds{
		anchors: channels.MaxWaitingAnchors, rows: channels.MaxWaitingRows, readKeys: channels.MaxReadKeys,
	}
}

// The inbox kinds a request may ask for.
const (
	// InboxKindWaiting is the kind of an entry whose session waits on the
	// caller.
	InboxKindWaiting = "waiting"
	// InboxKindUnread asks for the channels with messages above the
	// caller's read marker.
	InboxKindUnread = "unread"
	// InboxKindMention asks for the unread messages that mention the
	// caller, and is the kind of such an entry.
	InboxKindMention = "mention"
	// InboxKindReply asks for the unread replies to the caller's messages,
	// and is the kind of such an entry.
	InboxKindReply = "reply"
)

// MaxInboxUnread is the most unread channels one inbox answer lists.
const MaxInboxUnread = 50

// MaxInboxMentions is the most mention and reply entries one inbox answer
// lists.
const MaxInboxMentions = 50

// InboxRequest asks for one page of the caller's inbox. Limit 0 is
// [DefaultInboxLimit], for a caller that names no limit; a surface that
// reads a limit the user typed checks it with [CheckInboxLimit] first, so
// a typed 0 is refused rather than read as the default. Cursor "" is the
// first page. Kinds is a subset of [InboxKindWaiting], [InboxKindUnread],
// [InboxKindMention] and [InboxKindReply]; empty asks for all four (see
// [CheckInboxKinds]).
type InboxRequest struct {
	Limit  int
	Cursor string
	Kinds  []string
}

// CheckInboxKinds checks kinds a caller named: each is one of the inbox
// kinds, else [TypeInvalid]. An empty list is all four.
func CheckInboxKinds(kinds []string) error {
	_, err := inboxKinds(kinds)
	return err
}

// askedKinds are the parts of the inbox a request asks for.
type askedKinds struct {
	waiting, unread, mention, reply bool
}

// addressed reports whether the request asks for mention or reply entries.
func (k askedKinds) addressed() bool { return k.mention || k.reply }

// addressKind is the kind of addressed row the request asks the store
// for: one kind, or [channels.AddressAny] for both. It is meaningful only
// when addressed holds.
func (k askedKinds) addressKind() channels.AddressKind {
	switch {
	case k.mention && !k.reply:
		return channels.AddressMention
	case k.reply && !k.mention:
		return channels.AddressReply
	default:
		return channels.AddressAny
	}
}

// inboxKinds reports which parts of the inbox kinds asks for.
func inboxKinds(kinds []string) (askedKinds, error) {
	if len(kinds) == 0 {
		return askedKinds{waiting: true, unread: true, mention: true, reply: true}, nil
	}
	var k askedKinds
	for _, name := range kinds {
		switch name {
		case InboxKindWaiting:
			k.waiting = true
		case InboxKindUnread:
			k.unread = true
		case InboxKindMention:
			k.mention = true
		case InboxKindReply:
			k.reply = true
		default:
			return askedKinds{}, invalid("kinds must be %s, %s, %s or %s", InboxKindWaiting, InboxKindUnread,
				InboxKindMention, InboxKindReply)
		}
	}
	return k, nil
}

// InboxView is one page of the caller's inbox: the sessions that wait on
// the caller now, oldest wait first.
//
// Count is the size of the whole window, not of this page. Truncated
// reports that more anchors named the caller than one inbox reads; it is
// computed after the row gate and redaction. Degraded reports a degraded
// configuration, under which no entity-property role grants anyone, so
// the inbox is empty.
//
// UnreadChannels are the channels of the caller's read window with
// messages above the marker, newest activity first, at most
// [MaxInboxUnread]; UnreadCount is how many there are in the window. Both
// are set on the first page when the request asks for unread, so a real 0
// is sent; they are nil otherwise. A request without the waiting kind
// leaves Entries, Count, Truncated and NextCursor out of the JSON rather
// than answering "0 waiting".
//
// Mentions are the unread messages that mention the caller or reply to
// the caller's messages, newest first, one entry per message, at most
// [MaxInboxMentions]; MentionCount is how many entries the gated window
// holds. Both are set on the first page when the request asks for the
// mention or reply kind; they are nil otherwise.
//
// WaitingTypes are the channel types that have a waiting state in the
// configuration the answer used, sorted. WakeKeys are the keys of every
// channel in the caller's gated read window and of every gated mention
// entry, sorted. Neither is part of the
// answer on any wire; a watcher uses them to skip channel changes that
// cannot change an inbox.
type InboxView struct {
	Notice         string         `json:"notice"`
	Entries        []InboxEntry   `json:"entries"`
	Count          int            `json:"count"`
	Truncated      bool           `json:"truncated"`
	Degraded       bool           `json:"degraded"`
	NextCursor     string         `json:"next_cursor,omitempty"`
	UnreadChannels []InboxUnread  `json:"unread_channels,omitzero"`
	UnreadCount    *int           `json:"unread_count,omitempty"`
	Mentions       []InboxMention `json:"mentions,omitzero"`
	MentionCount   *int           `json:"mention_count,omitempty"`
	WaitingTypes   []string       `json:"-"`
	WakeKeys       []channels.Key `json:"-"`
	// Reader is whose inbox the answer is: the caller's reader. It is not
	// part of the answer on any wire; the live hub uses it to tell which
	// moved markers can change this inbox.
	Reader channels.Reader `json:"-"`
	// noWaiting is set when the request left the waiting kind out.
	noWaiting bool
}

// InboxSummary counts what an inbox first page holds, for a badge.
type InboxSummary struct {
	Waiting          int
	WaitingTruncated bool
	UnreadChannels   int
	Mentions         int
	Degraded         bool
}

// Summary projects v. It reads only fields the first page already
// carries, so it discloses nothing the page does not. A nil UnreadCount
// or MentionCount counts 0.
func (v InboxView) Summary() InboxSummary {
	out := InboxSummary{Waiting: v.Count, WaitingTruncated: v.Truncated, Degraded: v.Degraded}
	if v.UnreadCount != nil {
		out.UnreadChannels = *v.UnreadCount
	}
	if v.MentionCount != nil {
		out.Mentions = *v.MentionCount
	}
	return out
}

// HasWaiting reports whether the answer carries the waiting entries: the
// request asked for the waiting kind.
func (v InboxView) HasWaiting() bool { return !v.noWaiting }

// waitingJSON are the JSON keys of the [InboxView] fields that belong to
// the waiting kind. MarshalJSON drops only these from an answer without
// it, so a field added later shows up in every answer by default.
var waitingJSON = []string{"entries", "count", "truncated", "next_cursor"}

// MarshalJSON leaves the waiting fields out of an answer without them.
func (v InboxView) MarshalJSON() ([]byte, error) {
	type plain InboxView
	b, err := json.Marshal(plain(v))
	if err != nil || !v.noWaiting {
		return b, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	for _, k := range waitingJSON {
		delete(fields, k)
	}
	return json.Marshal(fields)
}

// InboxUnread is a channel with messages above the caller's read marker:
// the marker, the channel's last seq and the counting messages between,
// at most [channels.UnreadCap].
type InboxUnread struct {
	Channel InboxChannel `json:"channel"`
	Seq     int64        `json:"seq"`
	LastSeq int64        `json:"last_seq"`
	Unread  int          `json:"unread"`
}

// InboxMention is one unread message that addresses the caller. Kind is
// [InboxKindMention] when the message mentions the caller, else
// [InboxKindReply]. It carries no message content: the caller reads the
// message in its channel.
type InboxMention struct {
	Kind        string       `json:"kind"`
	Channel     InboxChannel `json:"channel"`
	Seq         int64        `json:"seq"`
	At          time.Time    `json:"at"`
	MessageType string       `json:"message_type"`
	Author      AuthorView   `json:"author"`
	// ReplyTo is the parent seq of a reply; 0 otherwise.
	ReplyTo int64 `json:"reply_to,omitempty"`
}

// InboxEntry is one session that waits on the caller.
type InboxEntry struct {
	Kind    string       `json:"kind"`
	Channel InboxChannel `json:"channel"`
	Session InboxSession `json:"session"`
	// Roles are the waiting roles the caller holds, in the state's
	// waiting_for order.
	Roles []string `json:"roles"`
	// Since is when the session entered its state.
	Since time.Time `json:"since"`
	// Transitions are the message types whose post by the caller would move
	// the session, as [SessionRead] lists them.
	Transitions []string `json:"transitions"`
}

// InboxChannel names the channel of an [InboxEntry]. AnchorTitle is the
// anchor's display title from its redacted properties.
type InboxChannel struct {
	Anchor      string         `json:"anchor"`
	EntityType  string         `json:"entity_type"`
	AnchorTitle string         `json:"anchor_title"`
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Title       string         `json:"title,omitempty"`
	State       channels.State `json:"state"`
}

// InboxSession is the session of an [InboxEntry]: the part of
// [SessionView] that says what it waits for. It has no clock field, so an
// unchanged inbox renders the same on every read.
type InboxSession struct {
	State      string               `json:"state"`
	Label      string               `json:"label,omitempty"`
	Status     string               `json:"status"`
	WaitingFor []SessionWaitingView `json:"waiting_for,omitempty"`
	Generation int64                `json:"generation"`
	EnteredAt  time.Time            `json:"entered_at"`
}

// inboxCursor is the position after the last entry of a page. It positions
// by comparison only, so a cursor that names a key the caller cannot see
// reveals nothing.
type inboxCursor struct {
	V     int       `json:"v"`
	Since time.Time `json:"since"`
	ID    string    `json:"id"`
	Face  string    `json:"face,omitempty"`
	Type  string    `json:"type"`
	Name  string    `json:"name,omitempty"`
}

// CheckInboxLimit is the check of a limit a user named: a whole number
// from 1 to [MaxInboxLimit], else [TypeInvalid].
func CheckInboxLimit(n int) error {
	if n < 1 || n > MaxInboxLimit {
		return invalid("limit must be between 1 and %d", MaxInboxLimit)
	}
	return nil
}

func invalidCursor() *DenialError {
	return deny(http.StatusBadRequest, TypeInvalidCursor, "the cursor is not one an inbox answer gave")
}

func inboxTooLarge() *DenialError {
	return deny(http.StatusServiceUnavailable, TypeInboxTooLarge,
		"the inbox matches too many channels to answer; narrow the waiting roles or close old sessions, "+
			"or ask for kinds=unread,mention,reply")
}

func encodeCursor(e InboxEntry, key channels.Key) string {
	b, err := json.Marshal(inboxCursor{
		V: 1, Since: e.Since.UTC(), ID: key.ID, Face: string(key.Face), Type: key.Type, Name: key.Name,
	})
	if err != nil { // coverage-ignore: invariant: the cursor is plain data
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*inboxCursor, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // nil is the absent cursor, not a failure
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, invalidCursor()
	}
	var c inboxCursor
	if err := json.Unmarshal(b, &c); err != nil || c.V != 1 || c.key().Validate() != nil {
		return nil, invalidCursor()
	}
	c.Since = channels.NormTime(c.Since)
	return &c, nil
}

// inboxItem is an entry with its channel key, for sorting and paging.
type inboxItem struct {
	entry InboxEntry
	key   channels.Key
}

func compareItems(a, b inboxItem) int {
	return cmp.Or(a.entry.Since.Compare(b.entry.Since), cmp.Compare(a.key.ID, b.key.ID),
		cmp.Compare(a.key.Face, b.key.Face), cmp.Compare(a.key.Type, b.key.Type), cmp.Compare(a.key.Name, b.key.Name))
}

// key is the channel key the cursor names. It need not exist.
func (c *inboxCursor) key() channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: c.ID, Face: entity.Face(c.Face)}, Type: c.Type, Name: c.Name}
}

// after reports whether it sorts after the cursor position.
func (c *inboxCursor) after(it inboxItem) bool {
	return compareItems(it, inboxItem{entry: InboxEntry{Since: c.Since}, key: c.key()}) > 0
}

// Inbox lists the sessions that wait on the caller now: open, stored
// channels in an input_required state whose waiting role the caller holds
// for update, exactly where [Service.Session] answers waiting_on_me. On
// the first page it also lists the channels of the caller's read window
// with unread messages, and the unread messages that mention the caller
// or reply to them (see [InboxView]).
//
// It costs one holder query per waiting pair of the configuration, one
// channel listing, one read window listing, one addressed-row listing, one
// batch anchor read and one role prime shared by every part, and one
// marker read, whatever the number of entries. Every anchor passed the row gate and redaction
// before it is counted, so Truncated, the ceiling and the unread fields
// are taken after the gate. Nothing is cached: every call answers for its
// own principal.
func (s *Service) Inbox(ctx context.Context, req InboxRequest) (InboxView, error) {
	limit := req.Limit
	if limit == 0 {
		limit = DefaultInboxLimit
	}
	if err := CheckInboxLimit(limit); err != nil {
		return InboxView{}, err
	}
	kinds, err := inboxKinds(req.Kinds)
	if err != nil {
		return InboxView{}, err
	}
	waiting, unread := kinds.waiting, kinds.unread
	cur, err := decodeCursor(req.Cursor)
	if err != nil {
		return InboxView{}, err
	}
	if cur != nil && !waiting {
		// A cursor positions waiting entries only.
		return InboxView{}, invalidCursor()
	}
	c, err := s.begin(ctx)
	if err != nil {
		return InboxView{}, err
	}
	if denied := c.requireCaller(); denied != nil {
		return InboxView{}, denied
	}
	identity := c.acc.QueryIdentity()
	if !identity.Valid() {
		return InboxView{}, noPrincipal()
	}
	view := InboxView{Notice: UntrustedNotice, noWaiting: !waiting}
	view.Reader, _ = callReader(c)
	if waiting {
		view.Entries = []InboxEntry{}
	}
	if c.snap.Degraded {
		view.Degraded = true
		return view, nil
	}
	run := inboxRun{s: s, c: c}
	w := c.snap.Config.Waiting()
	view.WaitingTypes = slices.Sorted(maps.Keys(w.States))
	got, err := run.gather(&view, kinds, cur == nil, identity.ID())
	if err != nil {
		return InboxView{}, err
	}
	views, err := run.prime(got.stored, got.marks, got.rows)
	if err != nil {
		return InboxView{}, err
	}
	if waiting {
		run.page(&view, run.waitingItems(got.stored, views), cur, limit)
	}
	if unread && cur == nil {
		if err := run.unread(&view, got.marks, views); err != nil {
			return InboxView{}, err
		}
	}
	if kinds.addressed() && cur == nil {
		run.mentions(&view, got.rows, views)
	}
	channels.SortKeys(view.WakeKeys)
	view.WakeKeys = slices.Compact(view.WakeKeys)
	return view, nil
}

// inboxRun is the state of one [Service.Inbox] call: the service and the
// call it began. It keeps the inbox steps off Service and call.
type inboxRun struct {
	s *Service
	c *call
}

// inboxRows are the stored rows one inbox call judges: the waiting
// channels, the caller's read markers and the rows that address the
// caller. A kind not asked for, or not on this page, leaves its rows nil.
type inboxRows struct {
	stored []channels.Channel
	marks  []channels.ReadMarker
	rows   []channels.Addressed
}

// gather lists the rows each asked kind needs. first is set on a first
// page: only there are the unread and addressed halves computed. It sets
// view.Truncated when the holder query was cut short.
func (r inboxRun) gather(view *InboxView, kinds askedKinds, first bool, id string) (inboxRows, error) {
	var out inboxRows
	var err error
	if kinds.waiting {
		w := r.c.snap.Config.Waiting()
		var anchors []channels.Anchor
		if anchors, view.Truncated, err = r.holderAnchors(w, id); err != nil {
			return inboxRows{}, err
		}
		if out.stored, err = r.listWaiting(w, anchors); err != nil {
			return inboxRows{}, err
		}
	}
	if kinds.unread && first {
		if out.marks, err = r.readWindow(); err != nil {
			return inboxRows{}, err
		}
	}
	if kinds.addressed() && first {
		if out.rows, err = r.addressedWindow(kinds); err != nil {
			return inboxRows{}, err
		}
	}
	return out, nil
}

// page sorts items, counts them and fills view with the page after cur.
func (r inboxRun) page(view *InboxView, items []inboxItem, cur *inboxCursor, limit int) {
	slices.SortFunc(items, compareItems)
	view.Count = len(items)
	if cur != nil {
		items = slices.DeleteFunc(items, func(it inboxItem) bool { return !cur.after(it) })
	}
	if len(items) > limit {
		items = items[:limit]
		view.NextCursor = encodeCursor(items[limit-1].entry, items[limit-1].key)
	}
	for _, it := range items {
		view.Entries = append(view.Entries, it.entry)
	}
}

// holderAnchors runs one holder query per waiting pair and merges the
// anchors, deduplicated, keeping the first bounds.anchors (by default
// [channels.MaxWaitingAnchors]) in (id, face) order. truncated reports that a pair had more, or the merged
// set did not fit.
func (r inboxRun) holderAnchors(w realtimecfg.WaitingIndex, identity string) ([]channels.Anchor, bool, error) {
	s, c := r.s, r.c
	maxAnchors := s.bounds.anchors
	set := map[channels.Anchor]bool{}
	truncated := false
	for _, p := range w.Pairs {
		refs, more, err := s.holders.Holding(c.ctx, HolderQuery{
			EntityType: p.EntityType, Property: p.Property, Identity: identity, Face: p.Face,
			Limit: maxAnchors,
		})
		if err != nil {
			return nil, false, fmt.Errorf("realtime: find role holders: %w", err)
		}
		truncated = truncated || more
		for _, r := range refs {
			set[channels.Anchor{ID: r.ID, Face: r.Face}] = true
		}
	}
	out := make([]channels.Anchor, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b channels.Anchor) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Face, b.Face))
	})
	if len(out) > maxAnchors {
		out, truncated = out[:maxAnchors], true
	}
	return out, truncated, nil
}

// listWaiting lists the candidate channels that may wait on the caller.
func (r inboxRun) listWaiting(w realtimecfg.WaitingIndex, anchors []channels.Anchor) ([]channels.Channel, error) {
	s, c := r.s, r.c
	if len(w.States) == 0 || (len(anchors) == 0 && len(w.Creator) == 0) {
		return nil, nil
	}
	q := channels.WaitingQuery{States: w.States, Anchors: anchors, MaxRows: s.bounds.rows}
	if len(w.Creator) > 0 {
		q.CreatorTypes, q.CreatedBy = w.Creator, c.p.User
	}
	if len(q.Anchors) == 0 && !q.CreatorBranch() {
		return nil, nil
	}
	stored, err := s.store.ListWaiting(c.ctx, q)
	if errors.Is(err, channels.ErrTooManyRows) {
		slog.WarnContext(c.ctx, "realtime: inbox matches too many channels",
			"user", c.p.User, "tool", c.p.Tool, "anchors", len(anchors), "error", err)
		return nil, inboxTooLarge()
	}
	if err != nil {
		return nil, fmt.Errorf("realtime: list waiting channels: %w", err)
	}
	return stored, nil
}

// readWindow lists the caller's read window: their markers on live
// channels of a type the configuration still declares.
func (r inboxRun) readWindow() ([]channels.ReadMarker, error) {
	c := r.c
	reader, _ := callReader(c)
	marks, err := r.s.store.ListReads(c.ctx, reader, channels.MaxReadWindow)
	if err != nil {
		return nil, fmt.Errorf("realtime: list read markers: %w", err)
	}
	return slices.DeleteFunc(marks, func(m channels.ReadMarker) bool {
		_, ok := c.snap.Config.ChannelType(m.Channel.Key.Type)
		return !ok
	}), nil
}

// prime reads the anchors of the waiting candidates, the read window and
// the addressed rows in one batch, and primes the role answers for them
// once.
func (r inboxRun) prime(
	stored []channels.Channel, marks []channels.ReadMarker, rows []channels.Addressed,
) (map[entity.Address]AnchorView, error) {
	all := stored
	if len(marks) > 0 || len(rows) > 0 {
		all = make([]channels.Channel, 0, len(stored)+len(marks)+len(rows))
		all = append(all, stored...)
		for _, m := range marks {
			all = append(all, m.Channel)
		}
		for _, a := range rows {
			all = append(all, a.Channel)
		}
	}
	if len(all) == 0 {
		return map[entity.Address]AnchorView{}, nil
	}
	views, err := r.anchors(all)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	slices.Sort(ids)
	r.c.ctx = r.c.acc.Prime(r.c.ctx, slices.Compact(ids))
	return views, nil
}

// waitingItems keeps the candidate channels that wait on the caller,
// judged per channel as [Service.Session] judges one.
func (r inboxRun) waitingItems(stored []channels.Channel, views map[entity.Address]AnchorView) []inboxItem {
	var out []inboxItem
	for i := range stored {
		ch := stored[i]
		ct, view, ok := r.anchorOf(ch.Key, views)
		if !ok {
			continue
		}
		if it, ok := r.item(ct, view, &ch); ok {
			out = append(out, it)
		}
	}
	return out
}

// anchorOf returns the channel type of k and the anchor read for it, when
// the type is declared, the anchor was readable and is of a type the
// channel type attaches to.
func (r inboxRun) anchorOf(
	k channels.Key, views map[entity.Address]AnchorView,
) (realtimecfg.ChannelType, AnchorView, bool) {
	ct, ok := r.c.snap.Config.ChannelType(k.Type)
	if !ok {
		return ct, AnchorView{}, false
	}
	view, ok := views[channelAddress(k)]
	if !ok || !slices.Contains(ct.AttachTo, view.Type) {
		return ct, AnchorView{}, false
	}
	return ct, view, true
}

// unread fills view's unread fields from the read window marks: it gates
// each channel as the waiting entries are gated, counts the gated channels
// with messages above the marker in one marker read, and keeps those with
// unread messages in window order.
func (r inboxRun) unread(view *InboxView, marks []channels.ReadMarker, views map[entity.Address]AnchorView) error {
	c := r.c
	type gated struct {
		mark channels.ReadMarker
		view AnchorView
	}
	var kept []gated
	var keys, behind []channels.Key
	for i := range marks {
		ch := marks[i].Channel
		ct, av, ok := r.anchorOf(ch.Key, views)
		if !ok {
			continue
		}
		c.ct, c.anchor, c.key, c.head, c.ops = ct, av, ch.Key, &ch, opGates{}
		if !roleAdmits(c, &ch, ct.Read, realtimecfg.CapabilityRead) {
			continue
		}
		kept = append(kept, gated{mark: marks[i], view: av})
		keys = append(keys, ch.Key)
		if ch.LastSeq > marks[i].Seq {
			behind = append(behind, ch.Key)
		}
	}
	c.head = nil
	view.WakeKeys = append(view.WakeKeys, keys...)
	var states map[channels.Key]channels.ReadState
	if len(behind) > 0 {
		var err error
		reader, _ := callReader(c)
		if states, err = r.s.store.Reads(c.ctx, reader, behind); err != nil {
			return fmt.Errorf("realtime: read markers: %w", err)
		}
	}
	view.UnreadChannels = []InboxUnread{}
	n := 0
	for _, g := range kept {
		st, ok := states[g.mark.Channel.Key]
		if !ok || st.Unread == 0 {
			continue
		}
		n++
		if len(view.UnreadChannels) == MaxInboxUnread {
			continue
		}
		ch := g.mark.Channel
		view.UnreadChannels = append(view.UnreadChannels, InboxUnread{
			Channel: inboxChannel(ch, g.view), Seq: st.Seq, LastSeq: st.LastSeq, Unread: st.Unread,
		})
	}
	view.UnreadCount = &n
	return nil
}

// addressedWindow lists the caller's unread addressed rows of the kinds
// the request asks for, without the rows of a channel type the
// configuration no longer declares. The store filters the kind inside its
// scan, and returns every unread row the scan finds, so the only cut
// before the gate in [inboxRun.mentions] is the documented scan of
// [channels.MaxAddressedScan] rows: rows in channels the caller cannot
// read never push a visible row out.
func (r inboxRun) addressedWindow(kinds askedKinds) ([]channels.Addressed, error) {
	c := r.c
	reader, _ := callReader(c)
	rows, err := r.s.store.ListAddressed(c.ctx, reader, kinds.addressKind(), channels.MaxAddressedScan)
	if err != nil {
		return nil, fmt.Errorf("realtime: list addressed rows: %w", err)
	}
	return slices.DeleteFunc(rows, func(a channels.Addressed) bool {
		_, ok := c.snap.Config.ChannelType(a.Channel.Key.Type)
		return !ok
	}), nil
}

// mentions fills view's mention fields from the addressed rows: it gates
// each row's channel as the unread channels are gated, then keeps one
// entry per message, a mention before a reply, in the rows' newest-first
// order. The count is taken after the gate, so a row the caller may not
// see changes nothing.
func (r inboxRun) mentions(view *InboxView, rows []channels.Addressed, views map[entity.Address]AnchorView) {
	c := r.c
	type msgKey struct {
		key channels.Key
		seq int64
	}
	at := map[msgKey]int{}
	var out []InboxMention
	for i := range rows {
		a := rows[i]
		ch := a.Channel
		ct, av, ok := r.anchorOf(ch.Key, views)
		if !ok {
			continue
		}
		c.ct, c.anchor, c.key, c.head, c.ops = ct, av, ch.Key, &ch, opGates{}
		if !roleAdmits(c, &ch, ct.Read, realtimecfg.CapabilityRead) {
			continue
		}
		mk := msgKey{ch.Key, a.Seq}
		if j, seen := at[mk]; seen {
			if a.Kind == channels.AddressMention {
				out[j].Kind, out[j].ReplyTo = InboxKindMention, a.ReplyTo
			}
			continue
		}
		at[mk] = len(out)
		kind := InboxKindReply
		if a.Kind == channels.AddressMention {
			kind = InboxKindMention
		}
		out = append(out, InboxMention{
			Kind: kind, Channel: inboxChannel(ch, av), Seq: a.Seq, At: a.At, MessageType: a.Type,
			Author: authorView(a.Author), ReplyTo: a.ReplyTo,
		})
		view.WakeKeys = append(view.WakeKeys, ch.Key)
	}
	c.head = nil
	n := len(out)
	view.MentionCount = &n
	if len(out) > MaxInboxMentions {
		out = out[:MaxInboxMentions]
	}
	view.Mentions = append([]InboxMention{}, out...)
}

// inboxChannel names ch, anchored to view, in an inbox answer.
func inboxChannel(ch channels.Channel, view AnchorView) InboxChannel {
	return InboxChannel{
		Anchor: entity.FormatStateRef(ch.Key.ID, ch.Key.Face), EntityType: view.Type,
		AnchorTitle: view.Title, Type: ch.Key.Type, Name: ch.Key.Name, Title: ch.Title, State: ch.State,
	}
}

// anchors reads the anchors of stored in one batch, with the properties
// every involved channel type's roles read.
func (r inboxRun) anchors(stored []channels.Channel) (map[entity.Address]AnchorView, error) {
	c := r.c
	var addrs []entity.Address
	var props []string
	seen := map[entity.Address]bool{}
	types := map[string]bool{}
	for _, ch := range stored {
		addr := channelAddress(ch.Key)
		if !seen[addr] {
			seen[addr] = true
			addrs = append(addrs, addr)
		}
		if ct, ok := c.snap.Config.ChannelType(ch.Key.Type); ok && !types[ct.Name] {
			types[ct.Name] = true
			props = append(props, roleProps(ct)...)
		}
	}
	if len(addrs) == 0 {
		return map[entity.Address]AnchorView{}, nil
	}
	slices.Sort(props)
	views, err := r.s.anchors.Anchors(c.ctx, addrs, slices.Compact(props))
	if err != nil {
		return nil, fmt.Errorf("realtime: read anchors: %w", err)
	}
	return views, nil
}

// channelAddress is the address the anchor of k is read at: the bare id for
// a family channel, `ID@face` for a face channel.
func channelAddress(k channels.Key) entity.Address {
	addr, err := entity.ParseAddress(entity.FormatStateRef(k.ID, k.Face))
	if err != nil { // coverage-ignore: invariant: a stored key is a valid address
		return entity.Address{}
	}
	return addr
}

// item judges one stored channel for the caller: the read role, then the
// session's waiting view. The role caches are reset first, because a
// creator answer is a fact of the channel.
func (r inboxRun) item(ct realtimecfg.ChannelType, view AnchorView, ch *channels.Channel) (inboxItem, bool) {
	c := r.c
	c.ct, c.anchor, c.key, c.head, c.ops = ct, view, ch.Key, ch, opGates{}
	if !roleAdmits(c, ch, ct.Read, realtimecfg.CapabilityRead) {
		return inboxItem{}, false
	}
	facts := waitingOf(c, ch)
	sv := sessionView(ct, ch, c.snap.Degraded, facts)
	if sv == nil || !sv.WaitingOnMe {
		return inboxItem{}, false
	}
	st, ok := ct.Session().State(sv.State)
	if !ok {
		return inboxItem{}, false
	}
	roles := []string{}
	for _, name := range st.WaitingFor() {
		if r, ok := ct.Roles[name]; ok && facts.held(r) {
			roles = append(roles, name)
		}
	}
	// The inbox lists the types only; an agent reads the entries with
	// read_session.
	types, _ := c.sessionAffordances(c.postable())
	return inboxItem{
		key: ch.Key,
		entry: InboxEntry{
			Kind:    InboxKindWaiting,
			Channel: inboxChannel(*ch, view),
			Session: InboxSession{
				State: sv.State, Label: sv.Label, Status: sv.Status, WaitingFor: sv.WaitingFor,
				Generation: sv.Generation, EnteredAt: sv.EnteredAt,
			},
			Roles:       roles,
			Since:       sv.EnteredAt,
			Transitions: types,
		},
	}, true
}
