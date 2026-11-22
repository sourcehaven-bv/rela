---
id: RR-W4O2TOQ
type: review-response
title: Entity ids in summary and triggered_by text are not gated
finding: copy-state summaries, mention-rewrite records and cascade/restore/session triggered_by strings name other entity ids that the per-page gate never sees.
severity: significant
resolution: 'Plan: /_changes serves a wire type built per op, not raw records. Summary passes only for ops whose summary is id-free; triggered_by is reduced to its kind (automation and schedule names kept, since config is not secret). A test names a hidden id in each producer and asserts it never reaches the body.'
status: addressed
---

copy-state summaries, mention-rewrite records and cascade/restore/session
triggered_by strings name other entity ids that the per-page gate never sees.
