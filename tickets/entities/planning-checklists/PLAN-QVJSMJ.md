---
id: PLAN-QVJSMJ
type: planning-checklist
title: 'Planning: restish discovers rela''s spec via Link service-desc; CI e2e with real restish'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: an RFC 8631 `Link: rel="service-desc"` header on `GET /`; a Go
test that drives a real restish against the router; a CI step that installs a
pinned, checksum-verified restish and makes the test required; the guide drops
`spec_files`. Out: serving the spec at `/openapi.json` (a second route outside
the /api/ gate); OAuth through a proxy in CI (needs an identity provider).

**Acceptance Criteria:**
1. `GET /` carries `Link: </api/v1/_openapi.json>; rel="service-desc"`; other SPA routes do not. Test: `TestOpenAPI_RootLinksToSpec`.
2. restish given only the origin connects, uploads with `put-ticket-attachment` and downloads the same bytes. Test: `TestRestish_DiscoversSpecAndRoundTripsAttachment`; it fails in CI when restish is missing (`RELA_TEST_RESTISH=1`).
3. The guide's setup has no `spec_files` step.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small follow-up; discovery order read from restish source)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- restish v2.3.0 `internal/spec/discover.go`: discovery order is explicit spec URL, `Link` rel service-desc/service-doc/describedby on GET of the base URL, `/openapi.json`, then the base body. Discovery uses the profile's auth transport, so it works behind a proxy.
- CI pattern for an optional external binary: the ClamAV job's `RELA_TEST_CLAMD=1` gate.
- CalDAV's root handler (`caldav_handler.go` registerWellKnown) wraps the SPA handler; wrapping `spa` before it covers both mounts.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `withSpecLink` wraps the SPA handler in `NewRouter` and
adds the header when the path is exactly `/`. `openAPISpecPath` const shared
with the route registration. The test uses
`httptest.NewServer(app.NewRouter())`, isolates restish's config and cache in a
temp HOME, and runs `api connect`, `put-ticket-attachment`,
`get-ticket-attachment`. The CI Test job installs restish 2.3.0 linux-amd64 with
a SHA-256 check and sets `RELA_TEST_RESTISH=1`.

Alternative rejected: spec at `/openapi.json`. It adds a public-surface route
outside /api/; the Link header reaches the same result with no new route.

**Files to modify:** internal/dataentry/router.go, api_v1.go,
openapi_routes_test.go, restish_e2e_test.go (new), .github/workflows/ci.yml,
docs-project/entities/guides/GUIDE-restish.md, docs/restish.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input. The header is a constant.

**Security-Sensitive Operations:** The header names a config path, which is not
secret; the spec stays behind the JWT gate. CI downloads a third-party binary:
pinned version and SHA-256, no untrusted workflow inputs interpolated.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: unit test on `/` and `/tickets`. AC2: restish e2e test.
AC3: doc review.

**Edge Cases:** Root with CalDAV enabled (same wrapped handler); HEAD / (header
set before the handler runs).

**Negative Tests:** Removing the wrapper makes the restish test fail (mutation
check, done locally: `put-ticket-attachment` is an unknown command).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A restish release changes command naming or discovery: version pinned; upgrades are deliberate.
- Download from GitHub releases flakes: the job fails loudly rather than skipping.
Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/restish.md (via GUIDE-restish)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: approach chosen with the user in TKT-3DLP0K's follow-up question; change is a constant header plus a test)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run, see above)

**Design Review Findings:** N/A
