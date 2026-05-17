package audit

import (
	"os"
	"strings"
)

// SystemUser returns the OS user running this process — $USER trimmed,
// or "unknown" if $USER is unset or whitespace-only. Used by entry-
// point wiring to populate [Principal.User].
//
// The original plan had a four-tier fallback chain ($RELA_ACTOR →
// $USER → git config user.email → "system") — dropped during round-2
// review as over-engineered for what is one bit of operator identity.
// Operators who want a different identity set $USER; data-entry will
// override per-request via HTTP middleware in a follow-up.
func SystemUser() string {
	u := strings.TrimSpace(os.Getenv("USER"))
	if u == "" {
		return "unknown"
	}
	return u
}
