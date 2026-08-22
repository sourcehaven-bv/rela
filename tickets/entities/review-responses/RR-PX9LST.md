---
id: RR-PX9LST
type: review-response
title: GC Scan ledgered rela-managed properties on entities (deleted managed state after grace)
finding: 'The relation loop in Scan skipped the managed order properties but the entity loop had no exclusion: any rela-managed or undeclared bookkeeping property on entities would be ledgered as an orphan and hard-deleted from every entity of that type after the grace period.'
severity: significant
resolution: Both Scan loops now skip the underscore namespace (`_`-prefixed property names), which is rela-managed and can never be schema-declared (the loader forbids declaring managed properties) — commit bddc13f3. Scan was also decomposed into scanEntities/scanRelations so the symmetric rule is visible. Pinned by TestGC_ScanSkipsManagedProperties.
status: addressed
---
