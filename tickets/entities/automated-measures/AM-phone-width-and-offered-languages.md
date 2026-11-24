---
id: AM-phone-width-and-offered-languages
type: automated-measure
title: Fluid mail card, create sheet at phone width and offered languages
description: A mailrender compat test pins that the card fills a narrow screen up to 600 px; an e2e test at 375 px pins that the create sheet does not scroll sideways; Go and vitest tests pin app.languages from config to the shell meta, the language resolution and the settings card.
kind: test
location: internal/mailrender/compat_test.go, e2e/tests/mobile-layout.spec.ts, internal/dataentryconfig/language_test.go, internal/dataentry/staticshell_test.go, frontend/src/i18n/language.test.ts, frontend/src/components/settings/LanguageSetting.test.ts
status: active
---

Fails when the mail card gets a fixed width again, when the create sheet's
body grows wider than the screen, or when a language the app does not offer
is shown or offered.
