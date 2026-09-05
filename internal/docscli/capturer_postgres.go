//go:build postgres

package docscli

import (
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/docscapture"
)

// NewCapturer builds a browser capturer for screenshot{} islands on the
// postgres build.
//
// This used to REFUSE, because the capture server stands up a project via
// appbuild.Discover, which on this build binds pgstore to the shared
// RELA_DATABASE_URL — so a "throwaway" seed would have written fixture
// entities into the operator's real database. The refusal was correct while
// nothing scoped those writes.
//
// docscapture.scratchBackend now scopes them: the temp project is pinned to a
// private, randomly-named PostgreSQL schema created for the build and dropped
// CASCADE at teardown. Nothing outside that schema is read or written, so the
// contamination the refusal existed to prevent cannot happen.
//
// Building the manual on THIS build is the only way to capture a screenshot of
// a screen backed by a postgres-only capability — version history being the
// one that motivated it (HistoryReader is implemented only by pgstore, so on
// the default build the history page can only ever photograph its own
// "not available for this deployment" message).
func NewCapturer(shared *docscapture.SharedProject) (docs.Capturer, error) {
	return docscapture.New(shared)
}
