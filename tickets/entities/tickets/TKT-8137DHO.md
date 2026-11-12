---
id: TKT-8137DHO
type: ticket
title: 'Flows M1g: cancel runs when their subject is deleted'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Split from TKT-FWQZGAM. Deleting a subject cancels its runs. A rename of a
subject that a run holds is already refused (TKT-XI7IDBS, #2190), and the fs
tier rekeys a run that raced the rename (`flowrun.Store.RekeySubject`).

- Cancel the unfinished runs of a deleted subject. RR-DTSDBII: part 1
handles the start race; this ticket handles a delete after the start.
- A crash between the delete and the cancel must not leave a run on a
missing subject for good; the flow worker can sweep for such runs.

## Acceptance

- Deleting a subject ends its runs as cancelled, on fs, postgres and
sqlite.
