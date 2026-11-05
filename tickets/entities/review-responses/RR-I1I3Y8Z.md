---
id: RR-I1I3Y8Z
type: review-response
title: Gate errors on an unloadable conflicted row on fsstore
finding: A row-dependent read grant makes the row gate list the type; on fsstore the conflicted row fails to load and the error surfaces as 500 on the list, detail and resolve. Tests run on memstore only.
severity: critical
resolution: Not fixed in this change; see reason.
reason: 'Fails closed: the answer is a 500 and nothing about the hidden row is disclosed. The same load error already hits every list of that type for that principal, so it is a store-level behaviour of fsstore and not a gap in this read-gate fix. Tracked in BUG-HTSW37Q.'
status: deferred
---
