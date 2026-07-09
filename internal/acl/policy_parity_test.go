package acl

import (
	"reflect"
	"strings"
	"testing"
)

// TestKnownPolicyKeysMatchStruct guards the same drift class as BUG-5XIN07 (the
// metamodel loader's whitelist), applied to acl.yaml: knownPolicyKeys is a
// hand-maintained duplicate of the Policy struct's top-level yaml tags (the
// declaration even carries a "keep in sync" comment). This asserts every
// top-level yaml tag on Policy is in knownPolicyKeys, so a newly-added field
// can't silently start emitting an unknown-key warning for valid config.
func TestKnownPolicyKeysMatchStruct(t *testing.T) {
	for field := range reflect.TypeFor[Policy]().Fields() {
		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue // computed / non-YAML field
		}
		key := strings.Split(tag, ",")[0]
		if key == "" {
			continue
		}
		if !knownPolicyKeys[key] {
			t.Errorf("Policy field %q (yaml:%q) is not in knownPolicyKeys — "+
				"LoadPolicy will warn on valid config that uses it", field.Name, key)
		}
	}
}
