---
id: RR-Y75YD
type: review-response
title: SerializedReadModifyWrite is a probabilistic regression detector
finding: storetest's lost-update counter test is deterministic when serialization works, but as a detector of BROKEN serialization it is probabilistic — a lucky scheduler could serialize unserialized code by chance and produce a green run. Inherent to lost-update tests; reviewer recommended documenting rather than rewriting.
severity: minor
resolution: 'Documented the limitation in the test comment (internal/store/storetest/tx.go): a green run is strong evidence, not proof; a red run is always a real bug. The pgstore case is stronger in practice (without the advisory lock, READ COMMITTED all but guarantees a lost update at this concurrency).'
status: addressed
---
