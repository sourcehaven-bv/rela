---
id: REV-AJYIJN
type: review-checklist
title: 'Review: Form relation direction: infer from schema, require it when self-referencing (drop the implicit outgoing default)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...` — full suite green)
- [x] Lint clean (`golangci-lint run ./...` — 0 issues; `just arch-lint` — OK)
- [x] Coverage maintained (`just coverage-check` — package floor 50% PASS,
total floor 65% PASS, total 78.0%)

Frontend unit tests were NOT run — `vitest` is not installed in this checkout.
Mitigating: no frontend file was changed (`git status -- frontend/` is empty).
The SPA's `direction === 'incoming'` check is unchanged by design; the server
now guarantees a non-empty direction reaches it.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BL8OW8 (critical, addressed), RR-GJLLB6 (critical,
addressed), RR-8KKIC8 (significant, addressed), RR-O22R9L (significant,
addressed), RR-5NENZ3 (significant, addressed), RR-NRYAFC (significant, deferred
→ TKT-E4I6NX).

The review found a genuine hole in the headline feature: `direction: ""`
unmarshalled to `"outgoing"` and walked straight past the ambiguity check. I
reproduced it independently before accepting the finding. Fixed at the parser so
an empty value stays empty and the single inference rule owns the decision.

One finding was deferred with justification: the mutable-singleton migration
registry (RR-NRYAFC) is a pre-existing pattern affecting every `MetamodelAware`
migration, so it is filed as TKT-E4I6NX rather than widened into this ticket.

Reviewer leverage suggestions taken: migrate idempotence assertion added to the
round-trip test (the original bug's worst property was that re-running reported
success while leaving damage). Not taken: fuzzing the round-trip invariant —
worthwhile, but a separate piece of work.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Result | Evidence |
| --- | --- | --- |
| 1. from-side infers outgoing | PASS | `TestResolveFormRelations_Direction/from-side...` |
| 2. to-side infers incoming | PASS | `TestValidateConfig_FormRelationToSide_InfersIncoming` + wire-level `TestResolveFormRelations_Direction/to-side...` |
| 3. self-referencing errors | PASS | `TestValidateConfig_FormRelationSelfReferencing_RequiresDirection`; also holds for `direction: ""` (`..._EmptyDirectionStringIsNotOutgoing`) |
| 4. explicit preserved | PASS | `..._ExplicitDirectionOK`, `TestFormRelationDirectionMigration/explicit_direction_is_preserved` |
| 5. migrate fills unambiguous, skips ambiguous | PASS | `TestFormRelationDirectionMigration` (7 subtests incl. wizard steps) |
| 6. migrate never invalidates | PASS | `TestMigrateThenValidate_RoundTrip` (+ idempotence) |
| 7. cleanup no longer strips direction | PASS | `TestDataEntryCleanupMigration_PreservesIncomingDirection` |

Real-config verification: `tickets/` and `prototypes/data-entry/project`
validate clean. `prototypes/data-entry/catalog` fails on an invalid widget
`"search"` — confirmed pre-existing via `git stash`, unrelated to this change.

E2E fixture caught during review: `e2e/tests/fixtures.ts` had an ambiguous
`tagged` (feature→feature) form binding that would have failed server startup
and broken the whole e2e suite. Fixed to `direction: outgoing`, matching the
seeded FEAT-001→FEAT-002 edge; the extracted fixture now validates clean.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-B5KK6J

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A yet: nothing has
been committed — the working tree holds the change and the user has not asked
for a commit or PR)
- [x] ~~All CI checks pass~~ (N/A yet: no PR; local equivalents all pass — see
Automated Checks above)
- [x] ~~PR URL documented below~~ (N/A yet: no PR)

**PR:** not created. This checklist stays `in-progress` and the ticket stays in
`review` until the change is committed and a PR is opened — the workflow's
"cannot be merged" gate is doing its job here, since there is genuinely nothing
merged yet.
