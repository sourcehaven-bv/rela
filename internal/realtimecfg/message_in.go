package realtimecfg

import (
	"fmt"
	"slices"
	"strings"
)

// messageTypeKey is the key path of message type mt of ct.
func (ct *ChannelType) messageTypeKey(mt string) string { return ct.messageTypesKey() + "." + mt }

// messageInCheck compiles the `in:` key of the message types of one
// channel type: raw is the type as written, ct the type compiled so far.
type messageInCheck struct {
	v   *validator
	raw *rawChannelType
	ct  *ChannelType
}

// withoutSession refuses `in:` on every message type of a channel type
// without a session: the key names session states.
func (m messageInCheck) withoutSession() {
	for _, mn := range sortedKeys(m.raw.MessageTypes) {
		if present, _ := keyPresence(m.v.root, "in", "channel_types", m.ct.Name, "message_types", mn); present {
			m.v.add(m.ct.messageTypeKey(mn)+".in", "in names session states; this channel type has no session")
		}
	}
}

// withSession compiles `in:` of every message type of a channel type
// with session s. It runs after the states, transitions and reminders
// compile, so it can check them against the list: a state whose `on:` is
// keyed by the type, and every state of a reminder that posts the type,
// must be listed. A type missing from the compiled channel type is
// skipped: there is nothing to set its `in:` on.
func (m messageInCheck) withSession(s *Session, rawStates map[string]rawSessionState) {
	for _, mn := range sortedKeys(m.raw.MessageTypes) {
		mt := m.ct.MessageTypes[mn]
		if mt == nil {
			continue
		}
		if in, ok := m.one(mn, s, rawStates); ok {
			mt.In = in
		}
	}
}

// one checks the `in:` of message type mn and returns it: nil when the
// type has no `in:`. ok is false when it has a problem.
func (m messageInCheck) one(mn string, s *Session, rawStates map[string]rawSessionState) (in []string, ok bool) {
	v, ct := m.v, m.ct
	present, null := keyPresence(v.root, "in", "channel_types", ct.Name, "message_types", mn)
	if !present {
		return nil, false
	}
	raw := m.raw.MessageTypes[mn].In
	key := ct.messageTypeKey(mn) + ".in"
	switch {
	case null:
		v.add(key, "name the states, or drop in to allow every state")
		return nil, false
	case len(raw) == 0:
		v.add(key, "in names no state, so the type could never be posted; drop in to allow every state")
		return nil, false
	}
	ok = true
	for i, st := range raw {
		ik := fmt.Sprintf("%s[%d]", key, i)
		switch _, declared := s.index[st]; {
		case !declared:
			v.add(ik, unknownState(st))
			ok = false
		case slices.Contains(in, st):
			v.add(ik, fmt.Sprintf("state %s is listed twice", st))
			ok = false
		default:
			in = append(in, st)
		}
	}
	list := strings.Join(raw, ", ")
	for _, st := range s.states {
		if _, keyed := rawStates[st.Name].On[mn]; keyed && !slices.Contains(raw, st.Name) {
			v.add(fmt.Sprintf("channel_types.%s.session.states.%s.on.%s", ct.Name, st.Name, mn),
				fmt.Sprintf("%s may be posted only in %s, so this transition could never fire; "+
					"add %s to message_types.%s.in", mn, list, st.Name, mn))
			ok = false
		}
	}
	for _, r := range s.reminders {
		if r.PostType != mn {
			continue
		}
		for i, st := range r.in {
			if !slices.Contains(raw, st) {
				v.add(fmt.Sprintf("channel_types.%s.session.reminders.%s.in[%d]", ct.Name, r.Name, i),
					fmt.Sprintf("the reminder posts %s, which may be posted only in %s", mn, list))
				ok = false
			}
		}
	}
	if ok && s.onFinal == OnFinalClose && allFinal(s, in) {
		v.add(key, "the channel closes when the session enters these states, so the type would be "+
			"offered at most until the close runs")
		ok = false
	}
	return in, ok
}

// allFinal reports whether every one of states is a final state of s.
func allFinal(s *Session, states []string) bool {
	for _, name := range states {
		if st, ok := s.State(name); !ok || !st.Final {
			return false
		}
	}
	return true
}
