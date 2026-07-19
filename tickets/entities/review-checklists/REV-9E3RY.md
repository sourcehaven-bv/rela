---
id: REV-9E3RY
type: review-checklist
title: 'Review: Add Tx write-transaction contract to store.Store with fs/mem/pg implementations (DEC-8UIL0 phase 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: default/postgres/memorybackend builds + vet; `go test ./...`; `-race`
on fsstore/memstore/storetest; pgstore suite `-race -tags postgres` against a
real PostgreSQL 15 (all Tx + TxRollback subtests confirmed executed); plimsoll,
arch-lint, golangci-lint, gofmt clean; coverage 76.2%.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** [[RR-XBZX5]] (minor, addressed — escaped-view godoc),
[[RR-Y75YD]] (minor, addressed — probabilistic-test comment), [[RR-QWFGJ]] (nit,
wont-fix with reason). Reviewer verdict: "fundamentally sound", no
critical/significant findings; lock ordering, pg view construction, savepoint
semantics, txPending replay ordering, self-echo filtering, and nested-join
correctness each explicitly verified.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS (three-tag builds + conformance suites); AC2
PASS (WriteVisibleAfterTx); AC3 PASS (SerializedReadModifyWrite, race detector);
AC4 PASS (ReadYourWrites + NestedTxJoins, no deadlock); AC5 PASS
(ErrorPropagates + pg TxRollback suite incl. NoEventsOnRollback). Details in
[[IMPL-KEG5N]].

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** [[DOCS-8I8CB]]

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1150 (stacked on #1149)
