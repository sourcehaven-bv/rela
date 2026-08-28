---
id: REV-XBQ5DR
type: review-checklist
title: 'Review: Ticket gate rejects code-only and non-bug-entity PRs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — run by the pre-commit hook on 822fceb
- [x] Lint clean (`just lint`) — 0 issues, pre-commit hook
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code
      changed; the diff is a workflow YAML file plus ticket entities)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: no application code — the change
      is a CI workflow step, reviewed directly against the GitHub Actions
      script-injection guidance; see the trailer-parsing evidence below)
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes — the branch carries only
      `.github/workflows/ci.yml` plus this ticket's entities. Pre-existing
      unrelated edits in the working tree (`e2e/tests/fixtures.ts`,
      `scripts/reachability.sh`, `reachability*.out`) were left unstaged.

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented below

**Acceptance Status:**

- **Widened allowlist accepts ticket-hygiene PRs** — PASS. Replayed the new
  glob set against `origin/tickets/rr-ugkoi5-addressed` (PR #1209) from its
  real merge-base: matches `review-responses/RR-UGKOI5.md` and
  `doc-tasks/DOC-WWNE41.md`. Previously matched nothing.
- **Code-only PR can pass via companion trailer** — PASS. Replayed against
  `origin/fix/views-acl-field-redaction` (PR #1212): still matches no
  work-item entity, so it takes the `Tickets-PR:` branch.
- **Existing ticket PRs still match** — PASS. `origin/tickets/acl-shielding-bugs`
  (PR #1214) matches 7 entities, a superset of what the old three globs found.
- **Trailer parsing is correct and injection-safe** — PASS. Table-tested
  plain / no-hash / lowercase / mid-prose / absent / empty, plus
  `'...; rm -rf /'`, `'$(whoami)'` and backtick payloads. The body is bound
  via `env:` and never interpolated into the script; only `[0-9]+` is ever
  extracted, so command substitutions yield empty.
- **Gate does not become a merge loophole** — PASS. The
  `Ticket done-before-merge check` step is unchanged and still globs only
  `bugs/` + `tickets/`, so terminal-status enforcement is intact.
- **Workflow still parses** — PASS. `yaml.safe_load` on `ci.yml` succeeds.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: internal CI policy with no
      user-facing surface; the contributor-facing contract is the error
      message the step prints, updated to document the `Tickets-PR:` trailer)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-B76AT8

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1216
