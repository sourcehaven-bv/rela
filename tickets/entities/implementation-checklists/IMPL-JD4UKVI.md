---
id: IMPL-JD4UKVI
type: implementation-checklist
title: 'Implementation: Rendered mail ignores the app palette and logo, and its text part drops email addresses'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (mailbrand_test, mailrender_test, lua mailrender_test, scheduled_mail_test)
- [x] Integration tests written (scheduled template rendered through Services with a data-entry.yaml palette and app.logo)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no palette, Settings palette over the project palette, dark mode off, SVG or relative logo without base URL, unreadable palette)
- [x] Error handling in place (a branding read error is logged and the defaults fill in)

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
Go tests of mailbrand, mailrender, lua, mail, dataentryconfig, appbuild and dataentry pass; the scheduled-mail test renders the project accent and the absolute app.logo URL. golangci-lint, arch-lint, plimsoll and comment-lint pass.

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side interfaces for config and state reads, as BaseURLCarrier)
- [x] Checked for DRY opportunities (one branding source for Lua and scheduled mail; palette resolution reuses dataentryconfig.ResolvePalette)
- [x] No security issues introduced (logo URL limited to http(s) or root-relative, no SVG, no control characters; palette values pass mailrender's allowlist)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
