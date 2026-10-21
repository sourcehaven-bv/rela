---
id: RR-A6J50L
type: review-response
title: sortStateKeys doc still speaks of several construction paths
finding: 'internal/store/fsstore/fsstore.go:553-557: "The index CONSTRUCTION paths must use the same comparator as the mutation paths". There is now one construction path (scanEntityDirs); the cached-restore path that justified the plural is gone. Change it to "The index construction path (scanEntityDirs) must use...".'
severity: nit
resolution: The sortStateKeys doc names the single construction path, scanEntityDirs.
status: addressed
---
