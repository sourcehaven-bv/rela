---
id: RR-STMWU7P
type: review-response
title: 'Design: check must run on flows-only edits'
finding: prepare returns early when the schema is unchanged; a checker placed beside aclReferences never runs for a flows-only edit.
severity: significant
resolution: 'The checker runs before the early return. Test: a flows-only edit is refused.'
status: addressed
---
