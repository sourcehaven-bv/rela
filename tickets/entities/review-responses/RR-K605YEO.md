---
id: RR-K605YEO
type: review-response
title: Condition check and start are not atomic
finding: 'Start checks the manual start''s when: and then calls Engine.Start; the subject can change in between, so a start can slip through just after the condition stopped holding.'
severity: minor
reason: A manual start is a person's request, checked against the subject they saw. The engine's per_subject and guard rules still apply after the start; a lock across both calls would hold a write lock around flow settlement for no gain.
status: wont-fix
---
