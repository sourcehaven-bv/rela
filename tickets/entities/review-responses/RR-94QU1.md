---
id: RR-94QU1
type: review-response
title: Pre-write plaintext hash is dead weight under encryption
finding: markdown.go writeDataFile records hashContent(content) BEFORE s.bytes.WriteFile. Under production stack cryptofs.FS(SafeFS(OsFS)), the bytes on disk are ciphertext, so the pre-recorded plaintext hash is guaranteed NOT to match. Design only works because SafeFS OnPostWrite hook overwrites the LRU with ciphertext hash. If seal fails (empty recipients), hook never fires, stale plaintext hash remains in LRU until eviction. The pre-write record is pure ceremony in the sole branch anyone runs in production. Comment at markdown.go:507-516 normalizes this — red flag, not justification.
severity: significant
resolution: Dropped the pre-write s.recordHash from writeDataFile. The post-write observer on the bottom-most FS is now the sole source of self-echo hashes. MemFS gained its own OnPostWrite hook (mirroring SafeFS) so tests still have a working observable stack. The watcher_internal_test helper subscribes s.RecordWrite at store construction. External-edit simulation in tests uses the new MemFS.WriteFileExternal method to avoid tripping the self-echo hook.
status: addressed
---
