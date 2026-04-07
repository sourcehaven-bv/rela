---
id: REV-TX18
type: review-checklist
title: 'Review: Harden rela-server against browser-based local attacks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

The cranky-code-reviewer pass produced 17 findings. All critical and significant
findings are addressed; minor/nit findings are addressed or explicitly deferred
with reason.

Critical (4):
- RR-RR8M — C1: Vue SPA still issues GET to /api/command. **Addressed**: CommandModal.vue, internal/dataentry/static/app.js, executeCommand dead helper removed, Vite bundle rebuilt.
- RR-X05X — C2/C3: RelationFilePath panic on `..` substring. **Addressed**: defensive panic removed; the upstream metamodel.ValidateRelation gate is the actual choke point.
- RR-2HDB — C4: handleV1Documents accepted unsanitised entityID. **Addressed**: isSafePathSegment validation at handler entry; cache write failures now logged via log.Printf.

Significant (4):
- RR-850M — S1: --bind 0.0.0.0 was silently broken. **Addressed**: isUnspecified helper, allowedHosts nil bypasses Host check on unspecified bind; test added.
- RR-51KK — S4: missing SSE no-CORS-headers assertion. **Addressed**: regression assertion added to TestHandleSSEHeaders.
- RR-91AX — S6: false slow-write claim in main.go comment. **Addressed**: comment rewritten honestly; per-handler deadlines tracked as future work.
- RR-8CNS — S7: Host case-insensitive comparison. **Addressed**: lowercased on both sides of the lookup; mixed-case test added.

Minor (6):
- RR-2BAD — S5: containedProjectPath conflated not-found with traversal. **Addressed**: errPathNotFound vs errPathOutsideProject; handler returns 404 vs 403.
- RR-UQ2Q — S9: log injection via raw header values. **Addressed**: strconv.Quote on all logged header values.
- RR-RHVA — N5: validateCacheFilename missed control bytes and Windows drive letters. **Addressed**: full control-byte sweep + drive-letter check.
- RR-U82Z — curl/non-browser blocked by default. **Addressed**: docs/security.md curl section.
- RR-BA6N — Same-origin SSE positive test missing. **Addressed**: TestSecuredRouter_AllowsSameOriginSSE added.
- RR-EXOG — S3: Referer fallback semantics undocumented. **Addressed**: docs/security.md troubleshooting section.

Nit (3):
- RR-G0JM — N2: truncate UTF-8 mid-rune. **Addressed**: rune-aware truncate.
- RR-SW4Y — S2: sensitive-path matcher opt-in not opt-out. **Deferred**: documented footgun in code; not a security blocker because every new top-level route is reviewed.
- RR-SDCY — L4: nil security config tolerated. **Deferred**: production entry point always sets it; deferring touches dozens of test fixtures.
- RR-DL7R — L1: per-instance session token defence in depth. **Deferred**: already in original ticket out-of-scope as follow-up.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 13 ACs from PLAN-S593 verified PASS. See IMPL-M7JN verification table for
the AC → test mapping. Post-review additions:

- AC1 (loopback bind): now also covered by TestNewSecurity_UnspecifiedBindAcceptsAnyHost which exercises 0.0.0.0/::/empty-host bind cases, plus TestRequireLocalHost_AllowsCaseInsensitiveHost.
- AC5 (SSE no CORS): now also asserted by regression checks in TestHandleSSEHeaders, plus positive path test TestSecuredRouter_AllowsSameOriginSSE.
- AC7 (path containment): now distinguishes 404 vs 403 (S5 fix), tests in path_security_test.go updated.
- AC10 (WriteCacheFile): now rejects control bytes (other than NUL) and Windows drive letters too.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs are part of this ticket scope, no separate checklist needed)
- [x] User-facing documentation updated — `docs/security.md` written, then expanded with curl/non-browser section, troubleshooting, and the corrected `--bind 0.0.0.0` instructions
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A — docs in scope of this ticket

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/318
