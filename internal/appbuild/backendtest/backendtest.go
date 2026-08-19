// Package backendtest supplies the backend a build-agnostic wiring test needs.
//
// Tests in `internal/appbuild` and `internal/cli` assert properties of the
// composition root that hold for EVERY backend: that Services comes back with
// every field populated, that Close is idempotent, that acl.yaml loads into a
// Declarative, that a corrupt CalDAV cache does not brick boot. None of them
// care which store is underneath — they call New/Discover because that is the
// wiring entry point, not because they are testing a store.
//
// On the default and memorybackend builds that is free: the recipe opens an
// fsstore or memstore from the project directory the test just wrote. The
// postgres recipe cannot — it requires a DSN and a reachable server, so every
// one of those tests failed under `-tags postgres` with "postgres build
// requires a database URL". The assertions were fine; they simply had no
// backend to make.
//
// This package is the seam. [Options] returns whatever the current build needs
// to reach a usable store: nothing at all for fs/memory, and for postgres a
// DSN pointing at a freshly-created, migrated, per-test schema. A test calls it
// and stays backend-agnostic.
//
// # Skip policy
//
// When the postgres build has no database the tests SKIP rather than fail. That
// is a deliberate trade with a known cost: a developer running
// `go test -tags postgres ./...` without a server gets a clean run, but a skip
// and a pass are indistinguishable in exit codes and CI summaries
// (RR-0EWZQW — the pgstore suite has the same shape and the same hazard).
//
// The mitigation is identical to pgstore's, and reuses its variable rather than
// inventing a second one: when RELA_TEST_DATABASE_REQUIRED is set, a missing
// DSN is a hard failure instead of a skip. CI's Postgres Backend job sets it,
// so the gate cannot silently disarm there — which is the environment whose job
// is to prove the postgres backend works.
package backendtest

import (
	"os"
	"strings"
)

// testDBEnv names the DSN the postgres build's wiring tests connect with. It is
// deliberately the SAME variable the pgstore suite uses: one database, one
// setup step, one thing to configure.
const testDBEnv = "RELA_TEST_DATABASE_URL"

// requireDBEnv turns the skip below into a failure. See the package doc; the
// semantics mirror internal/store/pgstore's requireDBEnv exactly, because an
// environment that promises a database should get the same strictness from
// every suite that needs one.
const requireDBEnv = "RELA_TEST_DATABASE_REQUIRED"

// TB is the subset of *testing.T the helpers use. Taking an interface rather
// than *testing.T keeps this package free of a testing import in its signature
// and lets a caller pass a *testing.B or a wrapper.
type TB interface {
	Helper()
	Skipf(format string, args ...any)
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// dsnRequired reports whether an absent DSN must fail rather than skip.
// A variable set to "" / "0" / "false" does not opt in, so a shell that exports
// it empty behaves like an unset one instead of hard-failing every suite.
func dsnRequired(getenv func(string) string) bool {
	v := getenv(requireDBEnv)
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// missingDSN raises the right signal for an absent DSN: a hard failure when the
// environment promised a database, a skip otherwise.
func missingDSN(tb TB, getenv func(string) string) {
	tb.Helper()
	if dsnRequired(getenv) {
		tb.Fatalf(
			"%s is set, so the %s wiring tests must run, but %s is empty.\n"+
				"These tests assert composition-root behavior on the postgres build; "+
				"skipping them silently would let a postgres-only wiring regression merge.\n"+
				"Set %s to a reachable PostgreSQL DSN, or unset %s to allow skipping.",
			requireDBEnv, buildName, testDBEnv, testDBEnv, requireDBEnv)
		return
	}
	tb.Skipf("%s not set; skipping %s wiring tests", testDBEnv, buildName)
}

// Getenv is the environment accessor the helpers read through. It exists so the
// resolution rules above are testable without mutating process state — the same
// discipline appbuild.resolveDatabaseURL uses.
var Getenv = os.Getenv
