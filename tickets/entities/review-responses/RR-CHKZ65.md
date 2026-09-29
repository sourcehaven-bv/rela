---
id: RR-CHKZ65
type: review-response
title: Rename logging skipped alias rewrite when the post-rename read failed
finding: familyRename.record returned early when no renamed rows were available, so a rename that stood on fs got no alias rewrite and no relation versions, and the log line hid the cause.
severity: minor
resolution: record now logs and skips only the per-face records; the alias rewrite and relation rename versions always run. The read error itself is what RenameEntity returns.
status: addressed
---
