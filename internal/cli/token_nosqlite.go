//go:build !sqlite

package cli

// tokenLockHint returns err unchanged: only the sqlite backend refuses a
// second process.
func tokenLockHint(_ string, err error) error { return err }
