package filechannels

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// GetMessages implements [channels.Store]. Per named channel it reads the
// meta.yaml once and each segment that holds a named seq once. A damaged
// meta.yaml skips the channel, and a damaged record is skipped; both are
// reported through the store's [channels.DamageWarner]. A read error, or
// ctx ending, fails the call; ctx is checked before each channel.
func (s *Store) GetMessages(
	ctx context.Context, refs []channels.MessageRef,
) (map[channels.MessageRef]channels.Message, error) {
	if err := channels.CheckGetMessages(refs); err != nil {
		return nil, err
	}
	keys, seqs := channels.GroupRefs(refs)
	out := make(map[channels.MessageRef]channels.Message, len(refs))
	for _, k := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := channelMessages(&s.disk, k, seqs[k], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// channelMessages adds the messages of live channel k at seqs to out.
func channelMessages(d *disk, k channels.Key, seqs []int64, out map[channels.MessageRef]channels.Message) error {
	data, err := d.readFile(chanDir(k) + "/" + metaFile)
	if err != nil {
		return fmt.Errorf("filechannels: read %s: %w", k, err)
	}
	if data == nil {
		return nil
	}
	if _, err := parseMeta(data, k); err != nil {
		d.damaged.Warn(k, time.Now(), "filechannels: skipping a channel whose meta.yaml cannot be read", err)
		return nil
	}
	segs := make(map[int64][]channels.Message)
	damaged := 0
	defer func() {
		if damaged > 0 {
			d.damaged.Warn(k, time.Now(), "filechannels: skipping damaged records",
				fmt.Errorf("%d damaged records in the segments read", damaged))
		}
	}()
	for _, seq := range seqs {
		idx := d.idx(seq)
		msgs, ok := segs[idx]
		if !ok {
			raw, err := d.segment(k, idx)
			if err != nil {
				return err
			}
			msgs = foldWith(raw, func() { damaged++ })
			segs[idx] = msgs
		}
		if i := find(msgs, seq); i >= 0 {
			out[channels.MessageRef{Key: k, Seq: seq}] = msgs[i]
		}
	}
	return nil
}
