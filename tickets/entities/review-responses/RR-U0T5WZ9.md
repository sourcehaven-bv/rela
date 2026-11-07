---
id: RR-U0T5WZ9
type: review-response
title: Type-less automations and validations fire on managed writes
finding: An automation or validation without a type filter matches _flow-run and managed relations, so operator actions would run on engine writes.
severity: significant
resolution: 'Plan changed: type-less automation triggers and validation rules skip rela-managed types and relations.'
status: addressed
---
