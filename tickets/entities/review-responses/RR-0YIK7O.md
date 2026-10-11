---
id: RR-0YIK7O
type: review-response
title: Catch-all 422s still send raw backend error text
finding: Create, ValidateCreate, patch/put, relation fallback, create relation, attachment write/delete, history restore and view execution answer any unclassified error as 422 validation_failed with err.Error(), so a store fault reaches the client. Raised by both reviewers.
severity: significant
reason: Deferred to BUG-E3V44J. The issue (#1774) scopes the 500 paths. Fixing the 422 catch-alls needs a per-handler inventory of which errors are client errors, and misclassifying one turns a useful validation message into an opaque 500. That is a separate change with its own tests.
status: deferred
---
