---
id: RR-Q4P2P8
type: review-response
title: Cache validity ignores the schema (dir-to-type mapping), so a schema change restores a wrong index
finding: 'The cache is restored whenever directory mtimes match, but the entries depend on the schema too: scanEntityDirs skips a directory whose name does not resolve through buildPluralToTypeMap and stamps the resolved type on every entry. Adding a type (or a plural) for a directory that already holds files leaves that directory''s mtime unchanged, so the cache restores the old index without those entities; removing or renaming a type restores entries of an undeclared type, or entries whose entityFileKey now points at the wrong directory. rela-server''s generation swap on a config save is exactly the path where the schema changes and a cache from the previous schema is read. The plan''s per-directory comparison keeps this bug. If the cache is kept, each cached directory must record the type it resolved to (or the cache must carry a hash of the plural map), and a directory whose current resolution differs is rescanned; add a test that adds a type for an existing directory and reopens.'
severity: significant
resolution: 'Resolved by removing the cache: the index comes from the listing under the current layout on every open; AC2 pins a schema change.'
status: addressed
---
