---
id: PLAN-VFBBQS
type: planning-checklist
title: 'Planning: i18n infrastructure for the data-entry SPA (vue-i18n, en+nl, language resolution)'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: vue-i18n (composition API) in the SPA; a Dutch message set for the
rela-components library through its existing `provideMessages` seam; `en`/`nl`
message files; language resolution (user > `app.language` > browser > `en`);
`Intl` formatting and `<html lang>` follow the resolved language; API error-code
map; a ratcheting guard against bare template strings; key parity and unused-key
check; sidebar (Sidebar, SidebarFooter, sidebarNav, git status text) and account
menu converted; a language selector on the Settings page; `app.language` in
`data-entry.yaml` (Go config, validation, wire, Configure allowlist); docs.

Out: other screens (TKT-5GX50Y), Configure and Settings internals (TKT-2TUDZH),
strings in TS outside the slice, server-side message translation.

**Acceptance Criteria:**
1. With no preference and no `app.language`, a browser set to `nl-NL` gets the
Dutch sidebar and account menu; `en-US` gets English. Test: `resolveLanguage`
unit table + manual check in the running app.
2. `app.language: nl` makes Dutch the default for a browser in English; a user
choice of English on the Settings page overrides it and survives a reload. Test:
resolution unit table + manual check.
3. `app.language` that is not BCP 47 (`dutch!`) or has no message file (`de`)
fails config load with a named error. Test: Go table test in dataentryconfig.
4. `<html lang>` and `Intl` date/number formatting follow the resolved language
(`nl` gives "15 jan 2024"). Test: unit test on the locale module and formatDate.
5. A known API error code shows a Dutch message under `nl`; an unknown code shows
the server message. Test: unit test on getErrorMessage.
6. A new `.vue` file with a bare string fails `npm run lint`; an allowlisted file
that gains or loses bare strings fails the ratchet test. Test: ratchet test.
7. A key missing from `nl`, or a key no source file uses, fails
`npm run test:run`. Test: parity test.
8. Library strings in the sidebar (collapse, filter, theme toggle) are Dutch
under `nl`. Test: unit test on `localeMessages` + manual screenshot.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approved brief G6 fixes the approach; the few options are listed below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: vue-i18n composition API is the standard setup)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- vue-i18n 11 (composition API, `legacy: false`), as the brief requires.
- rela-components already has `provideMessages`/`useMessages`
(`packages/rela-components/src/composables/useMessages.ts`): typed functions per
string, injected from the host. Reused instead of adding vue-i18n to the
library.
- Ratchet pattern: `internal/archguard/bareref_test.go` (allowlist that may only
shrink).
- Per-browser preferences already live in localStorage (`theme`,
`datetimeTimezone` in `stores/ui.ts`).
- Cross-boundary file comparison: `TestFrozenTypographyContractMatchesSPA`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `frontend/src/i18n/`:
   - `locales/en.json`, `locales/nl.json`, keyed by area (`nav.*`, `footer.*`,
`account.*`, `settings.language.*`, `errors.*`, `language.*`).
   - `language.ts`: pure `resolveLanguage({user, project, browser})` returning
`{language, locale, source}`. A tag counts when its primary subtag has a message
file. `locale` is the winning tag when it has a region, else the first browser
tag with the same language (so `en-GB` keeps 24h time), else the bare language.
   - `locale.ts`: reactive state (user preference from localStorage key
`language`, project default, browser languages), `resolved` computed,
`intlLocale()`, `setUserLanguage`, `setProjectLanguage`. Imports only vue.
   - `index.ts`: the vue-i18n instance; a watcher syncs `global.locale` and
`document.documentElement.lang` to `resolved`.
   - `apiErrors.ts`: code-to-key map. The code is the last segment of the
problem `type` URI.
2. `utils/format.ts`, `utils/dateBuckets.ts`: default `locale` to
`intlLocale()`. Because `intlLocale()` reads a ref, computeds re-run on a
switch. Bare `toLocale*String()` calls in comment, history and calendar views
pass `intlLocale()`.
3. `api/errors.ts`: `ApiError.code`; `getErrorMessage` maps known codes and
network/cancelled kinds when the UI language is not English. Under English the
server message stays, because it is more specific than a generic line.
4. Library: `ChromeTextKey` + `chromeText` in `Messages` for the sidebar and
theme-toggle strings that were hardcoded; `messages.nl.ts` with a full Dutch
`Messages` (the type enforces parity); `localeMessages(getLocale)` delegates per
call so a switch re-renders. The SPA calls
`provideMessages(localeMessages(...))` in App.vue.
5. Guard: ESLint `vue/no-bare-strings-in-template` as error on `src/**/*.vue`
(text plus title/aria-*/placeholder/label/alt attributes), with the files in
`frontend/src/i18n/bare-strings-allowlist.json` ignored. That file records each
file's current count. `src/i18n/guards.test.ts` lints with the rule and fails
when a count differs from the record in either direction, or when a listed file
is gone. The same test checks key parity and that every key appears literally in
`src/`.
6. Slice: Sidebar.vue, SidebarFooter.vue, sidebarNav.ts, AccountMenu.vue and the
git.ts status text use `t()`.
7. Settings: a new `LanguageSetting.vue` card (Automatic / English / Nederlands)
inserted with one line into SettingsView.vue.
8. Go: `AppConfig.Language` (`app.language`), validated with
`golang.org/x/text/language` (strict parse; base must be in
`SupportedUILanguages` = en, nl); `v1.AppConfig.Language`; Configure allowlist
`editable`; a test pins `SupportedUILanguages` to the locale files on disk.
9. Docs: `docs-project` data-entry guide, App section: `language` and the
Settings choice.

**Alternatives rejected:**
- vue-i18n inside rela-components: couples the library to one i18n library and
duplicates the existing typed seam.
- Storing the preference server-side in `_settings`: that file is one value
shared by all principals and needs `settings:edit`, so it is not per-user.
- Mapping error codes under English too: loses the server's specific message.
- Allowlist without counts: lets new strings into allowlisted files.

**Files to modify:** frontend/package.json, package-lock.json, eslint.config.js,
vite.config.ts (feature flags), src/main.ts, src/App.vue, src/test/setup.ts,
src/i18n/*, src/api/errors.ts, src/utils/format.ts, src/utils/dateBuckets.ts,
src/components/common/{Sidebar,SidebarFooter,AccountMenu}.vue, sidebarNav.ts,
src/stores/git.ts, src/views/SettingsView.vue (one line),
src/components/settings/LanguageSetting.vue, comment/history/calendar views
(locale argument only), packages/rela-components useMessages.ts, messages.nl.ts,
RlSidebar.vue, RlThemeToggle.vue, RlSidebarGroup.vue, index.ts;
internal/dataentryconfig/{config.go,validate.go,tests},
internal/apiwire/v1/responses.go, internal/dataentry/api_v1.go,
internal/configedit/allowlist.go, frontend/src/types/config.ts, docs-project
GUIDE-data-entry.md, frontend/CLAUDE.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `app.language` (operator config): strict BCP 47 parse plus an allowlist of
languages with a message file; an invalid value fails the load.
- localStorage `language`: accepted only when it is a supported language;
anything else is ignored.
- `navigator.languages`: only primary subtags in the allowlist are used; the
tag is canonicalised with `Intl.getCanonicalLocales` inside a try block.

**Security-Sensitive Operations:** none. Message text is rendered by
interpolation, never `v-html`. The error map replaces messages and adds no
content. `app.language` is config, not a secret (root CLAUDE.md).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each acceptance criterion names its test. Component tests:
AccountMenu and SidebarFooter render Dutch labels under `nl`. Integration: run
rela-server with the built SPA, switch nl/en, take screenshots.

**Edge Cases:**
- Empty or absent `app.language`; region tags (`nl-BE`); case (`NL`); a browser
list with an unsupported first entry (`de-DE, nl`); empty `navigator.languages`;
a stale localStorage value (`fr`); private mode where localStorage throws.

**Negative Tests:** invalid BCP 47, unsupported language, extra key in `nl`,
missing key, unused key, bare string in a new file, allowlisted count drift.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Concurrent PRs that add bare strings fail CI after this lands. Mitigation:
the failure message explains `t()`; noted in the PR.
- Existing component tests mount without the i18n plugin. Mitigation: install
it globally in `src/test/setup.ts`.
- The browser language can show for one render before `/_config` arrives when
`app.language` differs. Accepted: the user preference is known synchronously.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project GUIDE-data-entry (App section: `language`; Settings choice)
- [x] frontend/CLAUDE.md: i18n rules for new code (use `t()`, the ratchet)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-LC3TBZ, RR-E2SF4K, RR-00YZ8L (significant, folded
into the approach: literal keys only; bound literals converted by hand and
documented; SPA list derived from locale files, Go list pinned to them by a
test), RR-J7URXL, RR-PJPIP0 (minor, addressed).
