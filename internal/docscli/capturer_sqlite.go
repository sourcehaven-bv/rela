//go:build sqlite

package docscli

import (
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/docscapture"
)

// NewCapturer builds a browser capturer for screenshot{} islands on the sqlite
// build.
//
// This used to REFUSE. The capture server stands up a temp project and seeds
// fixture entities straight into its store, which is only safe when that store
// is genuinely throwaway — and it was not: the server resolved the project
// through project.Discover, which walks UPWARD, so a temp path that happened to
// sit under a real project opened THAT project's database instead. The seed
// would land in live data, and the run would take the user's single-writer
// lock. The refusal was correct while nothing pinned the root.
//
// appbuild.At now does (TKT-SK2QQW): the capture server names the directory it
// just created as the project root rather than discovering one, so the sqlite
// database is `<temp>/.rela/rela.db` and goes away with the temp directory.
// Neither hazard survives, so the refusal has nothing left to protect.
//
// Note this does NOT make the history figures available here — version capture
// is a pgstore-only capability, so a history screenshot on this build can only
// photograph the "not available for this deployment" message. Build the manual
// against postgres for those.
func NewCapturer(shared *docscapture.SharedProject) (docs.Capturer, error) {
	return docscapture.New(shared)
}
