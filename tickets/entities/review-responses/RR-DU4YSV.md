---
id: RR-DU4YSV
type: review-response
title: Rename verb in access reports would give wrong answers
finding: grantForRole and grantsVerb read the update list for rename, but the runtime asks for a rename grant on faced types (decideFamily) and also honours one on faceless types (GrantsVerbOnState). Adding rename to rela acl can, who-can and map would report wrong answers in both directions.
severity: critical
resolution: acl.Verb is derived from acl.Op, but the reports use aclmap.Verbs, which excludes rename by name and rejects it in Can, WhoCan and Map. CLI enums unchanged and pinned to aclmap.Verbs by TestACLVerbEnumsMatchReportVerbs. TKT-4APGVF fixes the helpers and lifts the exclusion.
status: addressed
---
