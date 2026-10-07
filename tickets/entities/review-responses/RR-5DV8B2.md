---
id: RR-5DV8B2
type: review-response
title: Soft fallback found by substring match wrote all creates directly
finding: The disallowed-type fallback was detected by a substring match on the error. It then wrote every create directly, without template, order or audit.
severity: minor
resolution: Fixed in 3b2b0bdd8 and 2869cf91e. Typed errors (InvalidRelationError, RelationCreateError) are matched with errors.As. Only the refused creates are written directly. Covered by TestPatchRelations_DisallowedTypeFallbackKeepsAllowedCreateOnManager and TestPatchRelations_DisallowedTypeWithDeniedRemoveWritesNothing.
status: addressed
---

Review finding R2-6.
