---
id: RR-V221YU
type: review-response
title: Relation-column titles leak hidden neighbor display value on the table-cell surface
severity: significant
status: addressed
finding: >-
  Code review of the executeView redaction fix found that it covered the entry
  and traversed-collection entities, but NOT the relation-column path in a table
  section. `sections.go` (table branch) calls
  `App.resolveRelationColumnValues` (helpers.go), which fetched each relation
  target with the RAW `svc.Store.GetEntity` and titled it via
  `DisplayTitle(...)` against raw properties (the old `entityTitle` helper). A
  table section with a relation column pointing at an entity whose display
  property is hidden by `visible:` leaked that value as the cell title — the
  BUG-R9EHKV title-leak class, on a surface BUG-9QL9XV's own why4 lists in scope
  (table cells / buildRow). It was also an UNGATED read: a neighbor the
  principal could not read at all still had its title rendered.
resolution: >-
  `resolveRelationColumnValues` now collects the relation targets and routes
  them through `viewReader.Filter` (row-gate + field-redact, DEC-ZBI39P) before
  deriving titles: an unreadable neighbor is dropped entirely, and a survivor
  with a hidden display property falls back to its id. The dead `entityTitle`
  helper was removed. Pinned by two new tests —
  `TestACLViews_RelationColumnRedactsHiddenNeighborTitle` (redacts hidden title)
  and `TestACLViews_RelationColumnDropsUnreadableNeighbor` (drops unreadable
  row) — both verified to FAIL against the pre-fix helper and pass after.
---
