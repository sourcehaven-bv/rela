//go:build !postgres

package backendtest

import "github.com/Sourcehaven-BV/rela/internal/appbuild"

// buildName labels this build in skip and failure messages.
const buildName = "filesystem/memory/sqlite"

// Options returns no options: the fs, memory and sqlite recipes each open a
// store from the project directory alone, so there is nothing to supply and
// nothing that can be unavailable. These builds never skip.
//
// sqlite belongs here despite having a real database, because the thing this
// file abstracts is whether a test must be GIVEN a connection — and it must
// not: the sqlite recipe creates .rela/rela.db under the project root by
// itself. What sqlite does NOT share with fs and memory is where entities come
// from, since it reads rows rather than the markdown files a test wrote. A
// test whose fixture is hand-written entity files needs its own exclusion
// (BUG-LL3C07); this seam cannot express that, and does not try to.
func Options(_ TB) []appbuild.Option { return nil }

// Env returns no environment overrides, for the same reason.
func Env(_ TB) map[string]string { return nil }

// DSN returns "": none of these builds takes an external database, and
// Config.DatabaseURL is ignored by their recipes.
func DSN(_ TB) string { return "" }
