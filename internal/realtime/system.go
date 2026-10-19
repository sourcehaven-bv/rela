package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// systemWriter makes the channel writes no request asks for. It is kept
// apart from [Service] so that no HTTP or MCP path can reach it.
type systemWriter struct {
	store channels.Store
	local *Local
	cfg   ConfigSource
}

// post appends a message of mt with props to key as [systemAuthor], naming
// triggeredBy. It charges no rate bucket: the trigger engine caps its own
// posts per round. With requireOpen the store refuses a channel that is
// not open with [channels.ErrNotOpen], in the same step as the append. A
// missing cardinality-one channel is created.
//
// The properties and the type's `when:` are checked again although
// realtimecfg checked both at load, because the message is the service's
// to keep valid whoever writes it.
func (w systemWriter) post(
	ctx context.Context, snap Snapshot, now time.Time, key channels.Key, ct realtimecfg.ChannelType,
	mt *realtimecfg.MessageType, props map[string]any, triggeredBy string, requireOpen bool,
) error {
	vp, err := validateProps(mt, props, snap.Config.Limits().MessageBytes)
	if err != nil {
		return err
	}
	if !whenHolds(ctx, mt, vp.props, now) {
		return fmt.Errorf("realtime: the message does not meet the when: condition of %s", mt.Name)
	}
	if recErr := recordType(ctx, w.store, w.local, w.cfg, snap, ct); recErr != nil {
		return recErr
	}
	_, err = w.store.Append(ctx, key, channels.NewMessage{
		Type: mt.Name, Author: systemAuthor(), At: now, Props: vp.props, TriggeredBy: triggeredBy,
		RequireOpen: requireOpen,
	})
	if err != nil {
		return err
	}
	w.local.mark(key)
	return nil
}
