---
id: TKT-MK9NJB
type: ticket
title: Per-field version tokens for list-section rows
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

TKT-2VDVHF guards autosave with per-field preconditions. The tokens come from
the snapshot a form starts from (`_versions`). The entity detail page and a
view's entry carry `_versions`, but list-section rows in `SectionEditForm` are
built from collection reads that carry none. Such a row saves unguarded until
its first save returns tokens, so a concurrent edit to the same field in that
window is overwritten without a conflict message.

## Approach

Emit `_versions` on the rows a view section serves (computed from the same wire
projection as the entry), and pass them into `SectionEditForm` as
`initialVersions` for each row. Check the per-row cost against the collection
read budget (TKT-1U8XYN): the tokens hash values already loaded, so no extra
store reads are expected.
