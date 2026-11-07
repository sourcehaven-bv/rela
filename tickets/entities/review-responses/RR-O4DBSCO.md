---
id: RR-O4DBSCO
type: review-response
title: '[security] Rules for gating and stored-hash use missing from reconcile doc'
finding: '[security] The package doc did not say that a range must be gated before the fold or that a stored content hash may stand in for the view hash only per row where the reader has no hidden field. A later SQL step could sum over hidden rows or fingerprint a redacted row''s stored hash and create an offline oracle (BUG-J01V0Q).'
severity: minor
resolution: The reconcile package doc lists three binding rules (gate before folding; stored hash only per row without hidden fields; relation endpoints readable) and says a matching fingerprint is never evidence of integrity or authorization.
status: addressed
---
