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
func isSafeReturnPath(s string) string {
	if s == "" {
		return ""
	}
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
	out := u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}
