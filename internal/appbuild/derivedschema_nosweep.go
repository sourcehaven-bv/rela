//go:build !postgres

package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// reconcileDerivedSchemaIfSupported is a no-op in non-postgres builds: only
// pgstore can synthesize derived-schema objects (partial unique indexes) from
// the metamodel. fsstore/memstore enforce `unique: true` with the
// application-level check-then-write scan, which is correct for their
// single-process nature (TKT-3Q0GP1).
//
// The SQLITE build inherits this deliberately, not by omission (TKT-L1A3PH).
// The scan is only safe because there is exactly one writer PER PROJECT
// DATABASE, and sqlitestore makes that true rather than assuming it: Open takes
// an exclusive lock on a sidecar file beside the database and refuses a second
// opener. (Two rela-sqlite processes on DIFFERENT projects are fine — the
// invariant is one writer per database, not one process per machine.)
//
// Were that lock ever removed, or were the database placed somewhere the lock
// cannot be trusted, this no-op would become a correctness hole: two writers
// would have no uniqueness backstop at all and the violation would be silent.
// The second case is why Open also verifies WAL actually engaged and refuses
// otherwise — the filesystems where flock is unreliable are the same ones where
// WAL is unavailable.
func reconcileDerivedSchemaIfSupported(_ context.Context, _ store.Store, _ *SharedBase) {}
