---
id: RR-RY4Y0A
type: review-response
title: target_type reachability compared a canonicalized name against a raw From/To list
finding: 'Code review. validateConstraintTargetType resolved the operator''s target_type through ResolveAlias but then did slices.Contains against relDef.To/.From verbatim. If from:/to: ever names an alias, EVERY spelling of target_type fails — including the alias the schema itself used — and the error message names the alias it just rejected ("target_type: taak is not reachable — connects to [task terugkerend]"). Unreachable today only because validateRelationReferences separately rejects an aliased to: via a direct m.Entities lookup, so this is a latent trap keyed to an unrelated invariant. The runtime deliberately resolves both sides (sameEntityType), so load and check disagreed by luck rather than construction.'
severity: significant
resolution: 'Reachability now uses slices.ContainsFunc with ResolveAlias applied to both sides, so load-time and check-time agree by construction. Covered by TestValidateValidationRelations_AliasedReachableType, which declares the relation reaching the type by its alias. Mutation-tested: reverting to slices.Contains fails the test with exactly the misleading message the reviewer predicted.'
status: addressed
---
