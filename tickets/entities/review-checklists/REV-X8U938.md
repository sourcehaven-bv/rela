---
id: REV-X8U938
type: review-checklist
title: 'Review: Sync 2/5: pgstore deletion tombstones + seq indexes + manifest query'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -race -tags postgres ./internal/store/pgstore/` → ok (live Postgres); all three build tags compile
- [x] Lint clean — my files 0 new issues; `just arch-lint` clean
- [x] Coverage maintained — DB-gated tests cover delete/rename tombstones, manifest, catch-up recovery, seq indexes

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — found 2 critical + 3 significant
- [x] All critical addressed — RR-EOXQIB, RR-ACJ0ZY (rename left no tombstones → ghost entities/edges; fixed in-tx)
- [x] All significant addressed — RR-AE54G9 (growth/retention documented), RR-QWC4OX (index lock documented), RR-947I7E (overlap comment corrected)
- [x] Self-reviewed the diff — only pgstore touched; default build / dep gates verified

**Review Responses:** RR-EOXQIB, RR-ACJ0ZY (critical, addressed); RR-AE54G9,
RR-QWC4OX, RR-947I7E (significant, addressed). The criticals were a class the
reviewer caught that I'd missed — rename is a disguised delete and wasn't
tombstoned. Fixed + regression-tested (TestRenameTombstonesOldIdentities).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence in implementation checklist

**Acceptance Status:** all PASS — delete/relation/cascade/rename tombstones,
manifest since-cursor, seq indexes exist, and the headline missed-NOTIFY delete
recovery (TestCatchUpRecoversMissedDelete). pgstore conformance still passes;
default build no pgx, postgres build no bleve.

## Documentation (enhancements only)

- [x] ~~Docs-checklist / user-facing docs~~ (N/A: internal store backend; no user-visible surface. The migration + manifest godoc document the retention/lock caveats for operators/maintainers.)

**Docs Checklist:** N/A — internal.

## Final Checks

- [x] Commit message explains the why
- [x] No TODOs/FIXMEs unaddressed (retention/pagination are tracked follow-ups, not TODOs)
- [x] Ready for another developer — ManifestSince is the API the sync server (TKT-PV0R3V) consumes

## Pull Request

- [ ] Run `/pr` — create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending push -->
