---
id: BUG-YI096TR
type: bug
title: Mail card at phone width, settings language and info, empty sidebar group, double not-found title, create sheet cut off
description: The mail card is 600 px wide on a phone; the Settings page offers languages the app does not use, says "rela" and shows App Info to every user; an empty sidebar group shows a bare heading; the not-found page says "Page not found" twice; the create sheet on a phone is cut off on the left.
priority: medium
effort: s
why1: The mail card had a fixed width of 600; the language list was the app's built-in list; the notification text named the product; the App Info card had no gate; a generated sidebar group with no entries kept only its heading; NotFoundPage put the title in the page header and in the empty state; the create sheet's body was wider than the sheet because the editor and the wizard action bar pulled out past its edges.
why2: Each part was built for one setting (desktop mail, a single-product app, a filled sidebar, the list page's empty state, the full page form) and reused elsewhere unchanged.
why3: There was no config key that says which languages an app offers, and no flag that tells the sidebar a list has loaded empty.
why4: Tests checked each part only where it was first used, such as the mail golden file at one width, the settings card with every language, the sidebar with entries, the empty state on a list page, the form on a page.
why5: Phone widths and white-label apps were checked by hand in browser tests, not by the test suite.
prevention: A compat test pins the fluid mail card; app.languages is validated and tested from config to the settings card; the sidebar test pins the empty label; a test counts the not-found title; an e2e test at 375 px fails when the create sheet scrolls sideways.
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

## Description

Found in the ninth browser test of the saas-dev setup (pool and harbourmaster).

- The digest mail card is 600 px wide on a 375 px phone, so the mail scrolls sideways.
- The Settings page offers English and Dutch in an English-only app.
- The notification setting says "while rela is in the background".
- Every user sees the App Info card.
- "Recent notes" in the sidebar is a heading with nothing under it.
- The not-found page shows "Page not found" twice.
- At 375 px the "New Note" sheet is shifted left, so its left edge is cut off.

## Acceptance Criteria

- [x] The mail card is at most 600 px and fills a narrower screen, with less padding below 480 px; Outlook keeps its 600 px table.
- [x] `app.languages` limits the languages the app shows and offers; with one language the Settings page offers no choice.
- [x] The notification text does not name the product.
- [x] Only a user who may configure the app sees App Info.
- [x] An empty generated sidebar group with a create says "None yet" once its list has loaded.
- [x] The not-found page shows "Page not found" once.
- [x] At 375 px the create sheet fits the screen and does not scroll sideways.
