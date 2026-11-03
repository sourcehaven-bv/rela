---
id: RR-26TJZU
type: review-response
title: Source values can change between check and Tx
finding: Field visibility can depend on values; re-reading the source raw in the Tx without comparison could copy a value that became hidden.
severity: significant
resolution: 'Plan: readability check before the Tx; in the Tx refuse with a retryable source-changed error when a mapped source field differs, as requireInputsUnchanged does.'
status: addressed
---
