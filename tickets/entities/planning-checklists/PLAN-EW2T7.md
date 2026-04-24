---
id: PLAN-EW2T7
type: planning-checklist
title: 'Planning: Lua HTTP API support'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Add `http.*` module to the Lua sandbox with request/convenience
methods, JSON helpers, per-request timeouts, and a 10 MiB response body
cap. Out of scope: SSRF filtering, multi-value response headers,
configurable body size, streaming, automatic redirect following.

**Acceptance Criteria:** see TKT-5Z863. Verified by the test cases in
`internal/lua/http_test.go` (happy path for each method, timeout, network
error, redirect non-following, full API flow, error table shape).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:** Used `net/http` from stdlib directly — no third-party
HTTP client needed. Error-handling convention modeled on `internal/ai` +
`internal/lua/ai.go` (the `(nil, err_table)` deviation for network calls).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** One new file `internal/lua/http.go` with parse
helpers, a shared `*http.Client` (connection pooling), classifier that
maps Go errors to stable kinds (`timeout/canceled/network/bad_response`),
and table-returning Lua bindings wired via `registerHTTPModule`. Redirects
disabled via `http.ErrUseLastResponse`. Body capped via `io.LimitReader`.

**Files to modify:** `internal/lua/http.go` (new), `internal/lua/http_test.go`
(new), `internal/lua/runtime.go` (register new module), `docs/lua-scripting.md`
(reference docs), `docs-project/entities/guides/GUIDE-lua-scripting.md` (mirror).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** URL validated (http/https schemes,
non-empty host). Method validated against RFC 7230 token chars. Headers
must be string→string. Timeout must be positive. Body is arbitrary string.

**Security-Sensitive Operations:** External HTTP calls — the new threat
surface is documented in top-of-file comments. No SSRF filter by design
(Lua scripts are treated as trusted). Response body capped at 10 MiB.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Each method (get/post/put/patch/delete) gets a
round-trip test against `httptest.NewServer`. Timeout, network error,
redirect-not-followed, non-success status, empty body all have dedicated
tests. JSON encode/decode have happy-path and error-path tests.

**Edge Cases:** Cycle in encoded table, deeply nested encode, deeply
nested decoded response, invalid HTTP method string, empty URL,
`timeout = 0` on convenience method, non-string header values.

**Negative Tests:** Invalid JSON returns `bad_response`; body over cap
returns `bad_response`; unreachable host returns `network`; programming
errors (missing URL, wrong arg types) raise Lua errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Threat surface addressed via top-of-file security doc and
10 MiB body cap. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] User guide / reference docs (`docs/lua-scripting.md`)
- [x] ~~CLI help text~~ (N/A: no new commands)
- [x] ~~CLAUDE.md~~ (N/A: section trimmed out on develop; detail lives in package docs)
- [x] ~~README.md~~ (N/A: no project-level change)
- [x] ~~API docs~~ (N/A: no API surface outside Lua)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Skipped the formal `/design-review` step; the
`/code-review` on the implemented code surfaced the findings that became
RR-2NRY1, RR-72U6V, RR-93W7S, RR-CAJCU, RR-D0RLL, RR-FUIUH, RR-H4W3L,
RR-HGQDT, RR-NJ8JJ, RR-R1X75, RR-ZXYX. Critical/significant findings
addressed before merge.
