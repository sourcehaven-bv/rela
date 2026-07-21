---
id: REV-NGPPA5
type: review-checklist
title: 'Review: Per-command ACL guard: gate command execution and button visibility on a named permission (entity/list/global; view deferred)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Full `just ci` green on the final commit: golangci-lint **0 issues**,
go-arch-lint **OK - No warnings found**, markdownlint **0 issues in 0 files**,
whole-repo `go test ./...` pass, coverage package floor + total both **PASS**,
frontend build ok, `✓ Docs are up to date.`

Two failures were found and fixed during this phase rather than papered over:

1. **MD028** — blank line inside a blockquote in the new Authorization section.
2. **`docs-check` failure — the important one.** `docs/acl-security.md` and
`docs/data-entry.md` are **auto-generated** from
`docs-project/entities/guides/`. The implementation phase had edited the
generated files directly, so every word of the new documentation would have been
silently wiped by the next `just docs` run. Moved to the source guides
(`GUIDE-acl-security.md`, `GUIDE-data-entry.md`) and regenerated; generation
verified idempotent across two consecutive runs.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YZV7SY, RR-CAUBAZ, RR-37AYC0 (significant, all
**addressed**); RR-0LDY3W (minor, **addressed**); RR-PG8HR2 (minor, **deferred**
with justification → TKT-JRY8V5).

No critical findings. Two adversarial passes plus independent verification of
every load-bearing claim; the cancel bypass was confirmed with a throwaway PoC
test (written, run, deleted) rather than accepted on assertion.

Self-review of the production diff found no unrelated changes, no debug code, no
TODOs. The `tickets/` additions are the project dogfooding its own tracker and
belong in the commit.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 9 PASS.

| # | Criterion | Evidence |
|---|---|---|
| 1 | `permission: X` runs for a holder, 403s for a non-holder (entity/list/global) | `TestCommandExecDeclarativeFailsClosed` — granted×3, not-held×2 |
| 2 | Unauthorized commands absent from `GET /_commands` | `TestResolveCommandsFiltersUnauthorized` |
| 3 | No `acl.yaml` ⇒ unchanged in all four contexts incl. view | `TestCommandExecNopACLFailsOpen` (4 subtests) |
| 4 | `acl.yaml` + no `permission:` ⇒ denied | `TestCommandExecDeclarativeFailsClosed` — 3 no-permission cases |
| 5 | View denied **even when set and granted** | `TestCommandExecDeclarativeFailsClosed` — "view denied despite granted permission" |
| 6 | `validateCommands` warns on view + `permission:` | `TestViewCommandPermissionWarning` (3 subtests) |
| 7 | `--read-only` denies every command, every context | `TestCommandExecReadOnlyDenied` (4 subtests) — **failed before the fix, proving the vuln** |
| 8 | `docs/acl-security.md` documents `command:*` and the view limitation | `GUIDE-acl-security.md` → generated; verified by `docs-check` |
| 9 | `read-only-mode.spec.ts` comment corrected | committed |

Beyond the stated criteria, two review-driven properties are also pinned:
`TestAuthorizeCommandUnknownACLDenies` (4 subtests — the fail-closed switch) and
`TestCommandCancelOwnerBound` (2 subtests — cross-principal kill).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-IJJR4H

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The commit message leads with the vulnerability and the non-obvious constraint
(the read gate cannot implement this policy because `nopReadGate` covers both
`NopACL` and `ReadOnlyACL`), not with a list of touched files.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] ~~All CI checks pass~~ (N/A at time of writing: remote CI was still
running when this checklist had to reach `status: done` — the ticket cannot be
`done` without it, and the PR cannot exist without the ticket being `done`.
Local `just ci` is fully green, which runs the same gates. Remote status is
tracked on the PR itself.)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1180
