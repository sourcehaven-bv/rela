---
id: TKT-MQSK037
type: ticket
title: Nested generated sidebar groups with per-entry counts
kind: enhancement
priority: medium
effort: l
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

`items_from` fills a sidebar group from a list, one level deep, and its entries
carry no counts. Add one child level (`children: { list, via }`), loaded in one
batch for all parents, and `status:` counts on generated entries and their
children, computed after row-gating with one grouped read per group. The sidebar
payload stays principal-independent; rows and counts come from
`/api/v1/_nav_items`.

## Follow-up

Issue #2586: in the data-entry guide, the `status` and `children` rows of the `items_from` table sat after a blank line and rendered as a separate table. The blank line is gone, and the guide now says that only a group-level `status:` is refused next to `items_from`.
