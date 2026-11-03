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

// defaultInKeyed is the one value of `session.default_in`: a message type
// that a transition is keyed by, and that has no `in:`, may be posted only
// in the states whose `on:` keys it.
const defaultInKeyed = "keyed"

// checkDefaultIn checks `session.default_in` at key and returns it, or
// empty when it is absent or has a problem.
func checkDefaultIn(add problemSink, key, value string) string {
	if value != "" && value != defaultInKeyed {
		add(key, "default_in may only be keyed")
		return ""
	}
	return value
}

// withSession compiles `in:` of every message type of a channel type
// with session s. It runs after the states, transitions and reminders
// compile, so it can check them against the list: a state whose `on:` is
// keyed by the type, and every state of a reminder that posts the type,
// must be listed. A type missing from the compiled channel type is
// skipped: there is nothing to set its `in:` on.
//
// rawDefaultIn is `session.default_in` as written. When it is keyed, a
// type without `in:` that has at least one keyed state gets In set to
// those states, in the order of session.states. The load warns when that
// gives no type an In.
//
// A derived In skips two checks that [messageInCheck.one] runs on an
// authored `in:`, because neither can fail on a config that loads. Every
// state of a reminder that posts the type must be listed, but a reminder
// may not post a type a transition is keyed by. Under on_final: close the
// list may not hold only final states, but a final state has no `on:`, so
// it is never a keyed state. Each of those configs is refused at its own
// key; TestDefaultIn_DerivedSkipsChecksThatCannotFire pins both.
func (m messageInCheck) withSession(s *Session, rawStates map[string]rawSessionState, rawDefaultIn string) {
	key := "channel_types." + m.ct.Name + ".session.default_in"
	defaultIn := checkDefaultIn(m.v.add, key, rawDefaultIn)
	keyedTypes, derived := 0, 0
	for _, mn := range sortedKeys(m.raw.MessageTypes) {
		mt := m.ct.MessageTypes[mn]
		if mt == nil {
			continue
		}
		keyed := keyedStates(s, rawStates, mn)
		if len(keyed) > 0 {
			keyedTypes++
		}
		present, null := keyPresence(m.v.root, "in", "channel_types", m.ct.Name, "message_types", mn)
		switch {
		case present:
			if in, ok := m.one(mn, null, s, rawStates, defaultIn); ok {
				mt.In = in
			}
		case defaultIn == defaultInKeyed && len(keyed) > 0:
			mt.In, mt.derived = keyed, true
			derived++
		}
	}
	if defaultIn != defaultInKeyed || derived > 0 {
		return
	}
	if keyedTypes == 0 {
		m.v.warn(key, "default_in: keyed has no effect, because no transition is keyed by a message type; "+
			"drop default_in")
		return
	}
	m.v.warn(key, "default_in: keyed has no effect, because every message type a transition is keyed by "+
		"declares in:; drop the in: lists that only repeat the on: keys, or drop default_in")
}

// keyedStates returns the states of s whose `on:` is keyed by message type
// mn, in declaration order, whatever the entries say.
func keyedStates(s *Session, rawStates map[string]rawSessionState, mn string) []string {
	var keyed []string
	for _, st := range s.states {
		if _, ok := rawStates[st.Name].On[mn]; ok {
			keyed = append(keyed, st.Name)
		}
	}
	return keyed
}

// one checks the `in:` of message type mn, which the file writes (null
// when its value is null), and returns it. ok is false when it has a
// problem. defaultIn is the session's default_in, which the refusal of a
// missing keyed state mentions.
func (m messageInCheck) one(
	mn string, null bool, s *Session, rawStates map[string]rawSessionState, defaultIn string,
) (in []string, ok bool) {
	v, ct := m.v, m.ct
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
	drop := ""
	if defaultIn == defaultInKeyed {
		drop = ", or drop in to take the keyed states from session.default_in"
	}
	for _, st := range keyedStates(s, rawStates, mn) {
		if !slices.Contains(raw, st) {
			v.add(fmt.Sprintf("channel_types.%s.session.states.%s.on.%s", ct.Name, st, mn),
				fmt.Sprintf("%s may be posted only in %s, so this transition could never fire; "+
					"add %s to message_types.%s.in%s", mn, list, st, mn, drop))
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
