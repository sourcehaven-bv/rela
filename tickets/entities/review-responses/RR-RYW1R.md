---
id: RR-RYW1R
type: review-response
title: DynamicForm and InlineCreateModal duplicate problem+json error parsing
finding: Near-identical 'parse problem+json or fall back to Error message' logic in two places. Primed for drift.
severity: nit
reason: Real but small. The two blocks are ~10 lines each, both feed the same toast surface, and neither has changed since the BUG-UNEBR fix. A formatFormError(err) util is the right destination but it's a refactor of pre-existing duplication, not new duplication introduced by this PR. Filing as a follow-up so the helper can be designed without being constrained by this ticket's commit boundary.
status: deferred
---
