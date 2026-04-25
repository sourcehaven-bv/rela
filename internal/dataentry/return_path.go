package dataentry

import (
	"net/url"
	"strings"
)

// isSafeReturnPath reports whether a return_to value is a safe same-origin
// path to redirect to after a form submit.
//
// A path is safe iff it parses as a URL with no scheme and no host, and the
// path component starts with a single "/". This rejects three classes of
// open-redirect payloads that `strings.HasPrefix(s, "/")` lets through:
//
//   - Protocol-relative URLs  like  //evil.com/pwn
//   - Backslash-prefixed URLs like  /\evil.com (browsers normalise \ to /)
//   - Fully-qualified URLs    like  http://evil.com  or  mailto:…
//
// Returns the normalised path on success (same-origin, path+query only) and
// the empty string on rejection. Callers should treat an empty return value
// as "no redirect target; fall back to default."
func isSafeReturnPath(s string) string {
	if s == "" {
		return ""
	}
	// Reject protocol-relative and backslash-tricks up front. url.Parse
	// happily accepts "//evil.com" and reports an empty scheme but sets
	// Host — but we want to reject before the parser normalises anything.
	// Percent-encoded separators are matched case-insensitively (both
	// /%5C and /%5c; both /%2F and /%2f) so the guard mirrors the
	// browser's normalisation and the client-side isSafeReturnPath.
	if strings.HasPrefix(s, "//") || strings.HasPrefix(s, `/\`) {
		return ""
	}
	if len(s) >= 4 && (strings.EqualFold(s[:4], "/%5C") || strings.EqualFold(s[:4], "/%2F")) {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.Scheme != "" || u.Host != "" {
		return ""
	}
	if !strings.HasPrefix(u.Path, "/") {
		return ""
	}
	// Re-assemble to drop any fragment/scheme the parser reconstructed.
	// We want path + raw query + raw fragment (fragment is fine; client-
	// side scroll anchors are expected).
	out := u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}
