//go:build !postgres

package backendtest

import "github.com/Sourcehaven-BV/rela/internal/appbuild"

// buildName labels this build in skip and failure messages.
const buildName = "filesystem/memory"

// Options returns no options: the fs and memory recipes open a store from the
// project directory the test already wrote, so there is nothing to supply and
// nothing that can be unavailable. These builds never skip.
func Options(_ TB) []appbuild.Option { return nil }

// Env returns no environment overrides, for the same reason.
func Env(_ TB) map[string]string { return nil }

// DSN returns "": these builds have no database, and Config.DatabaseURL is
// ignored by their recipes.
func DSN(_ TB) string { return "" }
