---
id: RR-I7QWPZZ
type: review-response
title: InvalidRelationError and cardinality refusals counted as Failed
finding: Endpoint-type and cardinality refusals were counted as Failed, never recorded, and retried every run, so every run reported failed rows.
severity: significant
resolution: rejectedWrite treats ValidationError, InvalidRelationError, unique violations and ErrCardinalityExceeded as rejections recorded in the ledger. TestRejectedWriteAndUniqueOn.
status: addressed
---
