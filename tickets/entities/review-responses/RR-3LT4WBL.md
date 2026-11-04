---
id: RR-3LT4WBL
type: review-response
title: Engine tests miss the security cases the plan claims
finding: No unique-within-batch test, no hidden-end test at the engine (the boundary for a later Lua rela.tx), no create-ACL denial mid-batch, no concurrent id minting test.
severity: significant
resolution: Added TestCreateBatch_UniqueWithinBatch, TestCreateBatch_HiddenEndIsMissing, TestCreateBatch_DeniedCreateWritesNothing and TestCreateBatch_ConcurrentBatchesMintDistinctIDs; they run on memstore, fsstore, sqlite and postgres where applicable.
status: addressed
---
