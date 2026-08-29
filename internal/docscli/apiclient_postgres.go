//go:build postgres

package docscli

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// NewAPIClient is unavailable on the postgres build, for exactly the reason
// NewCapturer is: the assertion server stands up a project via
// appbuild.Discover, which on this build binds pgstore to the shared
// RELA_DATABASE_URL — so seeding a "throwaway" fixture would write into the
// real database. api{} fails loud here rather than contaminating live data.
// (Tier-A resolvers, and the store-free shows{}/refuses{}/permits{}
// assertions, still work on the postgres build.)
func NewAPIClient(_ string) (docs.APIClient, error) {
	return nil, errors.New("api{} is not available on the postgres build (it would seed into the live database); run `rela-docs build` from the default build")
}
