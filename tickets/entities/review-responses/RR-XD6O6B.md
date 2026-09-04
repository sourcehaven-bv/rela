---
id: RR-XD6O6B
type: review-response
title: '[security] Symmetric content-scoped relation bypasses the faced-PATCH refusal via its inverse key'
finding: '[security] contentScopedRelationOn looked the body key up in m.Relations only, while applyRelationsModern resolves inverse names through resolveDirection. For a symmetric content-scoped relation with an inverse spelling, the addressed entity is still the tail, so PATCH ID@published with the inverse key attached a content-scoped edge to the BARE face — the misfiling the 422 exists to prevent.'
severity: significant
resolution: The guard now resolves keys through resolveDirection exactly as the writer does and refuses when the resolved edge's tail is the addressed entity and the type is content-scoped. Pinned by the symmetric `relates-to`/`related-from` case in TestFacedAddress_PatchWritesTheNamedFace (fixture parsed from YAML so the inverse index exists).
status: addressed
---
