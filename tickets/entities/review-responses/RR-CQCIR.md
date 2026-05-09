---
id: RR-CQCIR
type: review-response
title: Watcher parseEntityFromPath bypasses the magic-header check
finding: 'internal/store/fsstore/watcher.go reconcileEntityPath (line ~180) and parseEntityFromPath (line ~307) read raw bytes via s.rawReader.ReadFile(path) and feed them directly to parseDocument, bypassing readEntityFile/readRelationFile. The plan only hooks the latter. Result: encrypted files arriving via fsnotify get parsed as garbage and may overwrite a valid entry''s index state with empty ID/type. Fix: insert isGitCryptEncrypted check at the lowest common point (readDataFile, or a helper used by both readEntityFile/parseEntityFromPath). Also add an explicit AC + integration test: ''when git-crypt unlock decrypts files in place, the watcher transitions entries from inaccessible to live without manual reload.'''
severity: critical
resolution: Resolved by inserting the magic-header check at readDataFile (the lowest common entry point used by readEntityFile, readRelationFile, AND parseEntityFromPath in the watcher). Single insertion, no missed paths. Plan AC1 and AC13 (watcher transition test) cover this.
status: addressed
---
