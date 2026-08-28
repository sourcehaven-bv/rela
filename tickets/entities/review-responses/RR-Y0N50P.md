---
id: RR-Y0N50P
type: review-response
title: 'S1: no new/old distinction in an env for a change-triggered subsystem'
finding: 'Plan declared entity.<prop> only. But automations are change-triggered and every neighbouring mechanism is old/new aware: from:/becomes: trigger keys evaluated against oldValue/newValue at engine.go:206-227; interpolation uses {{new.title}}; Event carries Entity and OldEntity; project CLAUDE.md warns {{entity.title}} is wrong. Proposing entity. is a third spelling for the same object in the same YAML block - an API that cannot be changed after shipping.'
severity: significant
resolution: Env declares new and old matching {{new.x}}. entity is deliberately NOT declared. On create events old is the same RecordType bound to all-Nil; evalAttr already returns NewNil for missing fields (eval.go:113-121) so old.status == nil works. Pinned by a create-event test.
reason: 'Superseded by upstream TKT-J4IR1G phase 2b (PR #1315) which shipped an automation/validation condition env declaring `entity`. Changing it to new/old would now be a breaking change to released config rather than a free design choice. Revisit only if a change-aware condition (comparing pre/post state) is actually needed.'
status: deferred
---
