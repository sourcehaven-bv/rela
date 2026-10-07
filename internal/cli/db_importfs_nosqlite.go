//go:build !sqlite

package cli

import (
	"context"
	"errors"
)

// errImportFSNeedsSQLite answers `rela db import-fs` in every build but the
// SQLite one. The target is a SQLite project, so only that build can write
// it, and only that build can open it afterwards.
var errImportFSNeedsSQLite = errors.New(
	"'db import-fs' creates a SQLite project; run it with the SQLite build (rela-sqlite)")

func runDBImportFS(context.Context, string, string) error { return errImportFSNeedsSQLite }
