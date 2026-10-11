---
id: RR-8Q0EW9
type: review-response
title: failPatch answers 422 with raw error text
finding: 'Inherited from writePatchError: a non-validation PatchEntity error becomes validation_failed with err.Error() and is not logged'
severity: minor
reason: Pre-existing in writePatchError for every HTTP write; this ticket must keep HTTP behaviour unchanged, so reclassifying belongs in its own ticket covering all write routes.
status: deferred
---
