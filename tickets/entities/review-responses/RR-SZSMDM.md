---
id: RR-SZSMDM
type: review-response
title: isLoopbackHost became security-load-bearing with no test
finding: |-
    [security] isLoopbackHost (cmd/rela-server/main.go:808) was previously only a warning trigger and a pprof guard. This ticket promotes it to one of the three inputs deciding whether unauthenticated shell execution is permitted, and it had no test anywhere — grepped cmd/rela-server, internal/dataentryconfig and internal/jwtauth.

    TestSelectCommandAuthorizer passes `loopback` as a bool, so it covers the decision matrix but NOT the predicate that produces the bool. The one function converting an operator-supplied string into a security boolean was the uncovered one. Someone 'simplifying' it to strings.HasPrefix(host, "127.") — as internal/jwtauth/verifier.go:447 already does — would make 127.0.0.1.evil.com loopback and break nothing in CI.
severity: significant
resolution: |-
    Added TestIsLoopbackHost (cmd/rela-server/main_loopback_test.go), a 16-case table. The wantLoopback=false cases are the load-bearing half and the godoc says so: they assert the FAIL-CLOSED direction on purpose rather than incidentally — 0.0.0.0 and :: (wildcard binds accept every interface), "" (unset --bind), [::1] (bracketed form, over-denying a real loopback is the safe error), localhost.evil.com and 127.0.0.1.evil.com (the prefix/suffix confusions a substring check falls for), and 2130706433 / 0x7f000001 (integer encodings net.ParseIP rejects but dialers may resolve).

    Also verified and documented that the absence of hostname resolution is deliberate: a DNS-resolving check would be both a TOCTOU and a remote-influence surface on an authorization decision. Confirmed independently that passing the bare f.bind unsplit is correct — --bind takes a host, --port is separate and joined via net.JoinHostPort.
status: addressed
---
