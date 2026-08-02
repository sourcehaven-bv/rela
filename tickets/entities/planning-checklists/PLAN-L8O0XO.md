---
id: PLAN-L8O0XO
type: planning-checklist
title: 'Planning: gate the analyze reader (arc step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: make `_analyze` (data-entry) read through the requester's visible
reader so validation cannot read hidden data; remove the `visibleAnalysisIssues`
output filter and the provenance-era title/message patches. OUT: the `roles:`
rule annotation (arc step 2 / TKT-270KRY); sync ACL; the sentinel test; the
appbuild (non-data-entry) validator wiring beyond what's needed to keep it
building.

**Acceptance Criteria:**
1. A `visible:`-hidden property value never appears in any `_analyze` response.
2. A hidden ENTITY yields no analyze issues for a principal who can't read it.
3. Full-visibility (and nop-ACL) principals see identical analyze output to today.

## Research

- [x] ~~/research~~ (N/A: design settled in the design discussion that produced this arc).
- [x] Checked codebase for reusable patterns — the ctx-gating reader seam already exists.
- [x] Reviewed prior art — `visibility.PolicyReader` + `ctxRowGate` (the view fix, DEC-ZBI39P); the deliberate `Unrestricted` decision at app.go:560.
- [x] ~~Reference implementations~~ (N/A: internal).
- [x] Reviewed rela concepts — authorization, DEC-ZBI39P.

**Research Doc:** N/A (small, well-scoped refactor).

**Existing Solutions:**
- `visibility.PolicyReader` over `ctxRowGate{}` resolves the read gate from ctx
per request — exactly what the validator's reader needs. Used already by the
view pipeline and export handler.
- The current `visibility.Unrestricted(st)` at internal/dataentry/app.go:~565 is
the seam to replace; its comment documents the false-positive tradeoff we now
accept + guard with roles (step 2).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (ctxRowGate/PolicyReader)
- [x] Alternatives considered (provenance — rejected; see TKT-270KRY)
- [x] Dependencies identified

**Technical Approach:** Swap the data-entry validator's `ReadDeps.VisibleReader`
to a ctx-gating reader so Lua rules' `rela.get_entity`/`list`/`search` read
gated. Route analyze.go's ~6 raw `svc.store` reads through a ctx-gated reader so
the non-Lua checks (Orphans/Duplicates/ID-Gaps/Cardinality/Properties) see only
the requester's slice. Delete `visibleAnalysisIssues` + the title/message
patches in api_v1.go. Validator stays built-once; only its reader becomes
ctx-aware (ctx already flows via `runAnalysis`/`CheckRuleFull`).

**Files to modify:**
- internal/dataentry/app.go (validator VisibleReader wiring)
- internal/dataentry/analyze.go (route store reads through a gated reader)
- internal/dataentry/api_v1.go (remove visibleAnalysisIssues + title/message patches)
- tests: acl_analyze_test.go (assertions shift from "output filtered" to "read gated")

## Security Considerations

- [x] Input sources identified — the requester's principal (from ctx) drives the gate.
- [x] Input validation approach — reuse the existing gate; no new parsing.
- [x] Security-sensitive operations identified — this IS the security operation (read authorization on analyze).
- [x] Error handling doesn't leak — a gate error must fail closed (drop, not reveal); reuse Filter/PolicyReader's fail-closed semantics.

**Input Sources & Validation:** Principal/gate resolved from request ctx via
`readGateFromContext` / `ctxRowGate`, the same path every other gated read uses.
No new input surface.

**Security-Sensitive Operations:** The whole change. Reads route through
`visibility.PolicyReader` (fail-closed on gate error). Removing the output
filter is safe ONLY because the reads are now gated — order matters: gate the
reads in the same PR that drops the filter.

## Test Plan

- [x] Test scenarios documented per acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined (drive handleV1Analyze through the router with a policy)

**Test Scenarios:**
- AC1: hidden property (sentinel value) → assert absent from `_analyze` body (drive the real handler under a Declarative `visible:` policy).
- AC2: hidden entity → assert no issue references it (existing test, keep).
- AC3: full-visibility principal → analyze output identical to nop-ACL run.

**Edge Cases:**
- Gate error mid-analyze → fail closed (issue dropped, nothing revealed).
- nop-ACL deployment → gating is a no-op (everyone visible); output unchanged.
- A rule that reads another entity via get_entity → that read is now gated too.

**Negative Tests:**
- The neutralize-the-fix check: without gating, the sentinel value leaks (proves the test bites).

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated (m)

**Risks:**
- False positives on whole-graph built-in checks for partial-visibility
principals → ACCEPTED, documented; guarded by roles annotation in step 2.
- appbuild (non-data-entry) validator wiring uses a different reader; ensure it
still builds and its behavior is unchanged (out of scope to gate it here).
- Removing the output filter before reads are gated would open the leak wider —
mitigation: both land in ONE PR, reads-gated first.

## Documentation Planning

- [x] ~~User-facing docs~~ (N/A: internal refactor; behavior change to `_analyze` semantics noted in the parent ticket).
- [x] Docs-checklist created when entering implementation → N/A (internal; no doc surface).

**Documentation Impact:**
- [x] N/A - Internal change. (The `_analyze` semantic redefinition is captured in TKT-270KRY; a CLAUDE.md note may be warranted if the gated-analyze pattern becomes a convention — defer to review.)
