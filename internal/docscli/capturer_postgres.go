//go:build postgres

package docscli

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// NewCapturer is unavailable on the postgres build: the capture server stands
// up a project via appbuild.Discover, which on this build binds pgstore to the
// shared RELA_DATABASE_URL — so a "throwaway" seed would write fixture entities
// into the real database. Rather than contaminate live data, screenshot{} is
// not supported here; it fails loud with this reason. (Tier-A resolvers —
// tables, diagrams, matrices — still work on the postgres build.)
func NewCapturer() (docs.Capturer, error) {
	return nil, errors.New("screenshot{} is not available on the postgres build (it would seed into the live database); run `rela-docs build` from the default build")
}
