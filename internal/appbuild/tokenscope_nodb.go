//go:build !postgres

package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// backendTokenScope has no backend-derived scope outside postgres; the
// caller mints one into the state store.
func backendTokenScope(context.Context, store.Store) (scope string, ok bool, err error) {
	return "", false, nil
}
