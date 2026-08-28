---
id: RR-H02GFD
type: review-response
title: Fix lacked tests for relation-column / collection / neighbor redaction surfaces
severity: significant
status: addressed
finding: >-
  The two original tests seeded a single entry entity with no relations and no
  collections, exercising only the entry-source `properties` section. They did
  not cover the collection/neighbor path (`viewReader.Filter`), table relation
  columns, or grouped headings — surfaces the ticket claimed as fixed. For a
  confidentiality fix, the redacted-collection and neighbor paths must be pinned.
resolution: >-
  Added `TestACLViews_RelationColumnRedactsHiddenNeighborTitle` and
  `TestACLViews_RelationColumnDropsUnreadableNeighbor`, which exercise
  neighbor/collection redaction + row-gate through `resolveRelationColumnValues`
  under a real gated context (`gateCtxFor`); both fail without the fix. The
  groupBy heading surface reads `e.Properties[prop]` off the now-redacted
  collection entities, so a hidden group-by property yields `(none)` — covered
  by the same collection-redaction mechanism the relation-column tests exercise.
  The remaining broad guarantee (a route-enumerated sentinel scan) stays tracked
  as the `acl-wire-boundary-sentinel-test` measure, still `proposed`.
---
