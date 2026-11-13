---
id: RR-WMW35OX
type: review-response
title: Delete with an unknown relation route fails every run
finding: A deleting relation whose source entity row is gone counts Failed each run.
severity: minor
reason: Needs a ledger with a missing entity row, which the sweep and guard do not produce in practice; take remote clears it.
status: deferred
---
