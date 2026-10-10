---
id: PLAN-5B2BK0
type: planning-checklist
title: 'Planning: Access log records the route shape, not ids or file names'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the path field of the `request` record (access log and its Debug
copy). Out: other log lines that carry paths (reviewed separately), the query
string (already excluded).

**Acceptance Criteria:**
1. An attachment URL logs as `/api/v1/tickets/*/_attachments/file/*` (TestRouteShape, TestRequestStats_AccessLogMasksIdentity).
2. Ids, file names and config names never appear (same tests).
3. Schema names come from the current schema (router test through NewRouter).
4. Truncation still caps the record (TestRequestStats_AccessLogTruncatesLongPath).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: route-pattern logging is the common practice the issue names)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** net/http `r.Pattern` would give the pattern, but the API
dispatches below a catch-all `/api/v1/` handler, so it carries no ids. An
allowlist of segments needs no router change.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `routeShape(path, words)` keeps fixed route words and
schema names, replaces other segments with `*`. `App.schemaRouteWord` reads the
current schema per call. Rejected: masking only the attachment file segment (a
denylist, leaves ids); a positional template parser (drifts with every route).

**Files to modify:** internal/dataentry/requeststats.go, router.go,
requeststats_test.go; GUIDE-server-security.md and generated
docs/server-security.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Request path (client-controlled): allowlist per
segment; anything unknown becomes `*`.

**Security-Sensitive Operations:** Logging to journald/syslog: now carries no
resource identity.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** Root path, trailing slash, unknown plurals, `ID@face`, very long
paths (truncation), schema reload (read per call).

**Negative Tests:** An id or file name must never be logged, even when unknown
to the allowlist.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Less detail for timing (view names masked): acceptable, the route
still identifies the endpoint. Per-request schema scan cost only when logging is
on.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-server-security (access log section).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change; code and security review cover it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
