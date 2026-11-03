---
id: RR-POIFJN
type: review-response
title: Mixed rela versions keep the bug alive and thrash the cache
finding: Several agents and the installed rela CLI may run different builds against one project. An older binary ignores the new version field, reads the new file, misses (good), but then saves the old format with save-time mtimes, which is the original bug; the new binary then misses on every open after it. Note this under Risks (the fix only holds once every binary touching the project has it), and consider a new file name (for example fsstore-index.v2.json) so old and new binaries do not overwrite each other's cache. Moot if the cache is dropped.
severity: minor
resolution: 'Plan updated: new binaries ignore fsstore-index.json; an older binary on the same project keeps its own behavior, which removing the file would not change. Recorded under Out.'
status: addressed
---
