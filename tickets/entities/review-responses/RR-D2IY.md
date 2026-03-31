---
id: RR-D2IY
type: review-response
title: Silent error swallowing in checkRule
finding: In validation.go checkRule(), filter parsing errors are silently ignored with return nil. If a validation rule has a malformed filter, the rule is silently skipped.
severity: critical
reason: This is pre-existing behavior in the validation service, not introduced by this ticket. Fixing requires changing the Service.Check signature which would be a breaking change. Should be addressed in a separate ticket.
status: deferred
---
