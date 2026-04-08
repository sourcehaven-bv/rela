---
id: RR-AAW2
type: review-response
title: saveCacheQuietly can race itself across concurrent Reloads
finding: Two concurrent Reloads both reach saveCacheQuietly which writes the cache file. If SaveCache is not atomic-rename, the file can be torn. Verify SaveCache uses atomic rename; if not, serialize via the Reload mutex.
severity: minor
resolution: Fixed by adding reloadMu. Sync, Reload, and Close all take reloadMu, so two concurrent reloads cannot both reach saveCacheQuietly simultaneously. The cache write race is eliminated without needing to audit repo.SaveCache for atomic-rename semantics.
status: addressed
---
