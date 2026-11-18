---
id: RR-EX35HSI
type: review-response
title: Automation property writes are not recorded
finding: writeAutomationProperties (cas.go) writes the properties an on-create automation sets after the create, with no audit record; docs claimed automation writes were all covered.
severity: significant
reason: 'Predates this ticket: an on-create automation''s property write was never recorded on any tier, so this change does not widen the gap. docs/audit-log.md now states it, and follow-up TKT-LM1EOUI records it.'
status: deferred
---

writeAutomationProperties (cas.go) writes the properties an on-create automation
sets after the create, with no audit record; docs claimed automation writes were
all covered.
