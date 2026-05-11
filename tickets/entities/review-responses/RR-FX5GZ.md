---
id: RR-FX5GZ
type: review-response
title: Inconsistent Lua surface — update_entity gains warnings, create_relation does not
finding: 'Plan softens entity writes but out-of-scopes relation writes. After: rela.update_entity returns (entity, warnings), rela.create_relation returns (relation) only. Modern PATCH already produces warnings on HTTP side; Lua silently discards. Script writing both sees inconsistent surfaces. Worse: Lua automation creating edge with missing target succeeds without script knowing warning happened. Recommendation: (a) extend scope: add (relation, warnings) to rela.create_relation/update_relation; OR (b) document explicit known surface inconsistency on Lua side and file follow-up before merge. Plan currently does neither. From design-review F9.'
severity: minor
resolution: 'Scope extended: entitymanager.CreateRelation result type also gains Warnings []Warning. rela.create_relation gets multi-return treatment too (AC23, AC24). Lua surface stays consistent across entity and relation writes. Plan notes verifying update_relation''s existence during implementation; AC24 may be N/A if only create_relation exists today.'
status: addressed
---
