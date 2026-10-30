---
id: BUGA-X2DUOCV
type: bug-analysis-checklist
title: 'Analysis: Data API request without identity gets 500 instead of 401'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`TestACLMiddleware_AnonymousIsUnauthenticated` fails on develop with 500 `acl_unstamped_principal`)
- [x] Minimal reproduction steps documented (bug body: ACL configured, header identity, request without the header)
- [x] Environment/conditions noted (Declarative ACL, header or env identity mode, no JWT gate)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (in `attachACLRequest`, answer 401 `unauthenticated` when the stamper ran and named no user; keep 500 for an unstamped ctx or a named user with an unknown tool)
- [x] Regression test planned (middleware tests for both sides, golden corpus anonymous recording becomes 401)
- [x] Related areas checked for similar issues (below the edge `writeNoIdentity` already answers 401, BUG-O51NS5; JWT mode already answers 401; MCP HTTP and realtime outputs handle `ErrUnstampedPrincipal` on their own paths and are out of scope)
