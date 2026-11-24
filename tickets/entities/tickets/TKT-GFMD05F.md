---
id: TKT-GFMD05F
type: ticket
title: Natural-language date entry in date fields
kind: enhancement
priority: medium
effort: s
status: ready
---

## Description

Date fields in the data-entry web app accept typed phrases in English and Dutch,
such as "tomorrow", "next friday", "in 3 days", "morgen" or "volgende week". The
widget shows the resolved date before the value applies, and the form saves the
ISO date as before.

## Approach

Extend `DateWidget` with a companion text box. A pure parser in
`frontend/src/utils/naturalDate.ts` resolves the phrase against today's local
date. The native date input stays, so existing forms and tests are unchanged. No
server or config change.
