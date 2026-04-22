---
id: RR-1FA8W
type: review-response
title: 'entity_type must remain required for script: docs'
finding: Current validator (validate.go:970-984) requires both command and entity_type. Plan's new rule speaks only to {command, script} mutual exclusion and says nothing about entity_type. AC2 needs to assert entity_type is still required; plan's DocumentConfig snippet needs to make the invariant explicit.
severity: significant
resolution: entity_type remains required; validator error message explicit; AC2 covers missing-entity_type case.
status: addressed
---

From design-review on PLAN-78HJO. Direct follow-on from #2 — without required
entity_type the handler's type check has nothing to compare against.
