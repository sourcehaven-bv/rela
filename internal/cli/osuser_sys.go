//go:build !((!cgo && !darwin && !windows && !plan9) || (osusergo && !windows && !plan9))

package cli

// osUserFallsBackToEnv is false where os/user asks the system for the
// process's own account.
const osUserFallsBackToEnv = false
