---
id: RR-CNCGY15
type: review-response
title: 'Archplan: TriggerACLInput mirrors configedit.TriggerEdit'
finding: 'CI Archplan reported a types.synonyms finding: appbuild.TriggerACLInput and configedit.TriggerEdit share most fields.'
severity: minor
resolution: 'CheckTriggerACL now takes configedit.TriggerEdit and the editor and returns configedit.TriggerProblem; TriggerACLInput and TriggerACLProblem are removed. The concepts ratchet also rose from two accepted findings this PR grew: Snapshot fields are now the named type configedit.Tree, and problem paths use named flows.yaml keys.'
status: addressed
---
