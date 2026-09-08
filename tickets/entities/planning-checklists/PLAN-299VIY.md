---
id: PLAN-299VIY
type: planning-checklist
title: 'Planning: Runtime under the load line (elevation/output/schema-sort)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `internal/lua` — elevation (−2), output/filesystem (−3),
schema/sort (−3) clusters off Runtime; directive 45 → 37. Stacked on TKT-DOPCTI.
OUT: reads/writes/lifecycle/registration clusters (deliberately kept on Runtime
— they are its reason for existing), any behavior or ACL-semantics change.

**Acceptance Criteria:**
1. `just plimsoll` with the directive at the real count and Runtime **under
the 40-method load line**.
2. Full suite + `-race ./internal/lua/` green; the bypass_acl/elevated tests
pass unchanged (they are the ACL contract).
3. arch-lint/comment-lint/lint clean.

## Research

- [x] ~~Run `/research`~~ (N/A: third repetition of an established in-package pattern)
- [x] ~~Existing libraries~~ (N/A)
- [x] Checked codebase for similar patterns
- [x] Looked for reference implementations
- [x] Reviewed prior art

**Research Doc:** N/A — the Runtime survey behind TKT-4WBLG6 covered all 105
methods, per-cluster field usage included; this ticket implements its remaining
low-risk candidates.

**Existing Solutions:** `urlHelpers` (urls.go), `mdHelpers`/`mdASTConverter`/
`mdEntityRefs` (TKT-4WBLG6), `httpBindings`/`cacheBindings`/`aiBindings`
(TKT-DOPCTI). `registerElevatedWrites`/`registerElevatedReads` were already free
functions taking a ctx closure — the elevation move follows a seam that already
existed.

## Approach

- [x] Technical approach chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** `elevationBindings` over the three elevated deps plus a
`ctxFn func() context.Context`; `newElevatedHandle` becomes a free function (it
already received callerCtx as a value). `outputBindings` over
stdout/outputDir/projectRoot/isAction/isDocument — captured by value only after
verifying none is mutated post-construction (contrast: cacheBindings needed a
closure because SetScriptPath mutates scriptPath). `schemaBindings` over
deps.Meta; `luaSortEntities` becomes a free function since it touches no runtime
state. Alternative rejected: also moving reads/writes/lifecycle — those
genuinely use deps + callerCtx and are the runtime's purpose.

**Files to modify:** internal/lua/{runtime.go, elevation.go (new), output.go
(new), schemasort.go (new)}

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unchanged — same binding code, moved.

**Security-Sensitive Operations:** **This is the ACL-elevation surface.**
`rela.bypass_acl` is an object capability: the `admin` handle must carry ONLY
the capabilities actually wired (TKT-Y3JVFK), and `registerBindings` must keep
building the bindings only when an elevated handle exists so an unwired
capability is STRUCTURALLY ABSENT rather than present-and-erroring. The
ElevationRecorder post-closure audit must fire identically. The ctx must stay a
closure re-read at call time — a snapshot would strip the caller's Principal
from elevated writes.

## Test Plan

- [x] Test scenarios documented
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] ~~Integration approach~~ (N/A: existing Lua-level suites are the integration tests)

**Test Scenarios:** The 24 elevation tests (AbsentWithoutAnyHandle,
ReaderOnlyHandle, WriteElevationStillWorksWithoutReadElevation,
AuditsEvenWhenClosureRaises, DeniedReadIsNotAudited, …) are the contract and
must pass unmodified.

**Edge Cases:** Value-capture safety for the output mode flags — must be
verified, not assumed, against the sibling PR's scriptPath precedent.

**Negative Tests:** Unwired-capability absence and denied-read-not-audited both
covered by existing tests.

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated

**Risks:** The elevation cluster is the only security-sensitive piece; it is
receiver plumbing over a seam that already existed, and the contract is densely
tested. Effort: s.

## Documentation Planning

- [x] ~~User-facing docs~~ (N/A: internal refactor)
- [x] ~~Docs-checklist~~ (N/A: refactor kind)

**Documentation Impact:** N/A.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: third repetition of a reviewed design; the security-sensitive cluster's invariants were named up front and verified in code review)
- [x] ~~Findings addressed~~ (N/A)

**Design Review Findings:** N/A
