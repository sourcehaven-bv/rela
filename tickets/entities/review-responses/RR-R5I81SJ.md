---
id: RR-R5I81SJ
type: review-response
title: Legacy entity_refs() silently cuts the map at 2000
finding: Without ids or content the legacy form stopped after maxReadLimit entities so resolve_refs left references unlinked in a large project. The ids path raises for the same reason.
severity: significant
resolution: Past the default limit the legacy form raises (pass ids or content or set limit). A script that sets limit gets the cut it asked for. TestEntityRefs_LegacyRaisesPastDefaultLimit pins both.
status: addressed
---
