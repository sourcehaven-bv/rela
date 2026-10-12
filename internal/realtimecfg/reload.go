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
// Nil: prev and next are accepted; a nil value is a project without
// realtime.yaml.
func CheckReload(prev, next *Compiled) error {
	if prev == nil || next == nil {
		return nil
	}
	var problems []Problem
	for name, old := range prev.channelTypes {
		cur, ok := next.channelTypes[name]
		if ok && cur.Anchor != old.Anchor {
			problems = append(problems, Problem{
				Key: "channel_types." + name + ".anchor",
				Reason: fmt.Sprintf("cannot change from %s to %s while the server runs; "+
					"stored channels are keyed by the anchor, so restart after migrating them", old.Anchor, cur.Anchor),
			})
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Slice(problems, func(i, j int) bool { return problems[i].Key < problems[j].Key })
	return &Error{Problems: problems}
}
