---
id: RR-CNCGY15
type: review-response
title: 'Archplan: TriggerACLInput mirrors configedit.TriggerEdit'
finding: 'CI Archplan reported a types.synonyms finding: appbuild.TriggerACLInput and configedit.TriggerEdit share most fields.'
severity: minor
resolution: CheckTriggerACL now takes configedit.TriggerEdit and the editor and returns configedit.TriggerProblem; TriggerACLInput and TriggerACLProblem are removed.
status: addressed
---
