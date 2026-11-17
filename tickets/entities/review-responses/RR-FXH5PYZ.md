---
id: RR-FXH5PYZ
type: review-response
title: successHold can answer for a tenant removed within the hold
finding: A webhook re-sent within 5 s of a success, for a tenant whose control row was removed, gets the old run and a 200 without a create.
severity: minor
resolution: 'Documented on successHold: erase leaves a tombstone, so only a hand-deleted control row can hit it, and the first request after the hold starts a create. Pinned by TestProvision_SuccessHoldEnds.'
status: addressed
---
