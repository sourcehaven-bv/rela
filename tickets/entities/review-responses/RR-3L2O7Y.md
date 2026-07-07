---
id: RR-3L2O7Y
type: review-response
title: Entity-version timing is fully decoupled from relation churn (relation writes don't bump entity updated_at)
finding: 'Relation writes (relation.go CreateRelation:150 / UpdateRelation:187 / DeleteRelation:217) only touch the relations table, never the endpoint entities'' updated_at. So the sweep is never woken by relation changes — correct for entities-only v1 (D1), but it means a version snapshot''s rendered relations reflect the LIVE relation set at read time, not as-of the version, and the two are entirely uncorrelated. This is implied by the TKT-VFJKMB deferral but not called out against the sweep trigger. Fix (docs only for v1): document that relation-set-as-of is not captured and entity render of relations is live-only in v1; relation history is TKT-VFJKMB. Also (minor): automation-only writes DO change content so they version, attributed to {tool: version-sweep} with who/why recoverable only via fuzzy audit-log correlation — document this attribution limitation.'
severity: minor
resolution: 'Documented (v1 limitation): relation writes don''t bump entity updated_at, so the sweep is decoupled from relation churn and a version''s rendered relations are live-at-read-time, not as-of — relation history is TKT-VFJKMB. Also documented that automation-only writes produce sweep-attributed ({tool: version-sweep}) versions with who/why recoverable only via audit-log correlation. Both in docs/postgres-backend.md + postgres CLAUDE.md.'
status: addressed
---
