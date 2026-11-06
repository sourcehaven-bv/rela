---
id: RR-H5GSRPG
type: review-response
title: HasConflictMarkers copies its byte input
finding: HasConflictMarkers converts the byte slice to a string.
severity: nit
reason: Only the read path calls it, once per file, and the cost is one copy of a markdown file.
status: wont-fix
---
