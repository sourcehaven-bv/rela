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
func reconcileDerivedSchemaIfSupported(_ context.Context, _ store.Store, _ *SharedBase) {}
