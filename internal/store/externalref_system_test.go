package store_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestValidSystemNameMatchesTagGrammar pins the metamodel's `system:`
// check to the tag grammar: a system loads exactly when sync/<system> is a
// valid version tag name (TKT-SM20FG).
func TestValidSystemNameMatchesTagGrammar(t *testing.T) {
	for _, system := range []string{
		"basecamp", "a", "0x", "a.b_c-d", strings.Repeat("a", 63), strings.Repeat("a", 64),
		"Basecamp", "-a", ".a", "_a", "a/b", "a b", "a\"b", "é",
	} {
		_, tagErr := store.ParseVersionTagName("sync/" + system)
		schema := "version: \"1.0\"\nentities:\n  ticket:\n    label: Ticket\n    id_prefix: \"T-\"\n" +
			"    properties:\n      ref:\n        type: external_ref\n        system: \"" +
			strings.ReplaceAll(system, `"`, `\"`) + "\"\n"
		_, loadErr := metamodel.Parse([]byte(schema))
		if (tagErr == nil) != (loadErr == nil) {
			t.Errorf("system %q: tag error %v, load error %v", system, tagErr, loadErr)
		}
	}
}
