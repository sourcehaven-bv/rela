---
id: RR-5XNMEN
type: review-response
title: 'Swap did not bump updated_at, so the version sweep might never capture it'
finding: 'pgstore''s UPDATE set seq but not updated_at, and sqlitestore''s set neither. The version sweep selects candidates by updated_at (sweep.go:478), so a row that already looked settled could be rewritten and never selected. HashRelation folds the (from,type,to) triple into the content hash, so a swapped row genuinely HAS a new hash and would be captured if the sweep looked - it just might not look. TKT-9TQ6I left the atomic rename without this on the grounds that a miss costs only the rename marker and never lineage continuity; that reasoning does not transfer, because nothing captures a reversal synchronously, so a miss costs the version itself. Most likely in practice: rela migrate data is a short-lived CLI process that applies and exits, and the sweep is debounced.'
severity: critical
resolution: 'Both statements now set updated_at, with a comment naming TKT-9TQ6I and why its exemption does not apply. Pinned by a new conformance case (SwapMovesUpdatedAt) that runs on all four backends, and mutation-verified: removing updated_at from pgstore''s UPDATE fails it.'
status: addressed
---
