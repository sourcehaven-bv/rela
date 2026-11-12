---
id: RR-XBAR16F
type: review-response
title: Link had no edit check on the local fields
finding: 'Security review: POST link was allowed with remote:sync alone. A link makes the next sync set the local ref and merge the remote row into the local one so it is a write to the local fields and needs the same edit rights as taking the remote value.'
severity: significant
resolution: verdict() now requires mayWriteRemoteValue for link; ActAgreesWithActions covers link with and without edit rights (403 and 200).
status: addressed
---
