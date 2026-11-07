---
id: RR-1YO1UJ4
type: review-response
title: 'Design: heal needs a partial index and progress rule'
finding: The heal scanned for NULL rows without an index, could spin on a full batch without progress, and could abort version capture.
severity: significant
resolution: 'Plan changed: partial index on the key WHERE element_hash IS NULL, key-ordered batches, the noteBatch progress rule, and heal errors are logged and never abort the tick.'
status: addressed
---
