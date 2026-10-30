package realtimecfg

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// refusedKeys are keys a reader might expect from the design notes or a later
// phase. Each is refused with a reason that says what to do instead, rather
// than as a bare unknown key. A "*" segment matches any map key.
var refusedKeys = []struct {
	pattern string
	reason  string
}{
	{"session", "a session belongs under a channel type: channel_types.<name>.session"},
	{"channel_types.*.session.final", "mark each final state with final: true"},
	{"channel_types.*.session.states.*.after.*.from", "use since: state or since: session"},
	{"channel_types.*.session.timers", "reminders are declared under session.reminders; a timer that " +
		"moves the session goes under after: on its state"},
	{"channel_types.*.session.states.*.reminders", "reminders are declared once, under session.reminders, with in:"},
	{"channel_types.*.session.reminders.*.to", "a reminder posts a message and does not move the session; " +
		"for a timer that moves it, use after: on the state"},
	{"channel_types.*.session.reminders.*.period", "use every"},
	{"channel_types.*.session.reminders.*.since", "a reminder counts from when the session entered one of " +
		"its in: states"},
	{"channel_types.*.session.states.*.outputs", "outputs belong to a transition or a timer: put them under on.<type>.outputs or " +
		"after.<name>.outputs"},
	{"channel_types.*.session.reminders.*.outputs", "a reminder posts a message only; let a guard on its count " +
		"fire a transition with outputs"},
	{"channel_types.*.session.states.*.on.*.run_as", "run_as is set once per session type, at session.run_as"},
	{"channel_types.*.session.states.*.on_final", "on_final belongs to the session: session.on_final: close " +
		"closes the channel on a move into any final state"},
	{"channel_types.*.session.states.*.default_in", "default_in belongs to the session: session.default_in: keyed"},
	{"channel_types.*.session.states.*.after.*.run_as", "run_as is set once per session type, at session.run_as"},
	{"channel_types.*.session.states.*.awaits", "use waiting_for: the roles whose answer the state waits for"},
	{"channel_types.*.session.status", "status is set per state: session.states.<name>.status"},
	{"channel_types.*.session.waiting_for", "waiting_for is set per state: session.states.<name>.waiting_for"},
	{"channel_types.*.session.states.*.on.*.waiting_for", "waiting_for belongs to the state the session " +
		"enters: set it on that state"},
	{"channel_types.*.session.states.*.after.*.waiting_for", "waiting_for belongs to the state the session " +
		"enters: set it on that state"},
	{"channel_types.*.message_types.*.states", "use in: the session states in which the type may be posted"},
	{"channel_types.*.message_types.*.from", "use in: the session states in which the type may be posted"},
	{"channel_types.*.message_types.*.quiet", "use unread: false to keep the type out of unread counts; " +
		"to fold or hide it in the web app, use display in data-entry.yaml"},
	{"channel_types.*.message_types.*.display", "display belongs in data-entry.yaml: " +
		"channels.<type>.messages.<mt>.display"},
	{"channel_types.*.message_types.*.decision_button.label", "the type's decision button reads as the " +
		"message type's label; for another text, add a button: to the entry"},
	{"channel_types.*.message_types.*.decision_button.props", "the type's decision button posts no preset; " +
		"for a preset, add a button: to the entry"},
	{"agents", "agents are ordinary principals; restrict them with client_baselines in acl.yaml " +
		"and match them with a principal_type role"},
	{"channel_types.*.message_types.*.actions.*.by", "agents are ordinary principals, so an action " +
		"cannot be limited to humans; restrict agent clients with client_baselines in acl.yaml"},
}

// maxKeyNodes bounds how many nodes [keyChecker] visits. The walk follows
// aliases, and an alias may point at a node that itself holds aliases, so
// a small file can expand exponentially (the "billion laughs" file). A real
// realtime.yaml has a few thousand nodes at most.
const maxKeyNodes = 100_000

// keyChecker walks the YAML tree beside the raw type it decodes into and
// reports every key the type does not declare, and every value whose shape
// (mapping, list or single value) does not match. yaml.v3's KnownFields
// reports the first unknown key by line only; this names each one by its key
// path, which is what the operator searches for.
type keyChecker struct {
	add    func(key, reason string)
	visits int
}

// checkKeys runs a [keyChecker] over n. It stops after [maxKeyNodes] nodes
// and reports the file as too large.
func checkKeys(n *yaml.Node, t reflect.Type, path string, add func(key, reason string)) {
	k := &keyChecker{add: add}
	k.walk(n, t, path)
	if k.visits > maxKeyNodes {
		add("", fmt.Sprintf("the file expands to more than %d nodes; remove nested aliases", maxKeyNodes))
	}
}

func (k *keyChecker) walk(n *yaml.Node, t reflect.Type, path string) {
	if n == nil || k.visits > maxKeyNodes {
		return
	}
	k.visits++
	add := k.add
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			k.walk(c, t, path)
		}
		return
	case yaml.AliasNode:
		k.walk(n.Alias, t, path)
		return
	default:
		// Sequences, mappings and scalars are checked against t below.
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return
	}
	if hasMergeKey(n) {
		add(path, mergeReason)
		return
	}
	if t == reflect.TypeFor[rawBranches]() {
		k.branches(n, path)
		return
	}
	if t == reflect.TypeFor[rawPresetValue]() {
		k.presetValue(n, path)
		return
	}
	if reason, ok := nodeScalars[t]; ok {
		if n.Kind != yaml.ScalarNode {
			k.add(path, reason)
		}
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			add(path, "must be a mapping")
			return
		}
		fields := yamlFields(t)
		for i := 0; i+1 < len(n.Content); i += 2 {
			name := n.Content[i].Value
			child := joinKey(path, name)
			ft, ok := fields[name]
			if !ok {
				add(child, unknownKeyReason(child))
				continue
			}
			k.walk(n.Content[i+1], ft, child)
		}
	case reflect.Map:
		if n.Kind != yaml.MappingNode {
			add(path, "must be a mapping")
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k.walk(n.Content[i+1], t.Elem(), joinKey(path, n.Content[i].Value))
		}
	case reflect.Slice:
		if n.Kind != yaml.SequenceNode {
			add(path, "must be a list")
			return
		}
		for i, c := range n.Content {
			k.walk(c, t.Elem(), fmt.Sprintf("%s[%d]", path, i))
		}
	default:
		k.scalar(n, t, path)
	}
}

// nodeScalars are the raw types that keep their YAML node for the
// validator to read, keyed to the reason a value that is not a single
// value is refused. The validator checks the value itself.
var nodeScalars = map[reflect.Type]string{
	reflect.TypeFor[rawConfirm](): "must be true, false or the text to show",
	reflect.TypeFor[rawUnread]():  "must be a single value: true or false",
}

// branches checks an `on.<message type>` value, which is one transition
// entry (a mapping) or a list of them: see [rawBranches].
func (k *keyChecker) branches(n *yaml.Node, path string) {
	entry := reflect.TypeFor[rawTransition]()
	switch n.Kind {
	case yaml.MappingNode:
		k.walk(n, entry, path)
	case yaml.SequenceNode:
		for i, c := range n.Content {
			k.walk(c, entry, fmt.Sprintf("%s[%d]", path, i))
		}
	default:
		k.add(path, "must be a mapping or a list of mappings")
	}
}

// presetValue checks one value of a button's preset: a single value or a
// list of single values. A null was already let through by the caller.
func (k *keyChecker) presetValue(n *yaml.Node, path string) {
	ok := n.Kind == yaml.ScalarNode
	if n.Kind == yaml.SequenceNode {
		ok = !slices.ContainsFunc(n.Content, func(c *yaml.Node) bool { return resolveNode(c).Kind != yaml.ScalarNode })
	}
	if !ok {
		k.add(path, "must be a single value or a list")
	}
}

// scalar checks a value whose Go type is neither a struct, a map nor a
// slice: it must be a single value, and a bool must decode as one.
func (k *keyChecker) scalar(n *yaml.Node, t reflect.Type, path string) {
	if n.Kind != yaml.ScalarNode {
		k.add(path, "must be a single value")
		return
	}
	// yaml.v3 itself decides what a boolean is, so the accepted set (true,
	// yes, on and the rest) is exactly what the decoder takes. The check
	// only names the key of a value it would refuse.
	if t.Kind() == reflect.Bool {
		var b bool
		if n.Decode(&b) != nil {
			k.add(path, "must be true or false")
		}
	}
}

// mergeReason refuses a YAML merge key. The file is checked key by key
// against its schema, and a merged entry would bypass that check.
const mergeReason = "merge keys (<<) are not supported in realtime.yaml; write the entries out"

// hasMergeKey reports whether n is a mapping with a YAML merge key.
func hasMergeKey(n *yaml.Node) bool {
	if n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i]; k.Tag == "!!merge" || k.Value == "<<" {
			return true
		}
	}
	return false
}

// yamlFields maps each YAML key of struct type t to its field type.
func yamlFields(t reflect.Type) map[string]reflect.Type {
	out := make(map[string]reflect.Type, t.NumField())
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name != "" && name != "-" {
			out[name] = f.Type
		}
	}
	return out
}

func joinKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// unknownKeyReason returns the refusal for an undeclared key: the specific
// reason when the key is a known refusal, else a plain unknown-key message.
func unknownKeyReason(path string) string {
	for _, r := range refusedKeys {
		if matchKeyPattern(r.pattern, path) {
			return r.reason
		}
	}
	return "unknown key"
}

func matchKeyPattern(pattern, path string) bool {
	ps, ks := strings.Split(pattern, "."), strings.Split(path, ".")
	if len(ps) != len(ks) {
		return false
	}
	for i := range ps {
		if ps[i] != "*" && ps[i] != ks[i] {
			return false
		}
	}
	return true
}
