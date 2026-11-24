---
id: IMPL-8GWCL7E
type: implementation-checklist
title: 'Implementation: Mail card at phone width, settings language and info, empty sidebar group, double not-found title, create sheet cut off'
started: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (mailrender compat, dataentryconfig languages, staticshell, language, LanguageSetting, useShellAppConfig, sidebarNav, EntityList.empty)
- [x] Integration tests written (mobile-layout e2e: the create sheet at 375 px)
- [x] Happy path implemented
- [x] Edge cases from planning handled (an unsupported or duplicate language, app.language outside the list, a stored choice the app no longer offers, a group whose list has not loaded)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
rela-server on a copy of the pool project with `languages: [en]`: Settings shows no language card, a neutral notification text and no App Info for a member; the not-found page shows its title once; "Recent notes" shows "None yet". The digest golden file is 359 px wide with 16 px padding at 375 px and 600 px at desktop width. At 375 px the create sheet's body has no sideways scroll. Full vitest, vue-tsc, eslint, golangci-lint, go-arch-lint, commentlint, the mobile e2e suite and the Go tests of the changed packages pass.

## Quality

- [x] Code follows project patterns (check similar code) (app.languages follows app.language: config key, validation, /config field, shell meta, bootMeta)
- [x] Checked for DRY opportunities (the offered list is resolved once in locale.ts)
- [x] No security issues introduced (shell entries are filtered to base language codes and escaped in the meta)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
