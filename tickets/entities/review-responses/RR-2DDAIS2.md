---
id: RR-2DDAIS2
type: review-response
title: Case-only prefix overlap makes InferEntityType random
finding: With the fold, two types declaring nt- and NT- both match NT-001, and InferEntityType walks a Go map, so it returns either type. The loader does not reject such prefixes.
severity: significant
resolution: 'InferEntityType ranks matches: a byte-exact match beats a folded one, then the longer prefix, then the smaller type name. Folding never changes an answer an exact match gave. Pinned by TestMetamodel_InferEntityType_CaseFold. Rejecting shared prefixes at load is TKT-14J8NJO.'
status: addressed
---
