---
id: REV-DUPZ8K
type: review-checklist
title: 'Review: Duplicate an entity from the detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

2943 frontend unit tests (188 files), 309 e2e (10 skipped), Go tests for
`internal/dataentry`, `internal/dataentryconfig` and `internal/apiwire/v1`.
`just lint` 0 issues, `just arch-lint` no warnings, `just plimsoll` clean —
the last one mattered: `App` sits at exactly its 90/23 cap, so all new logic
went to a package function or a component. `just comment-lint` clean across
15216 comments. Coverage 79.9%, both thresholds satisfied. `just docs-check`
green after regenerating twice.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 14 from code review, plus 10 from the earlier design
review. Critical: RR-SLFLP, RR-SYMIN, RR-PICKTY, RR-NOTEST. Significant:
RR-TOUCHW, RR-DIRTY0, RR-FOCRST, RR-CARPTY, RR-PHSTCK, RR-VR2YGE. Minor:
RR-TYPDRP, RR-DIRASM, RR-WRLDRL, RR-TSTQAL. Deferred with reason: RR-INVERR
(pre-existing error-taxonomy gap in `resolveDirection`, out of scope).

A **security review** ran alongside and returned no findings, verifying all
seven design claims — including the one I most wanted checked, that marking
prefilled keys `userTouched` cannot let a value reach the store that the
principal may not write. It confirmed the server denies these with a 403 and
an audit row, and that a value for an unreadable field cannot be constructed
because `stripHiddenProperties` already emptied the source.

Three findings were genuine bugs that all tests passed over, and all three
fail the same way — a successful create with wrong or missing data and no
error:

1. **RR-SLFLP.** A self-loop returns under BOTH keys. Carrying either linked
   the copy to the source; carrying both made them point at each other.
   Verified against a running server: duplicating a self-blocking ticket
   produced `TKT-007 blocks TKT-001` AND `TKT-001 blocks TKT-007`. Now dropped
   and reported, since reproducing it needs an id that does not exist yet.
2. **RR-SYMIN.** A symmetric self-inverse relation maps to itself, so map
   membership cannot decide direction.
3. **RR-PICKTY.** A relation the create form does not render had no registered
   type, which aborts the WHOLE save with advice ("reload the form") that
   cannot help.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS. Evidence in IMPL-DUPZ8K, extended by this round:
AC18 (self-loop) moved from "satisfied by construction" — which was wrong, it
produced a mutual pair — to explicitly dropped and reported. AC2's counts now
agree with what actually carries (RR-TYPDRP). AC20's modal-stack registration
is joined by a focus-restore assertion that fails against the previous
implementation.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DUPZ8K

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One deliberate refactor came out of review rather than being planned: the
card-routing decision moved into a pure `planPrefillRouting`. The first test
for RR-SYMIN passed against the buggy code, because the body key recovers by
luck — only testing the decision rather than its downstream effect catches it.
That is also what the reviewer meant about tests that pin an import rather
than a behaviour.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: it
      post-dates this checklist — `/pr` gates on the ticket already being
      `done`, so checking it now would assert CI passed before CI ran.
      See TKT-UFV01M.)
