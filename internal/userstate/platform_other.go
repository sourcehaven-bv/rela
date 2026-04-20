//go:build !darwin && !windows

package userstate

// tagNotIndexed is a no-op on platforms without a default-on
// content indexer. Linux's tracker-miner is opt-in and respects
// the XDG_CONFIG_HOME convention of "don't index my config."
func tagNotIndexed(string) error { return nil }
