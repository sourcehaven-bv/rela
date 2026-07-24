---
id: RR-Y7P4MQ
type: review-response
title: Reader has no relation surface — PR 3's get_relations gating contract undecided
finding: 'The Reader interface covers entities only (Get/Filter), but PR 3 must gate rela.get_relations, which returns relation rows incl. relation properties and both endpoint ids. Leaving the contract undecided until PR 3 risks an ad-hoc shape. The codebase already has the precedent to adopt: relation history reads are gated on BOTH endpoints (FROM ∧ TO — ''the FROM entity only owns the UI placement, it is not the auth boundary; a TO-side oracle otherwise'', CLAUDE.md relation-versioning). Decide now: add FilterRelations(ctx, rels) with the both-endpoints-visible rule to PR 1 (interface + impl + conformance cases) so PR 3 stays mechanical. Relations have no field-level redaction today, so row-gating is the whole contract.'
severity: significant
resolution: 'Decided now: FilterRelations(ctx, rels) added to the Reader interface in PR 1 with the both-endpoints-visible rule (FROM ∧ TO, relation-history precedent), batched endpoint gating, conformance cases incl. a probe-count assertion (PLAN-RR12W4 AC6). PR 3''s get_relations becomes mechanical.'
status: addressed
---
