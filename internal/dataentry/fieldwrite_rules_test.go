package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// TestFieldWriteRulesMatchWire: the rule names the manager's field gate
// refuses with are the ones this package serves, so a denial from either
// path is the same 403.
func TestFieldWriteRulesMatchWire(t *testing.T) {
	for shared, wire := range map[affordances.FieldWriteRule]AffordanceDenialRule{
		affordances.RuleFieldHidden:       RuleFieldHidden,
		affordances.RuleFieldReadOnly:     RuleFieldReadOnly,
		affordances.RuleFieldEnumFiltered: RuleFieldEnumFiltered,
	} {
		if string(shared) != string(wire) {
			t.Errorf("shared rule %q != wire rule %q", shared, wire)
		}
	}
}
