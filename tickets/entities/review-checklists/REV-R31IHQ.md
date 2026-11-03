---
id: REV-R31IHQ
type: review-checklist
title: 'Review: Split metamodel.Metamodel part 2: relation and property-type queries'
started: "2026-10-11"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test ./...`: all pass except TestChromeStyle_TargetsShippedClasses, which needs a built SPA and fails the same way on develop)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: removed tests cover only deleted getters, and new tests cover the adapter and widget function; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CM4TUC (significant, addressed), RR-LDFLUN RR-Y99Y3N
RR-BA63VE RR-ZDDQV9 (minor, addressed), RR-DIO1EA RR-6AIK53 (nit, addressed),
RR-GHGMXA (nit, wont-fix). Security review: no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. No behaviour change except the typed-nil fix; full suite passes; TestDetectMigrationsWithFS_UnloadableSchemaUsesNoMetamodelPath panics without the guard: PASS.
2. `rela schema` JSON byte-identical before and after (overview, lists, entity and relation details): PASS.
3. Both plimsoll directives removed; `just plimsoll` passes: PASS.
4. arch-lint OK; lint clean on all tag sets; tests compile under default, memorybackend, postgres, sqlite: PASS.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor)
- [x] ~~User-facing documentation updated~~ (N/A: refactor; code comments updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
