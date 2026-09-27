---
id: REV-EEW9QL
type: review-checklist
title: 'Review: sqlitestore: push GraphQuery down into SQL like pgstore'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; plus `-tags sqlite` and `-tags postgres` runs of store, appbuild, cli and dataentry)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`; no new comment-report findings in the diff)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-IRMSDC, RR-GABZJD, RR-1C1KA5 (significant, addressed);
RR-PZXQ9P, RR-9L46EG, RR-WXEW6Y, RR-55RW2R (minor, addressed); RR-VPUYU9,
RR-B76Y5V (minor, deferred to BUG-LCHDSR); RR-ZBR6X5, RR-C0BRME, RR-QURKFT (nit,
addressed); RR-8QILNS, RR-XH8VUJ, RR-94DB8Q (nit, wont-fix). The security review
found no issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 PASS: the storetest conformance suite passes on sqlite through the SQL path.
- AC2 PASS: TestGraphDifferential passes on sqlite and pgstore against graphquerynaive over an independent memstore.
- AC3 PASS: the EXPLAIN QUERY PLAN tests show the derived query and list indexes in use, index-only endpoint matches, and page-driven MatchingIDs.
- AC4 PASS: the list-page and pushed-scope budgets hold at 10 and 50 rows on sqlite.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GS0C5P

## Final Checks

- [x] Commit message explains the why, not just what (commits carry the ticket ID)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after `done`)
