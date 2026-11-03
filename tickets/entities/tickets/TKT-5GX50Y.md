---
id: TKT-5GX50Y
type: ticket
title: Translate end-user screens of the data-entry SPA to Dutch
kind: enhancement
priority: medium
effort: xl
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

## Description

Extract the remaining end-user interface strings of the data-entry SPA into the
message files and add Dutch translations. Builds on TKT-ZL1QAO (i18n
infrastructure). Brief: G6 ticket 2.

**Waits on the G3 dashboard-cards PR**: it touches the same Vue files. Start
only after that PR has merged.

## Scope

- Screens: lists, forms (incl. wizard), entity detail, navigation, search,
dashboard, kanban, calendar, gantt, piles, comments, channels, actions and
flows, next actions, toasts and confirmations, empty and error states.
- Dutch wording: draft `nl` and add a glossary of chosen terms to the PR body
for review (Save = Opslaan, Delete = Verwijderen, Pile = ?). Use plain,
informal-neutral Dutch ("je", not "u") unless operator labels suggest otherwise;
ask in the PR.
- Shrink the bare-string allowlist accordingly. E2E tests keep passing; prefer
role or test-id selectors over English text where tests break.

Out of scope: Configure space and Settings internals (separate ticket).
