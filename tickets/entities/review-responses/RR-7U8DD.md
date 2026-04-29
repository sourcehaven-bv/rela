---
id: RR-7U8DD
type: review-response
title: ResolvedField doc comment lies about its consumer
finding: 'internal/dataentry/helpers.go:159 — the comment ''Used by form templates to render property inputs consistently'' refers to the deleted templates. Falls out of finding #1: deleting the struct deletes the comment.'
severity: significant
resolution: 'Resolved by addressing RR-DSIWT: deleting the struct also removed the misleading comment.'
status: addressed
---
