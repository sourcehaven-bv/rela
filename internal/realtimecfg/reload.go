package realtimecfg

import (
	"fmt"
	"sort"
)

// CheckReload refuses a reload that would strand stored channels. Today
// that is one change: the anchor of an existing channel type, because
// channels are keyed by family or by face and a change would orphan every
// stored channel of the type. Adding or removing a channel type is allowed.
//
// It compares two configurations only. A restart does not get around the
// rule: the channel store records each channel type's anchor, and
// [AnchorMismatches] holds a booting config against that record.
//
// Nil: prev and next are accepted; a nil value is a project without
// realtime.yaml.
func CheckReload(prev, next *Compiled) error {
	if prev == nil || next == nil {
		return nil
	}
	stored := make(map[string]Anchor, len(prev.channelTypes))
	for name, ct := range prev.channelTypes {
		stored[name] = ct.Anchor
	}
	return AnchorMismatchError(AnchorMismatches(next, stored))
}

// AnchorMismatch is a channel type whose configured anchor differs from the
// anchor its stored channels were keyed by.
type AnchorMismatch struct {
	ChannelType string
	Stored      Anchor
	Configured  Anchor
}

// Problem renders the mismatch as a load problem on the anchor key.
func (m AnchorMismatch) Problem() Problem {
	return Problem{
		Key: "channel_types." + m.ChannelType + ".anchor",
		Reason: fmt.Sprintf("cannot change from %s to %s: stored channels of this type are keyed by "+
			"the %s anchor, and migrating them to another anchor is not supported yet",
			m.Stored, m.Configured, m.Stored),
	}
}

// AnchorMismatches compares c against stored, the anchor each channel type
// had when its channels were written (channel type name to anchor), and
// returns every channel type whose anchor changed, sorted by name. A stored
// type that c no longer declares is not a mismatch; adding it back is
// checked against the same record.
//
// The boot path calls it with the store's record and refuses to start on
// any mismatch.
//
// Nil: c and stored are accepted and yield no mismatches.
func AnchorMismatches(c *Compiled, stored map[string]Anchor) []AnchorMismatch {
	if c == nil {
		return nil
	}
	var out []AnchorMismatch
	for name, old := range stored {
		cur, ok := c.channelTypes[name]
		if ok && cur.Anchor != old {
			out = append(out, AnchorMismatch{ChannelType: name, Stored: old, Configured: cur.Anchor})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChannelType < out[j].ChannelType })
	return out
}

// AnchorMismatchError returns an [*Error] listing ms, or nil when ms is
// empty.
func AnchorMismatchError(ms []AnchorMismatch) error {
	if len(ms) == 0 {
		return nil
	}
	problems := make([]Problem, len(ms))
	for i, m := range ms {
		problems[i] = m.Problem()
	}
	return &Error{Problems: problems}
}
