//go:build sqlite || postgres

package appbuild_test

// The database builds commit a promote atomically.
func init() { wantAtomicPromote = true }
