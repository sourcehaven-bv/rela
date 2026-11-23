---
id: RR-EG4DG3G
type: review-response
title: Sqlite-tagged desktop test not run in CI
finding: 'Design review: the per-backend desktop differential is mostly redundant with RunGraphDifferential, and a sqlite-tagged desktopnotify test only runs if CI runs the package with the tag.'
severity: nit
resolution: Kept a small table; added ./internal/desktopnotify/... to the sqlite-tagged CI job.
status: addressed
---
