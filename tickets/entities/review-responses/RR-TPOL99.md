---
id: RR-TPOL99
type: review-response
title: Delete-then-restore bypassed transition guards
finding: With the entry rule simply removed, a principal holding create and history:read could delete an entity and restore an old version at a guarded state (e.g. established) it could never enter by transition. Raised by both cranky-code-reviewer and rela-security-reviewer.
severity: significant
resolution: 'Added statemachine.Set.EnforceRestore: a path of declared edges from the entry value must reach the restored value with every guard held. RecreateEntity calls it; a denial maps to a 403 transition-guard ForbiddenError with a denied-write record. Pinned by TestEnforceRestore and TestTransition_RecreateEntity_GuardedStateIs403.'
status: addressed
---
