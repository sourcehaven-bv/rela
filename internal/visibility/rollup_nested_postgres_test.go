//go:build postgres

package visibility_test

import "testing"

// The postgres twin of TestRollups_Nested: the outer frontier walk and
// each inner walk render as postgres SQL. It runs in its own schema
// pinned through the DSN.
func TestRollups_Nested_Postgres(t *testing.T) {
	t.Parallel()
	assertNestedRollups(t, openPageStore(t, "relanested"))
}
