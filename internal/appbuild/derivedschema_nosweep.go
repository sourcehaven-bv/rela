//go:build !postgres && !sqlite

package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// reconcileDerivedSchemaIfSupported is a no-op on fsstore and memstore: they
// run no SQL, so there is no index to derive. They enforce `unique: true` with
// the application-level check-then-write scan, which is correct for their
// single-process nature (TKT-3Q0GP1).
func reconcileDerivedSchemaIfSupported(
	_ context.Context, _ store.Store, _ *SharedBase, _ config.Loader,
) {
}
