---
id: TKT-ICADEGW
type: ticket
title: Natural-language date input
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

Let a user type a date in words in a date field, in English or Dutch:
"tomorrow", "next friday", "in 3 days", "morgen", "volgende vrijdag", "over 2
weken", "5 maart". The date widget keeps its native date input and gains a text
box beside it. Enter or leaving the box turns the phrase into an ISO date and
sets the field. A phrase it cannot read is flagged and changes nothing.

Parsing is a pure function in the SPA. Both languages are always accepted,
whatever the interface language.
