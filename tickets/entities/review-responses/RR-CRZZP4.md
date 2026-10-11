---
id: RR-CRZZP4
type: review-response
title: Count errors read as zero in schema and prompt
finding: tools_schema.go and prompts.go discarded count errors with count _ :=
severity: significant
resolution: 'Schema DTO counts are now *int with omitempty: a failed count is left out and not shown as 0. The summary prompt returns the error. Pinned by TestSchemaCounts_FailureIsNotZero.'
status: addressed
---
