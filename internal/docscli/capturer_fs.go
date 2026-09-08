//go:build !postgres && !sqlite

package docscli

import (
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/docscapture"
)

// NewCapturer builds a browser capturer for screenshot{} islands. On the
// default (fsstore) build the capture server stands up a throwaway temp project
// via appbuild.Discover — an ephemeral fsstore that never touches real data.
//
// The temp project is not created here: it is the DOCUMENT's, supplied by the
// caller and shared with the api{} client, so a figure can photograph what an
// earlier api{} island wrote.
//
// This is the sole edge that pulls internal/docscapture (and thus chromedp)
// into a binary; it is reachable only from cmd/rela-docs, keeping chromedp out
// of rela / rela-server.
func NewCapturer(shared *docscapture.SharedProject) (docs.Capturer, error) {
	return docscapture.New(shared)
}
