---
id: RR-Z62OFQK
type: review-response
title: Endpoint lookups run one id at a time under the write lock
finding: About 200 single-id family reads inside the Tx for a full batch.
severity: nit
reason: 'Bounded by the 100-operation limit; batching the reads belongs with minting ids once per type: TKT-Y58LS2N.'
status: deferred
---
