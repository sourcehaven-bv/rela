//go:build windows

package audit

// oNoFollow is zero on Windows: the os package's syscall layer has no
// O_NOFOLLOW open flag, so it cannot be requested at open time. The
// directory-level symlink defense in [ensureDirSafe] still applies on
// every platform; only the per-file open-time symlink refusal is
// unavailable here.
const oNoFollow = 0
