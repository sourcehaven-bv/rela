---
id: IMPL-Y7XKCB
type: implementation-checklist
title: 'Implementation: i18n infrastructure for the data-entry SPA (vue-i18n, en+nl, language resolution)'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (language.test.ts: 13-case resolution table; locale.test.ts: state, html lang, Intl formatting, plurals; errors.test.ts: code map under en/nl/unknown; slice.test.ts: sidebar, footer, LanguageSetting, library seam; AccountMenu.test.ts Dutch rows; Go language_test.go)
- [x] Integration tests written (guards.test.ts runs the real ESLint config over the tree and the locale files; slice.test.ts mounts real components through provideMessages)
- [x] Happy path implemented
- [x] Edge cases from planning handled (invalid/unsupported tags at every level, region borrowing, unknown error code, empty project language, library unknown language)
- [x] Error handling in place (errors surfaced, not swallowed): invalid app.language fails config load with a named error; localStorage failures fall back to the in-memory choice

## Test Quality

- [x] Using fixture builders or factories for test data (existing axiosErrorWith/open helpers reused)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions are on translated text, which must be literal to prove the translation)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

rela-server built from the branch, demo project copy with `app.language: nl`,
headless Chromium with locale en-US (playwright script, screenshots in
/tmp/rela-g6):

- `/api/v1/_config` returns `"language":"nl"` under `app`.
- First load: `<html lang="nl">`; sidebar shows Zoeken, Analyse, Stapels,
Instellingen, Over, Sneltoetsen; theme toggle shows Systeem/Licht/Donker
(library text through provideMessages). Operator labels stay as written.
- Settings > Taal offers "Automatisch (Nederlands)", English, Nederlands.
Choosing English switches the whole chrome live, sets `<html lang="en-US">`
(region borrowed from the browser) and stores `language=en`.
- Reload keeps English. Choosing Nederlands again switches back.
- A missing entry on a view shows the server text: its code
`view_execution_failed` is not in the map, which is the designed fallback.
Recorded for TKT-5GX50Y.

Gates: npm run typecheck, lint (0 errors), test:run (all pass), build;
rela-components vue-tsc -b, npm run check, vitest; go test for dataentryconfig,
configedit, apiwire, dataentry; just docs.

## Quality

- [x] Code follows project patterns (check similar code): localStorage like theme/datetimeTimezone; library seam reuses provideMessages; allowlist ratchet mirrors archguard
- [x] Checked for DRY opportunities: one bare-strings.json feeds both ESLint and the ratchet test; SUPPORTED_LANGUAGES derived from the locale files
- [x] No security issues introduced (language tags validated; messages contain no HTML; no v-html)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
