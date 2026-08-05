---
id: RR-FOD7IB
type: review-response
title: 'Sync GET (/api/sync/) emits relation meta + entity fields raw, bypassing visible:'
finding: GET /api/sync/relations/<from>/<type>/<to> serializes rel.Properties raw, gated only by the row-level read verdict (permitsSyncReadRelation) — never visibleRelationMeta. A principal who can read the source entity pulls the visible:-redacted meta keys straight off this endpoint, bypassing the redaction added to the v1 endpoints. Same gap exists for entity fields on the sync entity GET (inherited from TKT-73C6B2). The new guide text claimed 'every relation read shape' redacts, which overstated coverage.
severity: critical
resolution: 'Resolved as doc-and-defer, NOT by redacting sync (confirmed correct by re-review): the sync GET is a read that feeds a write — the client hashes the body and pushes it back under If-Match — so redacting it would erase the hidden fields on the authoritative store on next push (the ''never redact a read that feeds a write'' data-destruction bug). Corrected the doc overstatement to ''every browser-reachable relation read shape'' with sync called out as the deliberate exception, and added a ''Machine-to-machine sync (/api/sync/)'' entry to ''What still leaks (deferred)'' in both docs/acl-security.md and the docs-project mirror: documents what leaks (full canonical body, entity + relation), why it can''t be naively redacted, the mitigation (gate /api/sync/ behind a trusted-replica boundary — network/mTLS/dedicated sync principal), and names the round-trip-safe follow-up. Residual operational gap (mitigation is prose-only, no test/lint asserts sync isn''t exposed to ordinary readers) tracked as a follow-up ticket.'
status: addressed
---
