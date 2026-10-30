package dataentry

import (
	"context"
	"log/slog"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// busyParam asks a collection response to carry each row's busy channels
// of one channel type (TKT-21G6JS). Off by default: a kanban card with a
// `busy:` field or a list with a busy column (TKT-TU3XH4) asks for it.
const busyParam = "busy"

// busyReader answers the busy channels of one type on a page of rows.
// realtime.BusyReader satisfies it.
type busyReader interface {
	Busy(ctx context.Context, channelType string, anchors []realtime.BusyAnchor) (
		map[entityPkg.Address][]realtime.BusyView, bool, error)
}

// wantBusy returns the channel type the request asks busy state for, or
// "". As for comment_counts, the first value counts.
func wantBusy(query map[string][]string) string {
	return queryGet(query, busyParam)
}

// serveRowBusy sets the busy channels of channelType on each row in data,
// which is parallel to rows. It makes one busy read for the page. When the
// read is served, every row gets a non-nil slice, so a row with nothing
// busy carries `[]`. When it is not served, or fails, no row carries
// `_busy`: the field decorates a list, so it must not fail the list. A
// failure is logged; a read that is not served for a reason of
// configuration or identity is not.
//
// The rows have already passed the read gate and redaction. The busy read
// gates each channel again, as a channel read would, so the answer is per
// principal and is never cached. A served read, and a read that failed
// with a store error, record the live wake of a replay (recordBusy), from
// the same rows. A read that is not served records none: a reload or a
// graph write must change it, and both start a full round.
//
// Nil: b is accepted and means channels are not wired; nothing is set.
func serveRowBusy(
	ctx context.Context, b busyReader, channelType string, rows []*entityPkg.Entity, data []v1.Entity,
) {
	if b == nil || channelType == "" {
		return
	}
	anchors := make([]realtime.BusyAnchor, len(rows))
	for i, e := range rows {
		anchors[i] = realtime.BusyAnchor{Ref: e.Ref(), Type: e.Type}
	}
	busy, ok, err := b.Busy(ctx, channelType, anchors)
	if err != nil {
		slog.WarnContext(ctx, "dataentry: reading busy channels failed; _busy omitted",
			"channel_type", channelType, "err", err)
		// The wake comes from the gated rows and the type, not from the
		// answer, so it holds after a store error. Without it the board
		// would leave the live wakes and the safety replay until a full
		// round, which on a quiet system can be hours.
		recordBusy(ctx, channelType, rows)
		return
	}
	if !ok {
		return
	}
	recordBusy(ctx, channelType, rows)
	for i, e := range rows {
		vs := busy[entityPkg.AddressOf(e.Ref())]
		out := make([]v1.BusyChannel, 0, len(vs))
		for _, v := range vs {
			out = append(out, v1.BusyChannel{
				Type: v.Type, TypeLabel: v.TypeLabel, Name: v.Name, State: v.State, Label: v.Label,
				Since: channelTime(v.Since),
			})
		}
		data[i].Busy = out
	}
}

// channelsBusy returns the busy read of h, or nil when channels are not
// wired. A function, so channelsHandler gains no method.
func channelsBusy(h *channelsHandler) busyReader {
	if h == nil || h.busy == nil {
		return nil
	}
	return h.busy
}
