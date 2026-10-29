package realtimecfg

import "fmt"

// Button limits behind the load warning on `choices.from`. A list longer
// than this, or with longer items, loads, but its buttons crowd the
// message.
const (
	buttonItems = 10
	buttonRunes = 100
)

// choiceChecker checks `choices:` on a validator's behalf. It keeps those
// checks off the validator, which is at its method budget.
type choiceChecker struct{ *validator }

// choices compiles `choices:` on every message type of ct. It runs after
// the message types and the session are compiled, because it reads the
// answer type and the session's transitions.
func (v choiceChecker) choices(raw *rawChannelType, ct *ChannelType) {
	for _, mn := range sortedKeys(raw.MessageTypes) {
		rc := raw.MessageTypes[mn].Choices
		mt := ct.MessageTypes[mn]
		if rc == nil || mt == nil {
			continue
		}
		key := ct.messageTypesKey() + "." + mn + ".choices"
		if mt.Persist == PersistEphemeral {
			v.add(key, "an ephemeral message is never stored, so it cannot carry choices")
			continue
		}
		from, fromOK := v.choicesFrom(key+".from", rc.From, mt)
		answer := v.choicesAnswer(key+".answer", rc, mt, ct)
		if answer != nil && fromOK {
			v.choicesInto(key+".into", rc.Into, from, answer)
		}
		mt.Choices = &Choices{From: rc.From, Answer: rc.Answer, Into: rc.Into, Other: rc.Other}
		if answer != nil && ct.session != nil && !choicesReachable(ct.session, mt, answer) {
			v.warn(key+".answer", fmt.Sprintf("in a channel with a session, buttons show only on the %s "+
				"message that moved the session, and no transition on %s leads to a state where %s may be "+
				"posted; the buttons never show", mn, mn, answer.Name))
		}
	}
}

// choicesFrom checks that name is a list property of the question type
// mt and returns it. It warns when the list is too long or its items too
// long for buttons.
func (v choiceChecker) choicesFrom(key, name string, mt *MessageType) (Property, bool) {
	if name == "" {
		v.add(key, "name the list property that holds the options")
		return Property{}, false
	}
	p, ok := mt.Properties[name]
	switch {
	case !ok:
		v.add(key, fmt.Sprintf("message type %s has no property %q", mt.Name, name))
		return Property{}, false
	case !p.List:
		v.add(key, fmt.Sprintf("property %s is a %s property; the options are a list, so declare it with "+
			"type: string and list: true", name, p.TypeName()))
		return Property{}, false
	}
	if p.MaxItems > buttonItems || p.Max > buttonRunes {
		v.warn(key, fmt.Sprintf("property %s allows %d items of up to %d characters; that is too many or too "+
			"long for buttons, so keep max_items at most %d and max at most %d", name, p.MaxItems, p.Max,
			buttonItems, buttonRunes))
	}
	return p, true
}

// choicesAnswer checks that the answer type is another log type of ct
// whose only required property, if any, is the into property. It returns
// the answer type, or nil when it is unusable.
func (v choiceChecker) choicesAnswer(key string, rc *rawChoices, question *MessageType, ct *ChannelType) *MessageType {
	if rc.Answer == "" {
		v.add(key, "name the message type a click posts")
		return nil
	}
	answer, ok := ct.MessageTypes[rc.Answer]
	switch {
	case !ok:
		v.add(key, fmt.Sprintf("channel type %s has no message type %q", ct.Name, rc.Answer))
		return nil
	case answer.Name == question.Name:
		v.add(key, "the answer must be another message type; a question cannot answer itself")
		return nil
	case answer.Persist == PersistEphemeral:
		v.add(key, fmt.Sprintf("message type %s is ephemeral; an answer is a reply, and only a log message "+
			"can reply", rc.Answer))
		return nil
	}
	for _, pn := range sortedKeys(answer.Properties) {
		if answer.Properties[pn].Required && pn != rc.Into {
			v.add(key, fmt.Sprintf("message type %s requires property %q; a click fills only %q, so the "+
				"answer type may require no other property", rc.Answer, pn, rc.Into))
		}
	}
	return answer
}

// choicesInto checks that name is a string property of the answer type
// that holds every option of from.
func (v choiceChecker) choicesInto(key, name string, from Property, answer *MessageType) {
	if name == "" {
		v.add(key, "name the property of the answer type that receives the option")
		return
	}
	p, ok := answer.Properties[name]
	switch {
	case !ok:
		v.add(key, fmt.Sprintf("message type %s has no property %q", answer.Name, name))
	case p.List || p.Type != PropertyString:
		// A markdown into would credit the clicker with markup the asker
		// chose, such as a disguised link or a mention.
		v.add(key, fmt.Sprintf("property %s is a %s property; an option goes into a string property",
			name, p.TypeName()))
	case p.Max > 0 && p.Max < from.Max:
		v.add(key, fmt.Sprintf("an option of up to %d characters does not fit into %s, which holds at most %d",
			from.Max, name, p.Max))
	}
}

// choicesReachable reports whether a question of type question can move
// the session into a state where answer may be posted. In a channel with
// a session, buttons show only on the question that moved it, so without
// such a state they never show.
//
// It over-approximates: it counts every entry of `on.<question>`, guarded
// or not, and ignores their order, so an entry after an unguarded one
// counts although it can never fire. That can only hide the warning, never
// refuse a config.
func choicesReachable(s *Session, question, answer *MessageType) bool {
	for _, st := range s.States() {
		if !question.PostableIn(st.Name) {
			continue
		}
		for _, t := range s.Branches(st.Name, question.Name) {
			if answer.PostableIn(t.To) {
				return true
			}
		}
	}
	return false
}
