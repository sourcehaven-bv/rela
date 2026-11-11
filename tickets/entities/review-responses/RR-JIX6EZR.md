---
id: RR-JIX6EZR
type: review-response
title: Template subject reads are not checked against run_as read access
finding: 'An output template can copy subject.<prop> into an entity the run_as identity creates. Load checks refuse properties a visible: grant restricts, but not a subject type run_as cannot read at all. Row-level read can depend on role_relations, which a load-time check cannot decide.'
severity: minor
resolution: 'Fixed in TKT-9IT3LN8: templates read the subject as run_as through the gated reader; a subject run_as cannot read gives empty placeholders (TestStart_TemplateSeesOnlyWhatRunAsReads).'
reason: The runtime is not in this PR. The runtime ticket must evaluate templates through the run_as-gated reader, so an unreadable subject yields no value. Recorded in the split section of TKT-XHANLJU.
status: addressed
---
