//go:build postgres || sqlite

package docscli

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// NewCapturer is unavailable on the database builds. The capture server stands
// up a project via appbuild.Discover and then seeds fixture entities straight
// into the store, which is only safe when that store is genuinely throwaway.
//
//   - postgres: Discover binds pgstore to the shared RELA_DATABASE_URL, so the
//     seed lands in the real database.
//   - sqlite: Discover resolves through project.Discover, which walks UPWARD
//     from the given directory. The capture server passes a temp dir, but if
//     that temp path happens to sit under a real project the seed writes into
//     that project's database instead. It also takes the single-writer lock,
//     so a capture run would fight the user's own open project.
//
// Rather than contaminate live data, screenshot{} is not supported on either;
// it fails loud with this reason. (Tier-A resolvers — tables, diagrams,
// matrices — still work on both builds.)
func NewCapturer() (docs.Capturer, error) {
	return nil, errors.New(
		"screenshot{} is not available on the postgres or sqlite builds " +
			"(it would seed fixture entities into a real database); " +
			"run `rela-docs build` from the default build")
}
