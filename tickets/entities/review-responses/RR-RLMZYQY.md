---
id: RR-RLMZYQY
type: review-response
title: Handler and engine not-found bodies can differ
finding: The engine's ErrEntityNotFound inside a relation BatchOpError must map to the same target_not_found body and pointer as the handler's precheck.
severity: minor
resolution: 'There is one not-found path: the engine''s BatchOpError{End} with ErrEntityNotFound, mapped once to target_not_found. TestBatch_HiddenEndIsIndistinguishableFromMissing compares the bodies byte for byte.'
status: addressed
---
