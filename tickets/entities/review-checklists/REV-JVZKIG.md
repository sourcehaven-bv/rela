---
id: REV-JVZKIG
type: review-checklist
title: 'Review: Phase 1: declarative calendar/feed export — internal/feed serializer, feeds: config (multi-source), ICS+JSON HTTP endpoint'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green.
- [x] Lint clean (`just lint`) — golangci-lint 0 issues.
- [x] Coverage maintained (`just coverage-check`) — `internal/calfeed` ~92%; floors hold. `just ci` and `just check` both exit 0 locally.

## Code Review

- [x] Run `/code-review` command (invoked cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-2V0019 (critical, addressed), RR-NA8DML / RR-RF88EH /
RR-279F73 (significant, addressed), RR-5880C0 (minor, addressed), RR-7AQX4E
(minor, wont-fix with justification). Plus the design-review findings from the
planning phase (RR-4C2AI4 / RR-4AWSTN / RR-FZRAC8 / RR-0E20T7 addressed;
RR-78OHN5 / RR-7C151B superseded). No open critical/significant responses.

Findings fixed:
- **CRLF injection (critical)** — `writeLine` strips raw CR/LF (structural
invariant); resolved RRULE also validated via rrule-go and dropped if invalid.
- **Ambiguous UID separator (sig)** — changed `<type>-<id>` → `<type>--<id>`
(entity ids reject `--`), fixing hyphenated types (`test-case`,
`review-response`).
- **Get type-check (sig)** — reject when a by-id lookup returns a different type.
- **Malformed property RRULE (sig)** — validated at render, dropped on failure.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-RBO4NA)

**Acceptance Status:** All acceptance criteria PASS. Serializer (AC1–7), Lua/
config surface (AC8–9 as the declarative config), HTTP endpoint + ACL + CSRF
(AC10–11), config validation (AC12/13 as validateFeeds + fail-fast), backend
isolation (AC13), lint/plimsoll/coverage (AC14) — all covered by automated tests
and verified manually end-to-end against the real pim.rela project (see
IMPL-RBO4NA verification evidence).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-35CLAY)
- [x] User-facing documentation updated (GUIDE-data-entry.md "Calendar feeds"
section → docs/data-entry.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-35CLAY

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — the code matrix (Lint, Test, E2E, Architecture,
God-object, all cross-compiles, Postgres, CodeQL, Vulnerability, Docs) is green;
the only remaining gate is this ticket reaching `done`.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1071
