---
id: RR-WBX0LY
type: review-response
title: Count and ratio cards redacted rows they never read
finding: Every folded card read and redacted the whole list; count and ungrouped ratio do not need redaction.
severity: minor
resolution: 'Count and ungrouped ratio skip redaction. Routing count through the pushed-down CountMatched is deferred: navStatusRows applies the list condition in Go, so a pushdown path would be a second implementation of the list pipeline to keep in step.'
status: addressed
---
