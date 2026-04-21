---
id: RR-3GURO
type: review-response
title: 'Prefix validation: plan says ''allowlist'' but handler never sees entityDef'
finding: 'The plan mandates validating `req.prefix` against `entityDef.GetIDPrefixes()` as an allowlist. However, `handleV1CreateEntity` in api_v1.go:370 is called via the single-type route dispatch and does not currently load the EntityDef. The plan must specify WHERE validation happens: either (a) in the handler before calling entityManager.CreateEntity (explicit 422 with clear message), or (b) add validation inside `workspace.GenerateID` which errors on unknown prefix for known types. Option (a) is cleaner because the error message can be specific (`''{prefix}'' is not a valid prefix for type {type}; allowed: [...]`). If we only rely on `GenerateID`, it will happily generate `UNKNOWN-abc` IDs because it just prepends the string. Specify which path is taken.'
severity: critical
resolution: 'Plan now specifies option (a): validation in `handleV1CreateEntity` BEFORE calling entityManager.CreateEntity. Validation uses `entityDef.GetIDPrefixes()` as allowlist; 422 with explicit message listing allowed prefixes.'
status: addressed
---
