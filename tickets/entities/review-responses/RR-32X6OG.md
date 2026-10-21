---
id: RR-32X6OG
type: review-response
title: 'Measured: the cache is slower than the scan it replaces; drop it instead of redesigning it'
finding: 'The plan defers "drop the cache" to "measure before removing it". Measured on tickets/ (8,640 entity files in 17 type dirs, 10,413 relation files, warm APFS, standalone program mirroring scanEntityDirs + scanRelationDir): full entity scan 7 ms, relation scan 12 ms, total about 19 ms; reading and json.Unmarshal of the current 1.6 MB fsstore-index.json alone takes 22 ms, before the per-entry map inserts and the entitiesDirMtime ReadDir that the cache path also does. On top of that every Close marshals 1.6 MB and fsyncs it. So the cache has negative value today, and the per-directory design adds more format, per-dir bookkeeping, a clock seam and a racy-window rule to keep a net loss. Change the approach to: delete the persisted entity/relation index (loadPersistedIndex, savePersistedIndex, persistedIndex, the mtime helpers used only by it), always scan in syncIndex, remove app.FSFactory.DropStoreIndex / appbuild.DropStoreIndex and the call in cmd/rela-server/serve.go retire(), and reduce Config.IgnoreIndexCache to a no-op or remove it with its factory wiring (fsimport/state.go lists the file name too). Remove a stale fsstore-index.json once, or leave it ignored. This removes every race class below in one step. If cold-cache numbers are a worry, measure a cold open (purge) before keeping any cache; keep the per-directory design only if that measurement shows a clear win.'
severity: significant
resolution: 'Plan changed: the persisted index is removed and every open scans; the measurement is recorded in the plan and a benchmark (AC5) makes it repeatable.'
status: addressed
---
