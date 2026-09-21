---
id: REV-7FDVAW
type: review-checklist
title: 'Review: Commands and actions are unreachable on entity types that declare faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./...` clean (only pre-existing macOS linker
warnings); `npm run test:run` 185 files / 2944 tests pass, up from 2930.
- [x] Lint clean — `golangci-lint` 0 issues on the touched packages;
`npm run lint` 0 errors, 130 warnings all pre-existing and none in the changed
files; `just arch-lint` OK, no warnings.
- [x] Comment lint gate clean — `just comment-lint` clean across 15324
comments. `just comment-report` shows one advisory `duplication` finding at
`runtime.go:1749`, which is **pre-existing** (luaCreateEntity / luaUpdateEntity
prose) and untouched by this diff.
- [x] Coverage maintained — `just coverage-check` package floor and total both
PASS, 79.7%.

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer, full diff.
- [x] All critical review-responses addressed — RR-114UN5.
- [x] All significant review-responses addressed — RR-J04T1K (four defects).
- [x] Self-reviewed the diff for unrelated changes — the diff is the three
planned changes, the security gate found in review, their tests, and the two doc
corrections. Nothing unrelated.

**Review Responses:** RR-114UN5 (critical, addressed), RR-J04T1K (significant,
addressed), RR-8H271T (minor, addressed).

The critical finding was a genuine miss on my part, not a false positive:
deleting the SPA suppression made a pre-existing ungated, unredacted store read
reachable. My justification for the deletion checked address *resolution* in
both command contexts and never checked *authorization*, where the two branches
differ. Fixed in this diff rather than deferred.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| Criterion | Status | Evidence |
|---|---|---|
| A `context: entity` command renders on a faced type | PASS | `renders operator COMMANDS at a NON-bare face, addressed BY that face (S1)` — the fixture declares `faces:`, which is where the old gate was total |
| The command receives the served address | PASS | same test asserts `CommandModal.props('entityId') === 'POL-1@published'` |
| A faceless type is unchanged | PASS | `addresses a command by the BARE id when no face is served`; `TestBuildCommandEnvFace/faceless_type_is_unchanged` |
| A script can address the face it was invoked on | PASS | `TestBuildCommandEnvFace` incl. a `ParseStateRef(FormatStateRef(…))` round-trip, both entity-carrying contexts |
| A Lua script can tell which face triggered it | PASS | `TestEntityToTable_CarriesFace` |
| An `action:` offer renders and runs | PASS | 11 cases in `NextActionOffers.test.ts > action` |
| No regression in command authorization | PASS | `TestCommandExec_RowGatesTheEntity`, `TestCommandExec_RedactsHiddenPropertyValue` |

**Every new test was mutation-verified.** Each fix was reverted in turn and the
corresponding test observed to fail:

- Lua `face` removed → both `TestEntityToTable_CarriesFace` subtests fail.
- Env block removed → both `TestBuildCommandEnvFace` subtests fail.
- Confirm latch moved back after the `await` → the double-click test fails.
- Security gate removed → the denied principal gets **200 with the entity**,
and the hidden property value **reaches the payload**. That is the
vulnerability, demonstrated and then closed.

This matters more than usual here: the bug's own root cause is a test that
asserted a suppression and so passed whether or not the affordance was
reachable. A new test that cannot fail would repeat it.

## Documentation (enhancements only)

Bug, so the docs-checklist section is skipped — but two corrections shipped in
the diff because they document behaviour this change creates or clarifies:

- `docs/data-entry.md` — `RELA_ENTITY_FACE` / `RELA_ENTITY_REF` in the command
env table, a faced example, and a note that an entity-context payload is scoped
to the invoking principal.
- `docs/content-states.md` — splits create from update. Update via a fused
address already worked; the old wording implied otherwise.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `set:` on a next-action offer is the
one thing still unrendered, and deliberately so: it has no server endpoint.
Stated in the code comment, the bug body and RR-J04T1K rather than left to be
rediscovered.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: post-dates this
  checklist — `/pr` gates on the ticket already being `done`, so this item can
  only be satisfied by a PR that does not exist yet. See TKT-UFV01M.)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M.
-->
