---
id: TKT-CJBOQON
type: ticket
title: Natural-language date entry in date fields
kind: enhancement
priority: medium
effort: s
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

Date fields in the data-entry web app accept typed phrases such as "tomorrow",
"next friday" or "in 3 days". The widget shows the resolved date before the
value applies, and the form saves the ISO date as before.

## Approach

The parser is language-neutral. It matches a fixed set of phrase shapes
(relative day, weekday, "next"/"this" weekday, "in N units", "N units ago",
"next week/month/year", day-month with or without year) against a vocabulary
pack. Each language is one pack in `frontend/src/utils/naturalDate/locales/`,
registered by language code. The widget uses the active UI language and falls
back to English. English and Dutch ship as the first packs; a new language adds
one file and a test, with no parser change.

`DateWidget` gains a companion text box. The native date input stays, so
existing forms and tests are unchanged. No server or config change.
