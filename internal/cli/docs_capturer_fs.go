//go:build !postgres

package cli

import (
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/docscapture"
)

// newDocsCapturer builds a browser capturer for screenshot{} islands. On the
// default (fsstore) build the capture server stands up a throwaway temp project
// via appbuild.Discover — an ephemeral fsstore that never touches real data.
func newDocsCapturer() (docs.Capturer, error) {
	return docscapture.New()
}
