---
id: RR-7NYMJK
type: review-response
title: 'Minor: HashRelation must fold in the triple; add relations(updated_at) index; attribution best-effort across renames; tick cost'
finding: Batched minor findings. (M1) content-hash dedup must include (from_id,rel_type,to_id) in the hashed projection (a HashRelation, not reusing HashEntity) or two different relations with identical props/content dedup against each other within a shared schema_versions. (M2) relations has NO updated_at index today (only 0001 PK + from/to/type indexes) — the migration must add relations(updated_at) for the sweep filter, same non-concurrent-CREATE lock caveat as entities_updated_at_idx (0004:83). (M3) swept relation create/update rows use the version-sweep principal with 'editing principal recoverable from audit log', but relation audit records key on (from,type,to) which mutate on rename — so recovery by old triple fails after an endpoint rename; document attribution recovery as best-effort across renames. (M4-cost) adding a second FROM relations candidate scan doubles per-tick work on the single advisory-locked conn; verify tick duration and keep entities-before-relations ordering deterministic (architect M1-M4, cranky M1-M3).
severity: minor
resolution: 'Design revised: HashRelation folds in the triple; migration adds relations(updated_at) index; attribution recovery documented best-effort across renames; entities-before-relations tick ordering deterministic and tick-duration to be verified in impl. All folded into the revised storage + capture sections.'
status: addressed
---
