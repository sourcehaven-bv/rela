---
id: RR-ICB0MX
type: review-response
title: Audit face match relied on substring
finding: renameRecordFaces used strings.Contains on the summary, so face draft would match face drafts.
severity: nit
resolution: The helper compares the exact summary string.
status: addressed
---
