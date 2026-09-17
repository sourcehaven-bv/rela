---
id: TKT-MG2FXQ
type: ticket
title: 'RelationPicker: separate dropdown search from linked-ID type resolution'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

RelationPicker's `candidates` array serves two purposes with opposite cost
profiles: it supplies the dropdown options (wants a bounded, searchable set) and
it is the ONLY ID→type lookup table for `buildOutgoingTypes` (wants exactly the
already-linked IDs, typically 1-5). Conflating them is what turned a paging
boundary into a data-loss bug — BUG-HOB9BR.

BUG-HOB9BR fixed the data loss by fetching every page. That is correct and is a
no-op for typical projects (one page, one request), but it scales the cost with
collection size to solve a problem whose size is the number of linked edges. At
5,000 entities of one target type it is 50 sequential round-trips per type, and
`loadCandidates` loops target types sequentially too. At the 50-page cap the
original bug returns (there is now a `console.warn` for that case).

## Check this first

This may need no new endpoint. The entity GET already runs `include=*`, so if
`included` carried each neighbour's `type`, `buildOutgoingTypes` could resolve
pre-existing links with **zero** extra requests. The dropdown could then move to
server-side search — the pattern `EntityPickerModal` already uses via
`searchEntities`, complete with an `AbortSignal`. That would also retire the
client-side `.filter()` over the full candidate set on every keystroke, which is
the real ceiling on large collections.

## Related

- BUG-HOB9BR — the data-loss bug this conflation caused
- RR-IT4HSP — the review finding that asked for this ticket
