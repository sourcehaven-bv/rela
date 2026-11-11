//go:build headless

package main

// rela-docs serves the embedded web UI, which the headless build tag leaves out.
// This undefined name stops the build with a readable error instead of
// producing a binary with no UI.
var _ = relaDocsCannotBeBuiltHeadless
