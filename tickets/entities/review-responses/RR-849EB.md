---
id: RR-849EB
type: review-response
title: Kanban DnD mutates DOM to set ID inside selector resolution
finding: Inside columnCards.evaluate(el => { if (!el.id) el.id = ... }) mutates page state as a side effect of a read-only step. Use elementHandle directly.
severity: nit
reason: Nit. DnD code is load-bearing and stable; refactoring risks breaking the fragile drag simulation. Defer to a dedicated test-infra cleanup.
status: deferred
---
