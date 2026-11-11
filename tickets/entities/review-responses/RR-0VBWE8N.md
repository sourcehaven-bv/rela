---
id: RR-0VBWE8N
type: review-response
title: Superseded conflict entries never expire
finding: Superseded entries stay until dismissed and holdsField scans all entries.
severity: minor
resolution: Superseded entries now expire after superseded_retention (per remote, default 30d; 0 stores no losing local value). rela history-purge drops an entity's superseded entries on every remote.
reason: Covered by RR-XGCZSM5 (conflict list size); rela remote resolve --dismiss removes them and they go with the row or the remote.
status: addressed
---
