---
id: dataentry-readout-visibility-gate
type: automated-measure
title: Read-out reads in internal/dataentry go through the visibility seam, not the raw store
description: Structurally enforce that read-out reads in internal/dataentry go through the visibility seam (visibleReader/scopedSortedEntities/visibleRelationIDs) rather than the raw store, so a new read-out reader cannot silently ship ungated — the systemic gap (BUG-9Z20WH why3) that left executeView's traversal unwrapped. Either an arch-lint/grep rule flagging raw store reads outside an allowlist (the seam + sanctioned write-prep paths), or an enumeration test. Implemented as part of BUG-9Z20WH.
kind: test
location: internal/dataentry (arch-lint rule or enumeration test — TBD during BUG-9Z20WH implementation)
status: proposed
---

## Purpose

Structurally enforce the CLAUDE.md rule "read-out paths go through visibility
wrappers, base readers stay ungated" (DEC-ZBI39P) so a new read-out reader
cannot silently ship ungated — the systemic gap that let BUG-9Z20WH's
`executeView` traversal stay unwrapped for so long (its why3).

## Shape (decide during BUG-9Z20WH implementation)

Two candidate mechanisms, pick one:

1. **Grep / arch-lint rule** — flag `store.GetEntity` / `ListEntities` /
`ListRelations` calls in `internal/dataentry` outside an allowlist (the
visibility seam itself: `visibleReader`, `scopedSortedEntities`,
`visibleRelationIDs`, and the sanctioned write-prep paths that must stay raw per
"never redact a read that feeds a write").
2. **Enumeration test** — a test that lists read-out entry points and asserts
each resolves through a gated reader.

The allowlist is the load-bearing part: write-prep reads (entitymanager diffing,
`luaUpdateEntity`) MUST keep raw store access, so the rule cannot be a blanket
ban.

## Status

`planned` — implemented as part of BUG-9Z20WH (its acceptance criteria require a
prevention measure). This entity is the placeholder the bug's `adds-measure`
relation points at; flip to `active` and fill `location` with the concrete file
when the rule lands.
