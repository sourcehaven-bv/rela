---
id: RR-1DRR
type: review-response
title: 'Attribution side-channel: cross-principal bleed, TOCTOU, unbounded growth (C1/C2/C3)'
finding: 'attributionStore keys on (entityID, path) without the principal, so concurrent denials for the same path attribute to the wrong role (C1). Attribution is recorded on GET and read on a later PATCH via process-global sync.Map: a PATCH without a prior GET gets NO attribution (the common case silently no-ops), and the value can be stale (C2 TOCTOU). The map accretes one entry per (entity,path) for the daemon lifetime with no eviction (C3). The ''determinism makes races benign'' claim is false because the value embeds principal-dependent role= text.'
severity: critical
resolution: 'Deleted attributionStore entirely (L1). Attribution now rides the verdict: dataentry FieldVerdicts/RelationVerdicts gained an Attribution map populated by the policyResolver adapter; validateFieldWrite/validateRelationOp/validateRelationMetaWrite stamp it onto AffordanceDenialError.Attribution from the freshly-computed write-path verdict; denyAffordance appends it to the audit Summary. No cross-request side table, so no cross-principal bleed (C1), no GET-dependency/TOCTOU (C2 — PATCH-without-GET now carries attribution, pinned by TestPolicyResolver_AuditCarriesAttribution), no unbounded growth (C3).'
status: addressed
---
