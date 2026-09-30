---
id: RR-3BHELG
type: review-response
title: History restore refuses a status past the entry state
finding: RecreateEntity enforces the state-machine entry rule, so a deleted entity whose status is past the entry value cannot be restored. The rule was set for sync; restore is now the only caller.
severity: significant
resolution: 'Fixed in BUG-KK1UXH: RecreateEntity no longer applies the entry rule. It calls statemachine.Set.EnforceRestore, which still requires a declared edge into the value whose guard the principal holds.'
reason: Behaviour is unchanged by this PR (the old create branch enforced it too). Whether restore is exempt is a product decision tracked in BUG-KK1UXH; the code comment names it.
status: addressed
---
