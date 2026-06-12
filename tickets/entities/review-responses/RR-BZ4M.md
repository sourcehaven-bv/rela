---
id: RR-BZ4M
type: review-response
title: sidebarCounts.typeCounts is cached per request struct — aliases counts across principals
finding: 'Plan addresses sidebar `countWithFilters` for the ACL path but the existing unfiltered branch `return c.typeCounts[entityType]` reads from a precomputed map populated once per sidebarCounts construction. If sidebarCounts (or its parent response builder) is ever cached across requests (which the codebase doesn''t appear to do today, but the cache structure invites it), principal A''s counts would be served to principal B. Even without cross-request caching, the in-request c.typeCounts pre-population happens BEFORE per-list ACL filtering — wasted work and a latent aliasing bug. Fix: when ACL is configured, drop the precomputed typeCounts entirely and always go through GraphCount/GraphQuery; the cache buys nothing when each list has different ACL visibility. Additionally: the technical-approach bullet only addresses the unfiltered branch. The filtered branch (`countWithFilters` when `len(filters) > 0`) calls listFromStoreByTypes unconditionally — pre-ACL — and applies config filters on the full set. Wire ACL there too (run GraphQuery → apply config filters in-memory → count).'
severity: significant
reason: 'Moved to TKT-VMD8 (sidebar/aggregate PR). Documented there: drop typeCounts cache when ACL active, gate both filtered and unfiltered branches via GraphCount/GraphQuery. TKT-VQGN (PR 1) does not touch sidebar.'
status: deferred
---
