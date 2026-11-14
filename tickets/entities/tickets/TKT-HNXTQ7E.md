---
id: TKT-HNXTQ7E
type: ticket
title: 'Data entry guide: object list shows as bullets'
kind: docs
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

GitHub issue #2232. The data entry guide, section "Options as cards", said an
object list shows its items "as a table of fields" by default. The web app
(`RlChannelMessage.vue`) renders a bulleted list, one bullet per item with its
set fields as "label: value". GUIDE-realtime already describes this correctly.

## Fix

Correct the sentence in `GUIDE-data-entry.md` and regenerate
`docs/data-entry.md`.
