package appbuild

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/filecomments"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// commentsDirName is the comment store's home inside the project's `.rela/`
// directory.
//
// Under `.rela/` rather than beside `entities/` because comments are not graph
// content: they should not appear as a sibling of the operator's data, and a
// project that never enables commenting should see nothing at all.
const commentsDirName = "comments"

// buildComments constructs the commentary service, or nil when the metamodel
// declares no enabled `comments:` block.
//
// Returning a genuinely nil *comments.Service (not a service over a no-op
// store) is load-bearing in the same way versionServiceFor's nil is: the
// data-entry app nil-checks it to decide whether to serve the comment routes
// at all, so a disabled feature costs an operator no routes, no directory and
// no storage — which is what AC1 asks for.
//
// Backend choice is deliberately NOT build-tagged today: the file backend is
// correct for every current build (fs, memory, sqlite are all single-process,
// and the postgres build's multi-process concern arrives with pgcomments in the
// follow-up ticket). Making it a per-recipe choice before there is a second
// backend would add four call sites that all pass the same thing.
func buildComments(fs storage.FS, paths *project.Context, meta *metamodel.Metamodel) (*comments.Service, error) {
	if !metamodel.NewCommentPolicy(meta).Enabled() {
		return nil, nil //nolint:nilnil // a nil service IS the "feature disabled" signal; see the doc above.
	}
	if fs == nil || paths == nil || paths.CacheDir == "" {
		// Commenting is configured but there is nowhere to put the data. This
		// is a wiring failure, not a degradation: silently disabling a feature
		// the operator switched on would surface later as comments vanishing.
		return nil, errors.New("comments: enabled in the metamodel but no project cache directory is available")
	}

	store, err := filecomments.New(fs, filepath.Join(paths.CacheDir, commentsDirName))
	if err != nil {
		return nil, fmt.Errorf("comments: %w", err)
	}
	svc, err := comments.NewService(store, nil)
	if err != nil {
		return nil, fmt.Errorf("comments: %w", err)
	}
	return svc, nil
}
