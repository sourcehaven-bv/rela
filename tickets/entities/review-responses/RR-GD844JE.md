---
id: RR-GD844JE
type: review-response
title: Org deletions acknowledged and lost before erasure ships
finding: logOrgEvents acknowledges organization.deleted at Info level; the IdP will not redeliver it so a deletion before TKT-TNERAS leaves only a filterable log line.
severity: significant
resolution: organization.deleted is now logged at Warn with a message telling the operator to erase the tenant by hand; the docs say acknowledged org events are not replayed.
status: addressed
---
