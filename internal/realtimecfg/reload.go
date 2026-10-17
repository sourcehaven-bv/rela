package realtimecfg

import (
	"fmt"
	"sort"
)

// CheckReload refuses a reload that would strand stored channels: a change
// of the anchor or the cardinality of an existing channel type. Channels
// are keyed by family or by face, and named or not, so either change would
// orphan every stored channel of the type. Adding or removing a channel
// type is allowed.
//
// It compares two configurations only. A restart does not get around the
// rule: the channel store records each channel type's anchor and
// cardinality when its first channel is written, and boot holds the config
// against that record through [ShapeMismatchError].
//
// Nil: prev and next are accepted; a nil value is a project without
// realtime.yaml.
func CheckReload(prev, next *Compiled) error {
	if prev == nil || next == nil {
		return nil
	}
	stored := make(map[string]TypeShape, len(prev.channelTypes))
	for name, ct := range prev.channelTypes {
		stored[name] = TypeShape{Anchor: ct.Anchor, Cardinality: ct.Cardinality}
	}
	return ShapeMismatchError(ShapeMismatches(next, stored))
}

// TypeShape is the part of a channel type that decides how its stored
// channels are keyed: the anchor and the cardinality.
type TypeShape struct {
	Anchor      Anchor
	Cardinality Cardinality
}

// ShapeMismatch is a channel type whose configured shape differs from the
// shape its stored channels were keyed by.
type ShapeMismatch struct {
	ChannelType string
	Stored      TypeShape
	Configured  TypeShape
}

// Problems renders the mismatch as one load problem per changed key.
func (m ShapeMismatch) Problems() []Problem {
	var out []Problem
	if m.Stored.Anchor != m.Configured.Anchor {
		out = append(out, Problem{
			Key: "channel_types." + m.ChannelType + ".anchor",
			Reason: fmt.Sprintf("cannot change from %s to %s: stored channels of this type are keyed by "+
				"the %s anchor, and migrating them to another anchor is not supported yet",
				m.Stored.Anchor, m.Configured.Anchor, m.Stored.Anchor),
		})
	}
	if m.Stored.Cardinality != m.Configured.Cardinality {
		out = append(out, Problem{
			Key: "channel_types." + m.ChannelType + ".cardinality",
			Reason: fmt.Sprintf("cannot change from %s to %s: stored channels of this type were written "+
				"with cardinality %s, and migrating them to another cardinality is not supported yet",
				m.Stored.Cardinality, m.Configured.Cardinality, m.Stored.Cardinality),
		})
	}
	return out
}

// ShapeMismatches compares c against stored, the shape each channel type
// had when its channels were written (channel type name to shape), and
// returns every channel type whose anchor or cardinality changed, sorted by
// name. A stored type that c no longer declares is not a mismatch; adding
// it back is checked against the same record.
//
// Nil: c and stored are accepted and yield no mismatches.
func ShapeMismatches(c *Compiled, stored map[string]TypeShape) []ShapeMismatch {
	if c == nil {
		return nil
	}
	var out []ShapeMismatch
	for name, old := range stored {
		ct, ok := c.channelTypes[name]
		if !ok {
			continue
		}
		cur := TypeShape{Anchor: ct.Anchor, Cardinality: ct.Cardinality}
		if cur != old {
			out = append(out, ShapeMismatch{ChannelType: name, Stored: old, Configured: cur})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChannelType < out[j].ChannelType })
	return out
}

// ShapeMismatchError returns an [*Error] listing the problems of ms, or nil
// when ms is empty.
func ShapeMismatchError(ms []ShapeMismatch) error {
	var problems []Problem
	for _, m := range ms {
		problems = append(problems, m.Problems()...)
	}
	if len(problems) == 0 {
		return nil
	}
	return &Error{Problems: problems}
}
