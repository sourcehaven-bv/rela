---
id: REV-SBYH5U
type: review-checklist
title: 'Review: Computed properties in schema.yaml: derived, non-editable, stored and indexed, with chained derivation and cycle detection'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`, race-enabled)
- [x] Lint clean (`just lint`, zero issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`: 77.8% total; 65% floor)
- [x] Architecture, markdown, frontend build/typecheck, binary builds, and
generated-doc checks pass (`just ci` exit 0)

## Code Review

- [x] Completed the `/code-review` security/correctness checklist manually
(the workflow's named review subagent was not available under this run's
delegation constraints)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none. One verification gap was found during review:
search-index propagation had only architectural evidence. It was fixed by adding
`TestComputed_MaterializedValueReachesSearchIndex`; focused tests and lint
stayed green.

Review covered expression sandboxing and static typing, dependency extraction,
cycle ordering, integer overflow/domain errors, SQL-portability classification,
write chokepoints, automation/cascade behavior, trusted sync recomputation,
read-only affordances, schema drift, ACL disclosure, and generated docs.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference PLAN-1ZF3O1)
- [x] Test evidence documented in IMPL-ZEIBM2

**Acceptance Status:**

- Typed entity-local `computed:` expressions: PASS (compiler/metamodel tests)
- Read-only via data-entry/CLI/MCP/Lua mutation chokepoints: PASS (unit tests and live CLI set/unset rejection)
- Stored, filterable, and indexed: PASS (live CLI create/filter plus observer-backed search-index integration test)
- Chained dependency order: PASS (`internal/computed` chain test)
- Cycles fail validation: PASS (compiler and `projectsetup.ValidateWithFS` tests)
- Expression changes report shape drift: PASS (metamodel shape tests)

## Documentation

- [x] Docs-checklist created and linked via `has-docs` (DOCS-E70GOV)
- [x] User-facing metamodel and data-entry API documentation updated
- [x] Docs-checklist marked done

## Final Checks

- [x] Commit messages explain the typed-IR/SQL-readiness rationale
- [x] No new TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run the repository PR workflow immediately after the ticket reaches done

Remote CI/PR URL remain authoritative on GitHub and are not duplicated as a
precondition for closing this review checklist.
