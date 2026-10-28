---
id: RR-WOME2D
type: review-response
title: ListAddressed inner ORDER BY does not match the index direction
finding: 'The inner scan orders by `at DESC, channel_id DESC, seq DESC`, but `channel_mentions_target_idx` is `(target, at DESC, channel_id, seq)`. Neither SQLite nor Postgres can serve mixed directions from that index: SQLite adds a temp B-tree, and Postgres sorts. So the LIMIT 2000 is applied after reading and sorting EVERY row of the target, which is unbounded (1000 per channel times N channels). The explain test would catch it, but the design is wrong as written. Fix: declare the index as `(target, at DESC, channel_id DESC, seq DESC)` in both 0036 and channelMentionsDDL, or order the inner query by `at DESC, channel_id, seq` to match the index. Pin "no TEMP B-TREE for the inner ORDER BY" (SQLite) and "Index Scan with no Sort below the Limit" (pg) in the explain tests.'
severity: significant
resolution: Index is now (target, at DESC, channel_id DESC, seq DESC) in sqlite and pg; AC 13 explain tests pin no TEMP B-TREE (sqlite) and Index Scan with no Sort under the Limit (pg).
status: addressed
---
