---
id: REV-4Z4TWV
type: review-checklist
title: 'Review: ACL read-side: /_search visibility — VisibleSearcher seam, generic + pgstore-native impls, conformance suite'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`) — sole failure is `e2e/node_modules/flatted/...`, a gitignored local node_modules artifact absent from CI checkouts; all real packages above floors, total 75.9%

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Code review (cranky-code-reviewer, full diff vs
origin/feat/acl-listside-tkt-vmd8): 0 critical, 2 significant — RR-A3V2FK
(untested pgstore Filters path → FiltersThenLimit conformance case +
deterministic SQL-shape unit tests), RR-L0ZGRI (filter-validation asymmetry →
ValidateFilters hoisted into generic impl + InvalidFilterRejectedIdentically
case) — both addressed; 3 minor — RR-PEZTM0 (SortIsIgnored case), RR-QGZG72
(no-DB SQL builder tests), RR-2ZS7ZT (GUIDE load note) — all addressed; 1 nit —
RR-EKUNMX (CTE prefix derivation) — wont-fix with justification (positional
uniqueness structurally guaranteed + now test-pinned). Reviewer explicitly
verified and cleared: SQL injection safety, filter-in-place aliasing,
MatchingIDs fail-closed semantics, wiring nil-paths, handleV1Search leak
surfaces, short-circuit condition, _position deny behavior. Design-review round
(13 RRs, RR-1LFQA5…RR-599CLE) closed in planning.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 denied principal empty | PASS | TestACLSearch_TypeLevelGrant + live (mallory → [] total 0) |
| 2 role-relation inheritance | PASS | TestACLSearch_RoleRelationInheritance + live (alice q=Hidden → 0, q=Visible → 5/5) |
| 3 no-leak on wire | PASS | raw-body assertions in AC1/AC2 tests (IDs, titles, property values) |
| 3b related-hidden no-leak | PASS | TestACLSearch_VisibleHitRelatedToHidden (includeRelations=false pinned) |
| 4 short-circuit + positive | PASS | TestACLSearch_DenyAllShortCircuit (0 calls denied / exactly 1 granted) |
| 5 conformance ×3 impls | PASS | 15 cases green on 4 combos: generic+memstore/linear, generic+fsstore/linear, generic+memstore/bleve, pgstore-native (real DB) |
| 6 cap starvation fixed | PASS | LimitPostVisibility + FiltersThenLimit cases; SQL LIMIT placement unit-pinned |
| 7 error semantics | PASS | TestACLSearch_ScopeErrorMapping (constant detail, no echo) + BackendErrorMapping |
| 7b sentinel wrapping + cancel | PASS | errors.Is asserts + TestACLSearch_CanceledScopeStaysSilent (empty body) |
| 8 resolveScope parity | PASS | TestACLPosition_SearchScopeGated green unchanged after readableSubset retirement |
| 9 NopACL regression | PASS | TestACLSearchRegression_NopACL incl. off-metamodel ghost type visible |
| 10 docs | PASS | GUIDE section + regenerated docs/acl-security.md + CLAUDE.md storetest line |

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-C09N2R

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending: created stacked on feat/acl-listside-tkt-vmd8 after ticket
reaches done -->
