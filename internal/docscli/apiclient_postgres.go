//go:build postgres

package docscli

import (
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/docscapture"
)

// NewAPIClient builds the client serving api{} assertions on the postgres
// build. Like NewCapturer it used to refuse, and for the same reason: the
// assertion server stands up a project via appbuild.Discover, which here binds
// pgstore to the shared RELA_DATABASE_URL.
//
// It is now scoped by the same mechanism — a private scratch schema per
// DOCUMENT, dropped CASCADE at teardown (see docscapture.scratchBackend) — so
// an api{} assertion on this build reads and writes only its own fixture. The
// schema is per document rather than per consumer because the client and the
// capturer share one project.
//
// That matters beyond parity with the capturer: api{} is how a manual DRIVES a
// write through the real entitymanager, which is what makes version history
// exist in the first place. A seed verb writes to the raw store and is never
// versioned; an api{} PATCH is a genuine edit and is.
func NewAPIClient(shared *docscapture.SharedProject) (docs.APIClient, error) {
	return docscapture.NewAPIClient(shared), nil
}
