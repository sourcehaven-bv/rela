---
id: RR-3AMR
type: review-response
title: Spec inlines selectors via page.evaluate, violating Page-Object Pattern
finding: e2e/tests/markdown-editor.spec.ts:34 attaches a request listener inline, and lines 66-69 call appPage.evaluate with a hardcoded `.editor-toolbar .fa-bold` selector — the eslint rule bans raw selectors in specs (FORBIDDEN_SELECTOR_METHODS) and the new code is the first/only page.evaluate call in any *.spec.ts file. Move the listener wiring and the getBoldToolbarBeforeFontFamily logic into FormPage.
severity: critical
resolution: Moved request-tracking out of the spec entirely (now in the appPage fixture, fixtures.ts:316-358) and the bold-toolbar font-family check into FormPage.getBoldToolbarIconFontFamily (form.page.ts:237-249). The spec no longer contains raw selectors or page.evaluate calls.
status: addressed
---
