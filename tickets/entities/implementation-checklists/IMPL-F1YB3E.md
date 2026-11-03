---
id: IMPL-F1YB3E
type: implementation-checklist
title: 'Implementation: Translate end-user screens of the data-entry SPA to Dutch'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (the i18n guards cover key parity, unused keys and the bare-string ratchet; existing component tests keep asserting the English text)
- [x] Integration tests written (test full flow, not just units) (N/A: no new flow; the full e2e suite runs against the converted screens)
- [x] Happy path implemented
- [x] Edge cases from planning handled (plural forms for counts, named placeholders, literal key records, locale-aware lowercasing of labels)
- [x] Error handling in place (errors surfaced, not swallowed) (error toasts keep the server message as a placeholder)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no new test data)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no new assertions)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no new assertions)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no new assertions)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no new assertions)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Ran rela-server (e2e build) on a copy of the tickets project with
`app.language: nl` and captured the dashboard, search, entity detail, list,
kanban, calendar, gantt, analysis and history screens with Playwright. The
interface text is Dutch; the remaining English is operator configuration or
server-generated (the default view heading "Properties"), which is out of scope.
- The entity page showed the library placeholder "Empty"; routed it through
`useMessages` (`emptyValue`, "Leeg") and confirmed "Leeg" on screen.
- Unit: `npx vitest run` 339 files / 5039 tests pass; `rela-components`
`npm test` and `npm run check` pass; `npm run typecheck` clean; `npm run lint` 0
errors.
- AC1: bare-string allowlist 111 files / 929 strings -> 60 / 441; the in-scope
entries left hold only symbols (arrows, ×, 🔒, ⋮⋮, `v` version prefix).
- AC2: guards key parity and unused-key checks pass.
- AC4: full e2e suite: 483 passed, 2 failed locally; both pass with TZ=UTC
(`gantt-drag` depends on the local time zone, the relation-picker spec is close
to its 30s timeout), so they are environment effects, not this change.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (`common.*` keys for shared verbs; `versionOpLabel` shared by both history views)
- [x] No security issues introduced (no new `v-html`; the one translated title inserted into HTML is escaped with `escapeAttr`)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
