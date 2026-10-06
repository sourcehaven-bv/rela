---
id: RR-44YDTU
type: review-response
title: FTS index keyed by unstable rowid
finding: entity_search is keyed by entities' implicit rowid, which VACUUM may renumber.
severity: minor
resolution: Not changed.
reason: rela never runs VACUUM; only an external tool would. Keying the index by a stable surrogate is follow-up work.
status: deferred
---
