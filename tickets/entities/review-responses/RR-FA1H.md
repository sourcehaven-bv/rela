---
id: RR-FA1H
type: review-response
title: initialServerSnapshot precedence over later recordServerSnapshot
finding: |
  Edge case row in PLAN says "second call overrides the first" -- correct because recordServerSnapshot does a full clear-and-rewrite. But the doc comment on initialServerSnapshot should say so explicitly, otherwise a caller passing both will be surprised.
severity: minor
status: addressed
resolution: |
  Add jsdoc to initialServerSnapshot: "Equivalent to calling recordServerSnapshot(entity) immediately after construction. Any later recordServerSnapshot call fully replaces this seed." One sentence -- done in implementation.
---
