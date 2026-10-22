package realtimecfg

import "fmt"

// TypingName is the message type name the built-in typing indicator
// reserves. A channel type with `typing: true` may not declare a message
// type of that name.
const TypingName = "typing"

// typing checks a channel type that sets `typing: true`. Typing announces a
// log message to come, so the type needs a log message type, and the
// built-in indicator replaces an operator message type named [TypingName].
func (v *validator) typing(key string, raw *rawChannelType, ct *ChannelType) {
	hasLog := false
	for _, mt := range ct.MessageTypes {
		if mt.Persist == PersistLog {
			hasLog = true
			break
		}
	}
	if !hasLog && len(ct.MessageTypes) > 0 {
		v.add(key+".typing", "typing announces a message to come; declare a log message type or drop typing")
	}
	if _, ok := raw.MessageTypes[TypingName]; ok {
		v.add(ct.messageTypesKey()+"."+TypingName, fmt.Sprintf("%q is reserved in a channel type with "+
			"typing: true: the built-in indicator replaces it; remove this message type", TypingName))
	}
}
