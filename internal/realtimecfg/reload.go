package realtimecfg

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// CheckReload refuses a reload that would strand stored channels or expose
// what they hold: a change of the anchor or the cardinality of an existing
// channel type, or a ref property of one of its message types declared as
// another type. Channels are keyed by family or by face, and named or not,
// so either of the first two changes would orphan every stored channel of
// the type. A stored ref holds an entity id, which a property of another
// type would render as plain text to readers who may not see the entity.
// Adding or removing a channel type, a message type or a property is
// allowed.
//
// It compares two configurations only. A restart does not get around the
// rule: the channel store records each channel type's shape when its first
// channel is written, and boot and reload hold the config against that
// record through [ShapeMismatchError].
//
// Nil: prev and next are accepted; a nil value is a project without
// realtime.yaml.
func CheckReload(prev, next *Compiled) error {
	if prev == nil || next == nil {
		return nil
	}
	stored := make(map[string]TypeShape, len(prev.channelTypes))
	for name, ct := range prev.channelTypes {
		stored[name] = ct.Shape()
	}
	return ShapeMismatchError(ShapeMismatches(next, stored))
}

// TypeShape is the part of a channel type that decides how its stored
// channels are keyed and read: the anchor, the cardinality and the
// properties whose stored values are entity ids.
type TypeShape struct {
	Anchor Anchor
	// Cardinality is empty in a stored shape the channel store recorded
	// before it kept cardinality. An empty stored cardinality matches any
	// configured one.
	Cardinality Cardinality
	// Refs lists the ref properties of the type's message types, as
	// "message-type.property", sorted.
	Refs []string
}

// cardinalityChanged reports whether the stored s and the configured cur
// have different cardinalities. An empty stored cardinality matches.
func (s TypeShape) cardinalityChanged(cur TypeShape) bool {
	return s.Cardinality != "" && s.Cardinality != cur.Cardinality
}

// refName is the name of property prop of message type mt in
// [TypeShape.Refs].
func refName(mt, prop string) string { return mt + "." + prop }

// Shape returns the shape of ct.
func (ct ChannelType) Shape() TypeShape {
	refs, _ := ct.splitProperties()
	return TypeShape{Anchor: ct.Anchor, Cardinality: ct.Cardinality, Refs: refs}
}

// PlainProperties lists the declared properties of ct's message types that
// are not refs, in the form of [TypeShape.Refs].
func (ct ChannelType) PlainProperties() []string {
	_, plain := ct.splitProperties()
	return plain
}

func (ct ChannelType) splitProperties() (refs, plain []string) {
	for mtName, mt := range ct.MessageTypes {
		for name, p := range mt.Properties {
			if p.Type == PropertyRef {
				refs = append(refs, refName(mtName, name))
			} else {
				plain = append(plain, refName(mtName, name))
			}
		}
	}
	slices.Sort(refs)
	slices.Sort(plain)
	return refs, plain
}

// ShapeMismatch is a channel type whose configured shape differs from the
// shape its stored channels were written with.
type ShapeMismatch struct {
	ChannelType string
	Stored      TypeShape
	Configured  TypeShape
	// LostRefs lists the stored refs the configuration declares with
	// another property type, sorted.
	LostRefs []string
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
	if m.Stored.cardinalityChanged(m.Configured) {
		out = append(out, Problem{
			Key: "channel_types." + m.ChannelType + ".cardinality",
			Reason: fmt.Sprintf("cannot change from %s to %s: stored channels of this type were written "+
				"with cardinality %s, and migrating them to another cardinality is not supported yet",
				m.Stored.Cardinality, m.Configured.Cardinality, m.Stored.Cardinality),
		})
	}
	for _, ref := range m.LostRefs {
		mt, prop, _ := strings.Cut(ref, ".")
		out = append(out, Problem{
			Key: "channel_types." + m.ChannelType + ".message_types." + mt + ".properties." + prop + ".type",
			Reason: "cannot change from ref: stored messages hold entity ids in this property, which " +
				"another type would show as plain text to readers who may not see the entity",
		})
	}
	return out
}

// ShapeMismatches compares c against stored, the shape each channel type
// had when its channels were written (channel type name to shape), and
// returns every channel type whose anchor or cardinality changed, or one of
// whose stored refs c declares with another property type, sorted by name.
// A stored type, message type or property that c no longer declares is not
// a mismatch; adding it back is checked against the same record.
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
		cur := ct.Shape()
		lost := lostRefs(old.Refs, ct.PlainProperties())
		if cur.Anchor != old.Anchor || old.cardinalityChanged(cur) || len(lost) > 0 {
			out = append(out, ShapeMismatch{ChannelType: name, Stored: old, Configured: cur, LostRefs: lost})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChannelType < out[j].ChannelType })
	return out
}

// lostRefs returns the refs that plain, a sorted list, names.
func lostRefs(refs, plain []string) []string {
	var out []string
	for _, r := range refs {
		if _, found := slices.BinarySearch(plain, r); found {
			out = append(out, r)
		}
	}
	slices.Sort(out)
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
