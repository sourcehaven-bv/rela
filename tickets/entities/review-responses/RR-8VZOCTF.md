---
id: RR-8VZOCTF
type: review-response
title: 'Design: restored rows trust element values saved at mark time'
finding: Unmark copies saved element values back; a future rewrite of marked rows could restore stale values.
severity: minor
resolution: 'Plan changed: Unmark strips the element keys from the saved jsonb (pg) and sqlite''s marked tables do not carry the columns, so restored rows are always rehashed in the same transaction.'
status: addressed
---
