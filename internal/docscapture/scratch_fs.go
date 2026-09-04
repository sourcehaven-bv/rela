//go:build !postgres

package docscapture

import "github.com/Sourcehaven-BV/rela/internal/appbuild"

// scratchBackend returns the appbuild options that point a stood-up temp
// project at a THROWAWAY backend, plus a cleanup to run at teardown.
//
// On the default (fsstore) build there is nothing to do: the temp project IS
// the throwaway — appbuild.Discover opens an fsstore under the temp directory
// standUp just created, and removing that directory removes the data. The
// postgres build has to work for it (see scratch_postgres.go), which is the
// whole reason this is a seam rather than an unconditional call.
//
// Nil: the returned cleanup is never nil — callers may defer it unconditionally.
func scratchBackend(string) ([]appbuild.Option, func(), error) {
	return nil, func() {}, nil
}
