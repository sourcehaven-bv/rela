---
id: TKT-ZL1QAO
type: ticket
title: i18n infrastructure for the data-entry SPA (vue-i18n, en+nl, language resolution)
kind: enhancement
priority: high
effort: l
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

## Description

Add the i18n layer to the data-entry SPA so its own interface text can be shown
in Dutch or English. This ticket builds the infrastructure and converts one
slice (navigation sidebar and account menu) to prove it. Brief: G6 ticket 1.

## Scope

- vue-i18n (composition API) in the SPA and in `rela-components`. The library
must not depend on the app.
- Message files `en` and `nl`, keyed by area (`nav.*`, `account.*`, ...).
- Language resolution, first match wins: user preference (per-user settings,
selector on the Settings page), `app.language` in `data-entry.yaml` (validated
BCP 47, only languages with a message file), browser language, `en`.
- `Intl` date and number formatting follow the resolved language.
- `<html lang>` follows it.
- API error codes map to messages; unknown codes fall back to the server
`message`.
- Guard: bare template strings are a lint error, except in an allowlist of
existing files that may only shrink.
- CI check: every key present in `en` and `nl`; no unused keys.
- Convert the navigation sidebar and the account menu.
- Docs: data-entry guide covers `app.language` and the user choice.

Out of scope: extracting strings from other screens (ticket 2), Configure and
Settings internals (separate backlog ticket).
