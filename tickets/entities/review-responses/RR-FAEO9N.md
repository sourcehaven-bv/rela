---
id: RR-FAEO9N
type: review-response
title: Atomic path skips on-create automations and cascade
finding: createCore is the bare write; CreateEntity adds audit, alias notification, automations and cascade after it.
severity: significant
resolution: 'Plan: extract the post-create tail of CreateEntity into a shared helper and call it after commit; the relation audit follows.'
status: addressed
---
