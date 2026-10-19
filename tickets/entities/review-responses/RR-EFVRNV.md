---
id: RR-EFVRNV
type: review-response
title: Changing SeenShards later strands every entry
finding: The shard index is `fnv32(ref) % SeenShards`. Raising the constant puts every key in a new shard and floods `enter`. Record the shard count per rule in the index.
severity: nit
resolution: The shard count is recorded per rule in the index.
status: addressed
---
