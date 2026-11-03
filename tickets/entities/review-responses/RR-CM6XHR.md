---
id: RR-CM6XHR
type: review-response
title: fsimport package doc still says fsstore ignores its index cache
finding: 'internal/fsimport/fsimport.go:6-7: the Contract paragraph says the source "is read through a read-only filesystem, fsstore ignores its index cache, and no database is opened in it". The cache and the switch that ignored it (IgnoreIndexCache) are gone, so the sentence describes a mechanism that no longer exists, and a reader will look for it. Drop the middle clause: "it is read through a read-only filesystem and no database is opened in it".'
severity: minor
resolution: Dropped the index-cache clause from the internal/fsimport package doc.
status: addressed
---
