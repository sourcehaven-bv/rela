---
id: RR-F3IIK3
type: review-response
title: 'Test gaps: world-selected face, manifest read fault'
finding: TestReadAddress only used the zero world; no test covered filterVisibleManifest failing on a header read fault; the relation-source test pinned the fallback.
severity: minor
resolution: Added TestReadAddress_WorldSelectsAFace, TestFilterVisibleManifest_SourceReadFaultFails and rewrote the relation-source test as TestRelationSources_ReadsTheTailFace.
status: addressed
---
