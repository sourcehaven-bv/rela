---
id: RR-2SKCFW
type: review-response
title: Fixed 16 shards cause heavy write amplification on postgres
finding: The seen-set rewrites whole shards. At 100k matches, a round touching all 16 shards rewrites about 5 MB per rule every 500ms on a busy system (risk 5 underplays this), all as `state_kv` row updates and WAL. Scale the shard count with the number of matches (recorded in the index), or give the database backends a keyed table. Add a budget AC on bytes written, not only on calls.
severity: significant
resolution: The shard count scales with the number of matches and is recorded per rule in the index. A budget AC pins bytes written per round.
status: addressed
---
