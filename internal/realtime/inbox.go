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
// return.
type inboxBounds struct {
	anchors, rows int
}

func defaultInboxBounds() inboxBounds {
	return inboxBounds{anchors: channels.MaxWaitingAnchors, rows: channels.MaxWaitingRows}
}

// InboxKindWaiting is the kind of an entry whose session waits on the
// caller. It is the only kind today.
const InboxKindWaiting = "waiting"

// InboxRequest asks for one page of the caller's inbox. Limit 0 is
// [DefaultInboxLimit], for a caller that names no limit; a surface that
// reads a limit the user typed checks it with [CheckInboxLimit] first, so
// a typed 0 is refused rather than read as the default. Cursor "" is the
// first page.
type InboxRequest struct {
	Limit  int
	Cursor string
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
// WaitingTypes are the channel types that have a waiting state in the
// configuration the answer used, sorted. They are not part of the answer
// on any wire; a watcher uses them to skip channel changes that cannot
// change an inbox.
type InboxView struct {
	Notice       string       `json:"notice"`
	Entries      []InboxEntry `json:"entries"`
	Count        int          `json:"count"`
	Truncated    bool         `json:"truncated"`
	Degraded     bool         `json:"degraded"`
	NextCursor   string       `json:"next_cursor,omitempty"`
	WaitingTypes []string     `json:"-"`
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
		"the inbox matches too many channels to answer; narrow the waiting roles or close old sessions")
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
// for update, exactly where [Service.Session] answers waiting_on_me.
//
// It costs one holder query per waiting pair of the configuration, one
// channel listing, one batch anchor read and one role prime, whatever the
// number of entries. Every anchor passed the row gate and redaction
// before it is counted, so Truncated and the ceiling are taken after the
// gate. Nothing is cached: every call answers for its own principal.
func (s *Service) Inbox(ctx context.Context, req InboxRequest) (InboxView, error) {
	limit := req.Limit
	if limit == 0 {
		limit = DefaultInboxLimit
	}
	if err := CheckInboxLimit(limit); err != nil {
		return InboxView{}, err
	}
	cur, err := decodeCursor(req.Cursor)
	if err != nil {
		return InboxView{}, err
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
	view := InboxView{Notice: UntrustedNotice, Entries: []InboxEntry{}}
	if c.snap.Degraded {
		view.Degraded = true
		return view, nil
	}
	run := inboxRun{s: s, c: c}
	w := c.snap.Config.Waiting()
	view.WaitingTypes = slices.Sorted(maps.Keys(w.States))
	anchors, truncated, err := run.holderAnchors(w, identity.ID())
	if err != nil {
		return InboxView{}, err
	}
	view.Truncated = truncated
	items, err := run.waitingItems(w, anchors)
	if err != nil {
		return InboxView{}, err
	}
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
	return view, nil
}

// inboxRun is the state of one [Service.Inbox] call: the service and the
// call it began. It keeps the inbox steps off Service and call.
type inboxRun struct {
	s *Service
	c *call
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

// waitingItems lists the candidate channels and keeps those that wait on
// the caller, judged per channel as [Service.Session] judges one.
func (r inboxRun) waitingItems(w realtimecfg.WaitingIndex, anchors []channels.Anchor) ([]inboxItem, error) {
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
	views, err := r.anchors(stored)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	slices.Sort(ids)
	c.ctx = c.acc.Prime(c.ctx, slices.Compact(ids))
	var out []inboxItem
	for i := range stored {
		ch := stored[i]
		ct, ok := c.snap.Config.ChannelType(ch.Key.Type)
		if !ok {
			continue
		}
		view, ok := views[channelAddress(ch.Key)]
		if !ok || !slices.Contains(ct.AttachTo, view.Type) {
			continue
		}
		if it, ok := r.item(ct, view, &ch); ok {
			out = append(out, it)
		}
	}
	return out, nil
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
	return inboxItem{
		key: ch.Key,
		entry: InboxEntry{
			Kind: InboxKindWaiting,
			Channel: InboxChannel{
				Anchor: entity.FormatStateRef(ch.Key.ID, ch.Key.Face), EntityType: view.Type,
				AnchorTitle: view.Title, Type: ch.Key.Type, Name: ch.Key.Name, Title: ch.Title, State: ch.State,
			},
			Session: InboxSession{
				State: sv.State, Label: sv.Label, Status: sv.Status, WaitingFor: sv.WaitingFor,
				Generation: sv.Generation, EnteredAt: sv.EnteredAt,
			},
			Roles:       roles,
			Since:       sv.EnteredAt,
			Transitions: c.fireable(c.postable()),
		},
	}, true
}
