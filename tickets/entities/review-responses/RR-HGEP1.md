---
id: RR-HGEP1
type: review-response
title: HTTP record_id parse errors silently coerced to newest lifetime
finding: 'internal/dataentry/relation_history_handler.go parseRecordID returned 0 (newest) on an unparseable ?record_id=, so a client sending record_id=abc or an overflow got the NEWEST lifetime with a 200 instead of a 400 — a silent wrong-answer, serving a different lifetime than asked for with no signal.'
severity: significant
resolution: parseRecordID now returns (int64, ok bool); a non-empty but unparseable or non-positive value yields ok=false and the handler returns 400 invalid_record_id. Covered by TestRelationHistory_BadRecordIDIs400.
status: addressed
---
