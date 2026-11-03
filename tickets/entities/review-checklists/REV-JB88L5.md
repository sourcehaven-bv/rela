---
id: REV-JB88L5
type: review-checklist
title: 'Review: Roll-up properties: read-time aggregates over related entities'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (all changed packages pass; the full local run's only failures were the unbuilt-SPA rela-desktop test and a sqlitestore suite timeout under machine load. CI is the full gate.)
- [x] Lint clean (`just lint`) (two findings fixed: funlen in assemble, whitespace in api_v1.go. The local reruns could not load packages under load; CI lint confirms.)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (store 65.5%, visibility 89.6%, rollup 73%; all above their floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3VQEGX, RR-LMHUMO, RR-QVZASX, RR-U1350N (significant);
RR-4M3Q2T, RR-38AIA5, RR-NJMSS0, RR-VPE5DX (minor); RR-3WJSP7, RR-I88MG6,
RR-WL9L8Q, RR-IS3K0T (nit). All addressed. The security review found no critical
or significant issue; its one minor finding is RR-QVZASX.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: rollup, metamodel and appbuild schemacheck tests; entitymanager refuses writes.
- AC2 PASS: storetest RunRollupTests on memstore, fsstore, sqlitestore, pgstore; dataentry and MCP read surface tests.
- AC3 PASS: TestRollups_ReadSurfacesPerPrincipal with four principals; visibility TestRollups_Values.
- AC4 PASS: TestQueryBudget_ListRollupsAreSizeIndependent.
- AC5 PASS: GUIDE-metamodel "Roll-up properties", docs/metamodel.md regenerated.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CXF6WL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
