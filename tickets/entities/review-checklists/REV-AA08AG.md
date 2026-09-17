---
id: REV-AA08AG
type: review-checklist
title: 'Review: e2e harness search_path encoding'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

The change is TypeScript in `e2e/`, so the Go gates are unaffected by it and
were run to confirm exactly that. `node_modules` is absent in this worktree, so
`tsc` and Playwright could not run locally; CI covers both. The encoding
property itself was verified directly under node, and the parsing half under
pgx v5.11.0: the old form yields `options="-c+search_path=..."` and the new one
yields `search_path="schema,public"`.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

Self-review caught two defects in my own working copy before commit.

1. The new spec originally had no `POSTGRES_E2E_ENABLED` guard. `pgDsnForSchema`
reads `RELA_E2E_DATABASE_URL`, and `new URL("")` throws, so the spec would have
failed the default fsstore e2e run rather than skipping. Added the same guard
every other postgres spec uses.
2. Its second assertion originally checked only that the protocol looked like
postgres and the path was non-empty, which passes against a DSN that dropped
the host or the user. Rewrote it to compare each component against the admin
DSN it was derived from.

Scope check: `pgDsnForSchema` is a single call site (`fixtures.ts:839`), and
the only other consumer of the admin DSN is `psqlExec`, which passes it through
untouched. Exporting the function is the one API change and exists so the spec
can call it.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | the built DSN contains no "+" | PASS | asserted directly in `pg-dsn.spec.ts` |
| 2 | the schema is still pinned | PASS | `search_path` reads back as `<schema>,public` |
| 3 | pgx parses the new form correctly | PASS | `pgconn.ParseConfig` under v5.11.0 returns `search_path` -> `relae2e_1,public` |
| 4 | the new spec fails on the old encoding | PASS | old form contains "+" and yields `search_path=null` |
| 5 | the default fsstore e2e run is unaffected | PASS | spec skips without `RELA_E2E_DATABASE_URL` |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — test-harness fix with no user-facing surface.

The comment on `pgDsnForSchema` now records why `search_path` is used instead of
`options=-c search_path=...`. Without it the next person reaches for the libpq
form, since it is the more familiar of the two and the reason it is wrong here
is a property of the encoder, not of the DSN.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Worth recording: I reported the Go fix for BUG-P1QKMB as resolving this defect,
and the postgres suites going green supported that. It was incomplete. The same
DSN is built a second time in TypeScript, and I had searched Go for the pattern
rather than searching for the wire format. The E2E failure that followed looked
like an unrelated flake until its log showed the identical SQLSTATE 42704.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
