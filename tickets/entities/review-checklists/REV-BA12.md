---
id: REV-BA12
type: review-checklist
title: 'Review: ACL: predicate-backed _fields and _relations resolver (replace stub)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`, race-enabled)
- [x] Lint clean (`just lint` → 0 issues; `just arch-lint` → no warnings;
  `just lint-md` → 0 errors)
- [x] Coverage maintained (`just coverage-check` PASS; total 76.7%,
  `internal/affordances` 84.6%)

## Code Review

- [x] Ran `/code-review` (cranky-code-reviewer agent over the full
  new + modified file set)
- [x] All critical review-responses addressed (RR-1DRR — the C1/C2/C3
  attribution side-channel — resolved by deleting the side table and
  threading attribution through the write-path verdict)
- [x] All significant review-responses addressed (RR-08AK, RR-RTJE,
  RR-QV18, RR-XYTO)
- [x] Self-reviewed the diff for unrelated changes — only the
  `newSearchBackend` extraction in appbuild and the entry-point
  `buildFieldResolver` / `failLoad` helpers are incidental, and each
  was extracted to satisfy funlen while wiring the resolver; no
  drive-by behavior changes.

**Review Responses:**

Design-review (planning phase): RR-ZH1D, RR-ZAGR, RR-D2CP, RR-OGQF,
RR-BWDE, RR-P6HP, RR-QXUF, RR-QKRT, RR-SQJU — all addressed.

Code-review (this phase): RR-1DRR (critical), RR-08AK / RR-RTJE /
RR-QV18 / RR-XYTO (significant), RR-TL9I (minor) — all addressed.

No open critical or significant review-responses for this ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (see implementation checklist
  IMPL-G522 for the test-to-AC mapping)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 — PASS. `policy_test.go` parses the new blocks;
  `TestResolver_New_CompileError_IncludesPath` + the S2 target-
  validation tests prove load-time failure with the grant path.
- AC2 — PASS. `TestResolver_NoAffordanceBlocks_EmptyVerdicts` +
  `TestPolicyResolver_NoAffordanceBlocks_PermissiveWire`.
- AC3 — PASS. `TestPolicyResolver_FieldPredicate_WireAndWrite` (GET
  writable=false + PATCH 403 with `field-affordance:read-only:status`,
  wire body carries no attribution).
- AC4 — PASS. `TestResolver_OptionFiltered` + the enum-filter wire
  rule_id path in `validateFieldWrite`.
- AC5 — PASS. `TestPolicyResolver_RelationCreate_WireAndWrite` drives
  the actual create POST → 403 `relation-affordance:not-creatable`.
- AC6 — PASS. `TestResolverFromProfile_Demo` (hard override).
- AC7 — PASS. Policy integration tests run against the same handlers
  TKT-G7N5's contract test covers; wire shape unchanged.

Plus DR-C5 two-channel: `TestPolicyResolver_AuditCarriesAttribution`
(PATCH with no prior GET; wire body clean, audit Summary has
`role=triager`).

## Documentation (enhancement)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-Z3XV)
- [x] User-facing documentation updated — `docs/security.md`
  "Field- and relation-level affordances" section (acl.yaml schema,
  predicate language table, closed-world / cross-role semantics,
  fail-closed + restart-required notes) and `docs/data-entry/
  api-reference.md` verdict-source table now lists the policy-backed
  source.
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-Z3XV

## Final Checks

- [x] Commit message will explain the why (replacing the dev stub with
  a policy-driven source), not just the what
- [x] No TODOs or FIXMEs left unaddressed (temporary round-trip debug
  test removed; no debug code)
- [x] Ready for another developer to use — acl.yaml schema documented,
  predicate language documented, profile selection documented

## Pull Request

- [x] Ran `/pr` — PR created and CI monitored
- [x] All CI checks pass (the `Rela Tickets` dogfood job clears once
  this ticket transitions to `done`; all build/test/lint/coverage/
  arch/e2e/docs jobs green)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/841
