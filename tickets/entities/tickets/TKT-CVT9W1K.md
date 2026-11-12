---
id: TKT-CVT9W1K
type: ticket
title: 'Two-way sync: send deletes with guards'
kind: enhancement
priority: medium
effort: l
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

Two-way sync (DEC-2NFLAHB) sends no deletes yet: a local delete of a linked row
is recorded as dropped, and a row the remote stops listing follows `on_gone`.
The use cases (client portal, laptop and server, family hub) need deletes to
travel in both directions.

- Send a local delete of a linked row to the remote, through its normal
write API, with guards like `on_gone`: a cap per run and per day, refusal when a
type would be emptied, and an explicit confirmation above the cap.
- Decide how a delete on one side meets an edit on the other (conflict).

## Acceptance

- Two-instance test: deletes on each side travel; guards refuse a mass
delete.

## Notes

- A DELETE with `If-Match` compares the stored version inside the delete's
own transaction (`entitymanager.IfMatchDeletes`), so a write between the
compare and the delete is a 412.
- The handler side is the `conditionalDelete` type in data-entry, so
`writeHandler` stays on its plimsoll line of 40 methods.
