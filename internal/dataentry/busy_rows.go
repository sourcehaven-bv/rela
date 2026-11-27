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

// serveRowBusy sets the busy channels of channelType on each row in data, which
// is parallel to rows, with one busy read per page. A served read gives every
// row a non-nil slice. An unserved or failed read sets no `_busy` and never
// fails the list; only a failure is logged. The busy read re-gates each
// channel, so the answer is per principal and never cached. Served and failed
// reads record the replay wake (recordBusy); an unserved read records none.
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
