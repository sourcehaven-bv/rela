---
id: FEAT-36AVYG
type: feature
title: Localized data-entry UI (Dutch and English)
description: The data-entry SPA chrome is translatable; Dutch and English ship first. Language follows the user preference, then app.language, then the browser.
status: proposed
---

## Summary

The data-entry web app shows its own interface text (buttons, menus, empty
states, errors) in the reader language. English and Dutch ship first.
Operator-authored text (schema labels, list and form titles) is already in the
operator language and is out of scope.

Language resolution, first match wins: user preference, `app.language` in
`data-entry.yaml`, browser language, English.
