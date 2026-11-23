---
id: BUGA-OK3XJU0
type: bug-analysis-checklist
title: 'Analysis: Rendered mail ignores the app palette and logo, and its text part drops email addresses'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (mail.render with a project palette: the HTML holds #4772fb and no img; an intro with a bare address: the text part lacks it)
- [x] Minimal reproduction steps documented (unit tests in internal/lua and internal/mailrender fail before the fix)
- [x] Environment/conditions noted (any project with a palette or app.logo; any message with an email address)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (mailbrand.Source reads the accent and app.logo; ReadDeps.MailBranding feeds mail.render; scheduled mail uses the same source; mailrender gains LogoURL; the text walker keeps email addresses)
- [x] Regression test planned (mailbrand, mailrender, lua and scheduled-mail tests)
- [x] Related areas checked for similar issues (scheduled mail had the same gap and is fixed here; the Settings-uploaded logo is not public, so it is not used)
