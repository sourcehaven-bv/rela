---
id: RR-E3IPT8D
type: review-response
title: Batch endpoint must run the data-entry create field gate
finding: The single create gates fields in the handler (validateCreateFieldWrite) and writes through the ungated h.manager. Plan C step 7 names a field-gated handle that the writeHandler does not have; calling CreateBatch with h.manager would skip the field gate (BUG-Q60V).
severity: critical
resolution: batchHandler runs validateCreateFieldWrite on every entity before CreateBatch and writes through the ungated manager, as the single create does. A denial is audited as denied-write and answers 403 with the operation. TestBatch_FieldGateDenialWritesNothing.
status: addressed
---
