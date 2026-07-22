---
id: RR-HGEP2
type: review-response
title: RecordID + AllLifetimes not mutually exclusive at the store trust boundary
finding: 'RelationVersionPurgeRequest accepted both RecordID != 0 and AllLifetimes == true; resolvePurgeLineage put the AllLifetimes case first in the switch, so a non-CLI caller setting both got AllLifetimes silently, ignoring RecordID. The store is the trust boundary (the request struct is public API), not just the CLI.'
severity: significant
resolution: resolvePurgeLineage rejects a request with both RecordID and AllLifetimes set ("mutually exclusive") before any resolution. Covered by TestPurgeRelationVersions_RecordIDAndAllLifetimesMutuallyExclusive.
status: addressed
---
