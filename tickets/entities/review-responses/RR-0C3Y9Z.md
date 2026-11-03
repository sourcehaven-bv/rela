---
id: RR-0C3Y9Z
type: review-response
title: rooted.WriteFile is already temp+rename via SafeFS, but with a fixed temp name shared between processes
finding: 'The plan leaves AC6 open ("unless rooted.WriteFile already does this"). It does in production: storage.SafeFS.WriteFile writes path+".tmp", fsyncs, renames and fsyncs the directory, and appbuild wraps OsFS in SafeFS. But the temp name is fixed, so two processes closing at the same time open the same fsstore-index.json.tmp with O_TRUNC, interleave writes into one inode, and one renames a mix into place while the other''s rename fails. A torn file most likely fails json.Unmarshal (a cache miss, so not data loss), but the plan should not claim "never torn". Under MemFS (tests) the write is not temp+rename at all. If the cache is kept, either accept and document "torn file means cache miss" (and test it), or write with a unique temp name in the same directory. Do not add a second temp+rename layer on top of SafeFS.'
severity: minor
resolution: The cache write is removed; the shared temp name for other SafeFS writes is part of follow-up TKT-H63QDR.
status: addressed
---
