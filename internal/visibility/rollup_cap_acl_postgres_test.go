//go:build postgres

package visibility_test

import "testing"

// The postgres twin of TestRollups_CapACL. It runs in its own schema
// pinned through the DSN.
func TestRollups_CapACL_Postgres(t *testing.T) {
	t.Parallel()
	assertCapACL(t, openPageStore(t, "relacapacl"))
}
