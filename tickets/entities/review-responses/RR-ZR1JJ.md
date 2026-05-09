---
id: RR-ZR1JJ
type: review-response
title: Half-encrypted relation cases are unspecified
finding: 'Three under-specified scenarios: (1) Relation file is cleartext but its target entity is encrypted — handlers_api.go currently silently drops the relation (if err != nil { continue }); user sees phantom missing relations. (2) Relation file is encrypted but endpoints are cleartext — relation appears in index from filename parsing but loadRelation returns ErrEncrypted. (3) Cardinality validation: does it count or skip relations with inaccessible endpoints? Plan must answer: do inaccessible relations show as placeholder relations in entity detail? Do entities with encrypted endpoints show relations pointing at Inaccessible{} placeholder targets? Add explicit ACs and integration tests for each case.'
severity: significant
resolution: 'With the field-on-entity model, half-encrypted scenarios resolve naturally: (1) Cleartext relation pointing at encrypted entity — the relation loads normally, the target entity loads with Inaccessible populated, both visible in UI; entity-detail view shows the relation pointing at a locked-but-named entity. (2) Encrypted relation between cleartext entities — relation file derives from filename, loads with Inaccessible properties, From/Type/To populated. (3) Cardinality validation: encrypted endpoint counts as a real node (relation exists, ID known); rules requiring property values on the encrypted endpoint are skipped per AC9. Plan now covers all three.'
status: addressed
---
