package realtimecfg

import (
	"fmt"
	"reflect"
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
	{"session", "sessions are not supported yet"},
	{"channel_types.*.session", "sessions are not supported yet"},
	{"agents", "agents are ordinary principals; restrict them with client_baselines in acl.yaml " +
		"and match them with a principal_type role"},
	{"channel_types.*.message_types.*.actions.*.by", "agents are ordinary principals, so an action " +
		"cannot be limited to humans; restrict agent clients with client_baselines in acl.yaml"},
}

// checkKeys walks the YAML tree beside the raw type it decodes into and
// reports every key the type does not declare, and every value whose shape
// (mapping, list or single value) does not match. yaml.v3's KnownFields
// reports the first unknown key by line only; this names each one by its key
// path, which is what the operator searches for.
func checkKeys(n *yaml.Node, t reflect.Type, path string, add func(key, reason string)) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			checkKeys(c, t, path, add)
		}
		return
	case yaml.AliasNode:
		checkKeys(n.Alias, t, path, add)
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
			checkKeys(n.Content[i+1], ft, child, add)
		}
	case reflect.Map:
		if n.Kind != yaml.MappingNode {
			add(path, "must be a mapping")
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			checkKeys(n.Content[i+1], t.Elem(), joinKey(path, n.Content[i].Value), add)
		}
	case reflect.Slice:
		if n.Kind != yaml.SequenceNode {
			add(path, "must be a list")
			return
		}
		for i, c := range n.Content {
			checkKeys(c, t.Elem(), fmt.Sprintf("%s[%d]", path, i), add)
		}
	default:
		if n.Kind != yaml.ScalarNode {
			add(path, "must be a single value")
		}
	}
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
