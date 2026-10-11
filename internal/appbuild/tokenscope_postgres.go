//go:build postgres

package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// backendTokenScope is the store's schema on postgres: every process serving
// the schema derives the same scope, so they agree on lock keys and on the
// additional data sealed into each token.
func backendTokenScope(ctx context.Context, st store.Store) (scope string, ok bool, err error) {
	schema, ok, err := pgstore.SchemaOf(ctx, st)
	if err != nil || !ok {
		return "", ok, err
	}
	return "pg:" + schema, true, nil
}
