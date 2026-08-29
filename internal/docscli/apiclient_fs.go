//go:build !postgres

package docscli

import (
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/docscapture"
)

// NewAPIClient builds the client serving api{} assertions. On the default
// (fsstore) build it stands up a throwaway temp project via appbuild.Discover —
// an ephemeral fsstore that never touches real data.
//
// Unlike NewCapturer this pulls in no browser: the client only issues HTTP
// requests against the data-entry router, so api{} assertions run wherever the
// Go tests do, with no built frontend and no Chrome.
func NewAPIClient(projectDir string) (docs.APIClient, error) {
	return docscapture.NewAPIClient(projectDir), nil
}
