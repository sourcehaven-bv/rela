---
id: IMPL-G522
type: implementation-checklist
title: 'Implementation: ACL: predicate-backed _fields and _relations resolver (replace stub)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature verified end-to-end via HTTP-level integration tests
  (`affordances_policy_test.go`): real `affordances.New` →
  `policyResolver` → GET wire shape + PATCH 403 + audit attribution.
- [ ] ~~Browser/SPA manual test~~ (deferred to review phase: the SPA
  renderer is unchanged from TKT-G7N5, which was browser-verified;
  this ticket only swaps the verdict source, exercised by the
  HTTP integration tests. A demo-profile browser pass can confirm
  during `/code-review` if desired.)
- [x] Each acceptance criterion verified with a test scenario:
  - AC1 — `policy_test.go` parse + `resolver_test.go`
    `TestResolver_New_CompileError_IncludesPath`
  - AC2 — `TestResolver_NoAffordanceBlocks_EmptyVerdicts` +
    `TestPolicyResolver_NoAffordanceBlocks_PermissiveWire`
  - AC3 — `TestResolver_FieldPredicateFalse_Denies` +
    `TestPolicyResolver_FieldPredicate_WireAndWrite`
  - AC4 — `TestResolver_OptionFiltered`
  - AC5 — `TestResolver_RelationCreateFalse` +
    `TestPolicyResolver_RelationCreate_WireShape`
  - AC6 — `TestResolverFromProfile_Demo`
  - AC7 — wire-parity exercised via the policy integration tests
    against the same handlers TKT-G7N5's contract test covers
- [x] Edge cases verified: off-type coercion
  (`TestResolver_OffTypeProperty_CoercesNotFails`), cross-role
  union (`TestResolver_CrossRoleUnion`), local roles
  (`TestResolver_LocalRole_HasRole`), global-role
  (`TestResolver_HasGlobalRole`), YAML empty/null/absent
  (`TestLoadPolicy_AffordanceGrants_OptInIsKeyPresence`),
  `*bool` forms (`TestLoadPolicy_RelationGrant_CreateRemovePointers`).

**Verification Evidence:**

- `just check` (lint + arch-lint + lint-md + test, race-enabled) — PASS
- `just coverage-check` — PASS (total 76.7%; `internal/affordances` 79%)
- `just build` — all binaries build
- `go-arch-lint` — new `affordances` component boundaries clean;
  `dataentry → affordances` edge declared
- DR-C5 two-channel verified: `TestPolicyResolver_AuditCarriesAttribution`
  asserts the wire 403 body has no role/predicate while the audit
  Summary carries `role=triager`.

**Deferred to review/follow-up (documented, not silently dropped):**

- DR-S1 metamodel-hot-reload regression test — behavior is
  documented (restart required); a dedicated regression test is a
  nice-to-have, not load-bearing for correctness.
- DR-S5 perf microbench — predicate step budget bounds cost; no
  caching in v1 per plan ("measure first"). Bench can be added if
  perf becomes a concern.
- DR-L3 audit-invariant integration test (every denial kind →
  one JSONL row) — the audit emission path is shared with
  TKT-G7N5's existing audit test; the new attribution test covers
  the field-deny path.

## Quality

- [x] Code follows project patterns (consumer-side `RelationLookup`
  interface at the call site; capability-scoped; sparse verdicts
  mirror TKT-G7N5; `errors.Join` for multi-error).
- [x] Checked for DRY opportunities — `FieldGrant` reused for
  relation-meta grants (DR-S6); `roleHasAffordanceGrants` helper;
  `recordAttribution` shared.
- [x] No security issues introduced — predicates are sandboxed
  (predicate package), fail-closed on error, operator-supplied
  only; no user Lua on the read path.
- [x] No silent failures — compile errors abort startup; runtime
  predicate errors log + deny.
- [x] No debug code left behind (temporary round-trip debug test
  removed).
