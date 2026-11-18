---
id: RR-J2IWTO9
type: review-response
title: One append per record inside a write
finding: Each record was appended with its own savepoint and chain lock round trip.
severity: significant
resolution: Records of one write are batched on the transaction (txBatch) and appended with one AppendAll call.
status: addressed
---

Each record was appended with its own savepoint and chain lock round trip.
