---
id: BUGA-P2XQVT
type: bug-analysis-checklist
title: 'Analysis: API error messages discarded at 22 call sites (interceptor rejects plain objects)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (any 4xx with a ProblemDetail body — e.g. a policy-denied PATCH — surfaces the generic catch-site fallback instead of the server's `detail`; verified by reading the interceptor contract at `client.ts:31-46` against the 22 `instanceof Error` consumer sites)
- [x] Minimal reproduction steps documented (trigger a 403/422 from any view; toast shows "Failed to …" generic text while the network tab shows a ProblemDetail with `detail`)
- [x] Environment/conditions noted (all environments; affects every structured API error, every consumer using the `instanceof Error` idiom)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (single `ApiError extends Error` normalized in the interceptor; `getErrorMessage()` helper; guards delegate; four parsers deleted — see bug content)
- [x] Regression test planned (pure `normalizeApiError()` table-tested against all four failure shapes + `getErrorMessage` test; pins the boundary contract that why4 found untested)
- [x] Related areas checked for similar issues (the four compensating parsers are the known siblings and are removed by this fix; A7 — surfacing autosave validation warnings — builds on the typed `validationErrors` and is tracked separately)
