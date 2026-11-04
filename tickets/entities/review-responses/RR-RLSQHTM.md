---
id: RR-RLSQHTM
type: review-response
title: An existing end deleted between prepare and the Tx commits a dangling edge
finding: Existing ends are checked on the outer store before the Tx and not again inside it. A delete or type change in between commits an edge to a missing entity or with a disallowed type pair.
severity: significant
resolution: 'run() calls recheckEnds inside the Tx: every existing end is looked up again on the Tx view and the relation is validated again, so a deleted or retyped end fails the batch and rolls back. TestCreateBatch_EndDeletedBeforeTx.'
status: addressed
---
