//go:build (!cgo && !darwin && !windows && !plan9) || (osusergo && !windows && !plan9)

package cli

// osUserFallsBackToEnv is true where os/user has no system lookup and
// answers the process's own uid from $USER (Go's lookup_stubs.go).
const osUserFallsBackToEnv = true
