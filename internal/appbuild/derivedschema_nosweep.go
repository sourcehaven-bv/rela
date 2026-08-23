//go:build !postgres

package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// reconcileDerivedSchemaIfSupported is a no-op in non-postgres builds: only
// pgstore can synthesize derived-schema objects (partial unique indexes) from
// the metamodel. fsstore/memstore enforce `unique: true` with the
// application-level check-then-write scan, which is correct for their
// single-process nature (TKT-3Q0GP1).
//
// The SQLITE build inherits this deliberately, not by omission (TKT-L1A3PH).
// The scan is only safe because there is exactly one writer, and sqlitestore
// makes that true rather than assuming it: Open takes an exclusive lock on a
// sidecar file and refuses a second process. Were that lock ever removed, this
// no-op would become a correctness hole — two processes would have no
// uniqueness backstop at all, and the violation would be silent.
func reconcileDerivedSchemaIfSupported(_ context.Context, _ store.Store, _ *metamodel.Metamodel) {}
