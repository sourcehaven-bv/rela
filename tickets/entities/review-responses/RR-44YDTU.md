---
id: RR-44YDTU
type: review-response
title: FTS index keyed by unstable rowid
finding: entity_search is keyed by entities' implicit rowid, which VACUUM may renumber.
severity: minor
resolution: The index is keyed through entity_search_key (INTEGER PRIMARY KEY, UNIQUE(id, face)), which VACUUM keeps; schema v13 rebuilds existing indexes. Tested with moved rowids and a v11/v12 migration.
reason: rela never runs VACUUM; only an external tool would. Keying the index by a stable surrogate is follow-up work.
status: addressed
---
