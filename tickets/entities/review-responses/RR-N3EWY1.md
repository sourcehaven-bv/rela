---
id: RR-N3EWY1
type: review-response
title: Crash-orphaned attachment temps now accumulate and nothing sweeps them
finding: 'internal/store/fsstore/attachment.go:80-84: the attachment temp name is now random per upload, so a process killed mid-upload leaves a new .rela-attach-tmp-<hex> file each time (the old name was derived from the file name and was reused on the next upload). No sweep covers this prefix: cleanupTempFiles walks only entitiesKey and relationsKey, and loadPropertyAttachments (fsstore.go:429) only skips it. The partial upload bytes stay on disk until the entity is deleted (OWASP PDR: purge temporary copies). Fix: in loadPropertyAttachments, remove files with attachTempPrefix whose mtime passes storage.IsStaleTemp, reusing the same staleness rule.'
severity: minor
resolution: 'Same fix as RR-61Q2TK: stale attachment temp files are swept on open and reported by rela gc --temp-files.'
status: addressed
---
