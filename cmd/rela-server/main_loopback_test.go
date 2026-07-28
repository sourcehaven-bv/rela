package main

import "testing"

// TestIsLoopbackHost pins the predicate that decides, for a server with no
// acl.yaml, whether configured commands run ungated (TKT-AQIT9M). It used to be
// a warning trigger and a pprof guard; it is now one of the three inputs to
// SelectCommandAuthorizer, so a "helpful" edit here silently changes who may
// execute shell commands.
//
// The wantLoopback=false cases are the load-bearing half. Every ambiguous or
// malformed spelling must answer false, because false means "network bind",
// which means deny. A case asserted false here is asserting the FAIL-CLOSED
// direction on purpose, not incidentally:
//
//   - "0.0.0.0" / "::" are the wildcard binds — they accept traffic from every
//     interface, so they are the opposite of loopback however local they look.
//   - "" is what an unset --bind would give; it must not read as local.
//   - "[::1]" is the bracketed form, which belongs in a host:port string, not a
//     bare --bind host. Over-denying a real loopback is the safe error.
//   - "localhost.evil.com" and "127.0.0.1.evil.com" are the prefix/suffix
//     confusions a substring check would fall for (internal/jwtauth still uses
//     strings.HasPrefix(host, "127."), which accepts the latter).
//   - "2130706433" and "0x7f000001" are integer encodings of 127.0.0.1 that
//     net.ParseIP rejects; dialers may still resolve them, so deny is correct.
//
// Deliberately NO hostname resolution: a DNS-resolving check would be both a
// TOCTOU and a remote-influence surface on an authorization decision.
func TestIsLoopbackHost(t *testing.T) {
	for _, tc := range []struct {
		name         string
		host         string
		wantLoopback bool
	}{
		{"ipv4 loopback", "127.0.0.1", true},
		{"ipv4 loopback, other octet", "127.0.0.2", true},
		{"ipv6 loopback", "::1", true},
		{"ipv4-mapped ipv6 loopback", "::ffff:127.0.0.1", true},
		{"localhost", "localhost", true},
		{"localhost, mixed case", "LocalHost", true},

		{"ipv4 wildcard is not loopback", "0.0.0.0", false},
		{"ipv6 wildcard is not loopback", "::", false},
		{"empty host is not loopback", "", false},
		{"bracketed ipv6 is not a bare host", "[::1]", false},
		{"private lan address", "192.168.1.5", false},
		{"public address", "8.8.8.8", false},
		{"localhost as a domain prefix", "localhost.evil.com", false},
		{"loopback ip as a domain prefix", "127.0.0.1.evil.com", false},
		{"decimal-encoded loopback", "2130706433", false},
		{"hex-encoded loopback", "0x7f000001", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLoopbackHost(tc.host); got != tc.wantLoopback {
				t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.wantLoopback)
			}
		})
	}
}
