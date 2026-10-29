package realtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// BusyAnchor is one row a busy read asks about: the row's ref and its
// entity type.
type BusyAnchor struct {
	Ref  entity.Ref
	Type string
}

// BusyView is one busy channel of a row: a channel whose session is in a
// state with status working. TypeLabel and Label come from the
// configuration in force. Since is when the session entered the state.
//
// [BusyReader.Busy] lists a row's views oldest Since first, then by
// Name, so a client that shows one entry shows the run that has worked
// longest, the likeliest to have stalled.
type BusyView struct {
	Type      string
	TypeLabel string
	Name      string
	State     string
	Label     string
	Since     time.Time
}

// BusyReader answers which channels of one type are busy on a page of
// rows. It is a type of its own because [Service] is at its plimsoll
// method cap.
type BusyReader struct {
	s *Service
}

// NewBusyReader returns the busy read over s.
//
// Nil: s is rejected.
func NewBusyReader(s *Service) (*BusyReader, error) {
	if s == nil {
		return nil, errNil("Service")
	}
	return &BusyReader{s: s}, nil
}

// Busy lists, per row address ([entity.AddressOf] of the row's ref), the
// busy channels of channelType on the rows of anchors, in the order
// [BusyView] gives. A busy channel is open and live, addressable under
// the type's cardinality (the unnamed channel for one, a named channel
// for many), its current session state has status working, and the
// caller may read it: the anchor through the gated reader and the
// channel through the type's read role. A row with no busy channel is
// absent from the map.
//
// ok is false when the read cannot be served for a reason of
// configuration or identity: realtime off, a request without a principal,
// a degraded configuration, an unknown type, a type without a session or
// without a working state, or no row of a type the channel type attaches
// to. Such a case is not an error and logs nothing, so a list never fails
// for it. A store error is returned.
//
// It costs one [channels.Store.ListWaiting] call, one anchor batch and one
// role prime, whatever the number of rows. On the file backend that one
// call reads a face directory and each channel's meta.yaml per anchor, so
// its I/O grows with the page although the call count does not. Nothing
// is cached: every call answers for its own principal.
func (b *BusyReader) Busy(
	ctx context.Context, channelType string, anchors []BusyAnchor,
) (rows map[entity.Address][]BusyView, ok bool, err error) {
	s := b.s
	c, err := s.begin(ctx)
	if err != nil {
		d, denied := errors.AsType[*DenialError](err)
		if errors.Is(err, ErrRealtimeOff) || (denied && d.Type == TypeNoPrincipal) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if c.snap.Degraded {
		return nil, false, nil
	}
	ct, ok := c.snap.Config.ChannelType(channelType)
	if !ok || ct.Session() == nil {
		return nil, false, nil
	}
	working := ct.Session().WorkingStates()
	if len(working) == 0 {
		return nil, false, nil
	}
	run := busyRun{s: s, c: c, ct: ct, rows: map[channels.Anchor][]entity.Address{}}
	if !run.collect(anchors) {
		return nil, false, nil
	}
	out := map[entity.Address][]BusyView{}
	if len(run.anchors) == 0 {
		return out, true, nil
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	stored, err := s.store.ListWaiting(c.ctx, channels.WaitingQuery{
		States: map[string][]string{ct.Name: working}, Anchors: run.anchors, MaxRows: run.ceiling(),
		Names: run.names(),
	})
	if err != nil {
		return nil, false, fmt.Errorf("realtime: list busy channels: %w", err)
	}
	views, err := run.prime(stored)
	if err != nil {
		return nil, false, err
	}
	for i := range stored {
		ch := stored[i]
		v, ok := run.entry(&ch, views)
		if !ok {
			continue
		}
		for _, addr := range run.rows[ch.Key.Anchor] {
			out[addr] = append(out[addr], v)
		}
	}
	for _, vs := range out {
		slices.SortFunc(vs, func(a, b BusyView) int {
			return cmp.Or(a.Since.Compare(b.Since), cmp.Compare(a.Name, b.Name))
		})
	}
	return out, true, nil
}

// busyRun is the state of one [BusyReader.Busy] call, as [inboxRun] is for
// the inbox: the service, the call, the channel type and the anchors
// asked about.
type busyRun struct {
	s  *Service
	c  *call
	ct realtimecfg.ChannelType
	// anchors are the channel anchors to query, deduplicated, in row order.
	anchors []channels.Anchor
	// rows maps a channel anchor to the rows it shows on: one row for a
	// face channel, every face row of the entity for a family channel.
	rows map[channels.Anchor][]entity.Address
}

// collect maps the rows of a type the channel type attaches to onto their
// channel anchors: face "" for a family type, the row's face for a face
// type. It reports false when anchors is not empty and no row's type
// attaches.
func (r *busyRun) collect(anchors []BusyAnchor) bool {
	attached := false
	for _, a := range anchors {
		if !slices.Contains(r.ct.AttachTo, a.Type) {
			continue
		}
		attached = true
		ch := channels.Anchor{ID: a.Ref.ID}
		if r.ct.Anchor == realtimecfg.AnchorFace {
			ch.Face = a.Ref.Face
		}
		if _, seen := r.rows[ch]; !seen {
			r.anchors = append(r.anchors, ch)
		}
		addr := entity.AddressOf(a.Ref)
		if !slices.Contains(r.rows[ch], addr) {
			r.rows[ch] = append(r.rows[ch], addr)
		}
	}
	return attached || len(anchors) == 0
}

// names narrows the query to the channels the type's cardinality
// addresses: the unnamed channel for one, the named channels for many. A
// type switched between the two keeps the channels of its old
// cardinality. The service cannot address those, so they are not busy,
// and they must not count toward the ceiling.
func (r *busyRun) names() channels.NameFilter {
	if r.ct.Cardinality == realtimecfg.CardinalityMany {
		return channels.NamedOnly
	}
	return channels.UnnamedOnly
}

// ceiling is the row ceiling of the query: the most channels each anchor
// can store under [busyRun.names], one unnamed channel for cardinality one
// and [realtimelimits.MaxChannelsPerAnchor] named ones for many, clamped
// to [channels.MaxWaitingRows]. The query counts before the read gate, so
// the ceiling uses the hard limit, not the operator's channels_per_entity:
// no stored channel set reaches it, and a count the caller may not see
// never decides the answer. A full list page of 100 rows fits.
func (r *busyRun) ceiling() int {
	n := len(r.anchors)
	if r.ct.Cardinality == realtimecfg.CardinalityMany {
		n *= realtimelimits.MaxChannelsPerAnchor
	}
	return min(n, channels.MaxWaitingRows)
}

// prime reads the anchors of stored in one batch, with the properties the
// type's roles read, and primes the role answers for them once.
func (r *busyRun) prime(stored []channels.Channel) (map[entity.Address]AnchorView, error) {
	if len(stored) == 0 {
		return map[entity.Address]AnchorView{}, nil
	}
	var addrs []entity.Address
	seen := map[entity.Address]bool{}
	for _, ch := range stored {
		if addr := channelAddress(ch.Key); !seen[addr] {
			seen[addr] = true
			addrs = append(addrs, addr)
		}
	}
	c := r.c
	views, err := r.s.anchors.Anchors(c.ctx, addrs, roleProps(r.ct))
	if err != nil {
		return nil, fmt.Errorf("realtime: read anchors: %w", err)
	}
	ids := make([]string, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	slices.Sort(ids)
	c.ctx = c.acc.Prime(c.ctx, slices.Compact(ids))
	return views, nil
}

// entry judges one stored channel for the caller: the anchor was readable
// and of an attached type, the read role admits the caller, and the
// current session is in a declared state with status working.
//
// The query lists only declared working states, so a stranded session
// never reaches here; the state lookup below drops one anyway.
func (r *busyRun) entry(ch *channels.Channel, views map[entity.Address]AnchorView) (BusyView, bool) {
	c, ct := r.c, r.ct
	view, ok := views[channelAddress(ch.Key)]
	if !ok || !slices.Contains(ct.AttachTo, view.Type) {
		return BusyView{}, false
	}
	c.ct, c.anchor, c.key, c.head, c.ops = ct, view, ch.Key, ch, opGates{}
	admitted := roleAdmits(c, ch, ct.Read, realtimecfg.CapabilityRead)
	c.head = nil
	if !admitted {
		return BusyView{}, false
	}
	cur := currentSession(ct.Session(), *ch)
	st, ok := ct.Session().State(cur.State)
	if !ok || st.Status != realtimecfg.StatusWorking {
		return BusyView{}, false
	}
	return BusyView{
		Type: ct.Name, TypeLabel: ct.Label, Name: ch.Key.Name, State: st.Name, Label: st.Label,
		Since: cur.EnteredAt,
	}, true
}
