---
id: RR-ZOFENL
type: review-response
title: Pre-link and an incoming picker for the same relation collide on the inverse key
finding: An incoming RelationPicker for the pre-linked relation emits under the same inverse body key. Spreading the card body over the picker body dropped the pre-link when the user picked another peer, and the carried check refused the save.
severity: significant
resolution: mergeRelationsFields merges add/remove deltas per key instead of spreading; a full data replacement still wins. Unit-tested in relationsPatch.test.ts.
status: addressed
---
