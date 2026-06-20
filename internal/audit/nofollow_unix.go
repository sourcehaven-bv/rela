//go:build !windows

package audit

import "syscall"

// oNoFollow refuses to open the audit file through a symlink, so an
// attacker who can plant a symlink at the audit path cannot redirect
// appended audit records elsewhere. Pairs with the symlink check in
// [ensureDirSafe]. Available on every unix target; see the windows
// variant for the platform that lacks this open flag.
const oNoFollow = syscall.O_NOFOLLOW
