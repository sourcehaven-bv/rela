---
id: RR-4D4UBM
type: review-response
title: 'TOCTOU: manifest-listed id can 404 on fetch (lost access between feed and fetch) — client must skip+advance, not retry-forever'
finding: 'Design-review S3. The row gate is evaluated twice on different store snapshots: manifest-read time (filterVisibleManifest) and fetch time (permitsSyncReadEntity). Fetch-time is authoritative (good for security), but the consequence must be in the client contract: a row visible in the manifest can 404 on fetch if access is lost in between. The client must treat a 404 on a manifest-listed id as ''skip, advance cursor'', NOT error/retry-forever, or a re-permissioning during a sync run wedges the client. Reverse direction is safe: a row hidden in the manifest but fetched by a client that remembers the id is still correctly denied at fetch time.'
severity: minor
resolution: 'Accepted as designed (2026-08-08). Fetch-time gate is authoritative (good). Documented in the sync client contract: a 404 on a manifest-listed id (access lost between feed and fetch) means skip + advance cursor, never retry-forever. No server change. Doc work folded into the implementation-phase docs-checklist.'
status: addressed
---

## Finding (design-review S3)

**Accept + document in the client contract.** Two-phase gate (manifest then
fetch) on a moving store means:
- manifest-says-visible → fetch-says-404 (access lost in between): client
**skips and advances**, never retries forever.
- manifest-hidden → direct fetch by remembered id: correctly 404'd at fetch time.

No server change; this is a documented client-behavior requirement. Belongs in
the sync client contract / docs.
