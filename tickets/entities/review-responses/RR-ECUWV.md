---
id: RR-ECUWV
type: review-response
title: --all purge must use the fenced lineage CTE, not WHERE entity_id=$1 (id-reuse destroys unrelated history)
finding: 'Both reviewers (cranky S5 = architect M5). ''Delete all its version rows'' is ambiguous. `WHERE entity_id = $1` is BOTH incomplete (misses pre-rename segments under old ids — PII survives) AND over-broad (a REUSED id''s unrelated new entity''s rows get deleted — wrong data destroyed). Fix: --all must purge the FENCED lineage — reuse lineageCTE (entity vseq [lo,hi) range) / relationLineageIDs (rel_record_id set) so it matches EXACTLY what ListVersions shows the operator, and refuse/warn when the lineage spans reused ids. This composes with RR-EQQP1 (rename-row handling): a --all that includes rename rows must either sever cleanly with a dry-run warning or be scoped to non-rename rows. Specify --all lineage semantics precisely in the design.'
severity: significant
resolution: 'Design revised: --all purges the FENCED lineage (lineageCTE / relationLineageIDs), matching ListVersions, refusing/warning on reused-id spans. See revised design #3.'
status: addressed
---
