---
id: RR-SAO3KW
type: review-response
title: Per-channel row cap lets any poster erase other people's unread mentions
finding: 'The plan says "a flood cannot erase anything; old rows only drop out of the window". The cap of 1000 rows per channel DELETES rows with the lowest seqs. Anyone who may post can send 100 messages with 10 `@x` or job targets and remove every unread mention and reply in that channel. The rate limit slows this but does not stop it. The SQL cap statement also reads 1000 index entries on every append that has rows. Because it uses `seq <` on a non-unique seq, it keeps more than 1000 rows when seqs tie. Fix, in order of preference: (a) drop the cap on SQL, where storage is already about 10 small rows per message and the per-target scan bounds inbox cost; keep a cap only on the file tier for the file rewrite cost; (b) evict rows that are already read first (the marker is in the same directory or table), then the oldest; (c) cap per (channel, target). Whatever is chosen, correct the abuse paragraph in Security Considerations and docs/acl-security.md.'
severity: significant
resolution: Cap is now 200 rows per (channel, target), plus 5000 per file on fs; evicts read rows first, then the same poster's own unread rows, else skips the new row. Security text and AC 7 tests updated.
status: addressed
---
