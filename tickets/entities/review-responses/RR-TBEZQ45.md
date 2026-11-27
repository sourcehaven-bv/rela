---
id: RR-TBEZQ45
type: review-response
title: stop can leave the forwarder blocked
finding: 'Pre-existing: closing the input channel does not unblock a forwarder blocked on a full broker or store channel after the loop exits.'
severity: nit
reason: Pre-existing and needs more than 64 queued events; no test reaches it.
status: wont-fix
---
