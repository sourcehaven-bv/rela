---
id: RR-FA92E95
type: review-response
title: Undo of a soft-deleted row after its delete was sent
finding: A local delete in the web is a soft delete. If the run sends the delete and the person then undoes it, the local row returns with a ref to a remote row that is gone; the ledger has forgotten it and the push skips rows whose ref is set.
severity: minor
reason: Narrow window; the row stays local and unlinked, as a dropped row does today. Documented; a follow-up could clear the ref on restore.
status: deferred
---
