---
id: RR-TMJTK1N
type: review-response
title: ChangedProps has no nil contract
finding: changedProps(nil, e) panics and Write.Old documents no nil contract.
severity: nit
resolution: 'PrepareUpdate maps a nil old row to an empty entity; Write.Old documents Nil: rejected.'
status: addressed
---
