---
id: RR-C3OY9NC
type: review-response
title: Renumber after an explicit order write skips the gate a move applies
finding: checkRelationMoveMeta gates other sources' edges on a move but runRenumberAfterUpdate rewrites the same edges after an explicit _order_in write with no gate.
severity: significant
resolution: runRenumberAfterUpdate on a field-gated handle now renumbers only when every other source passes the gate for the order key (renumberAllowed) and skips the renumber otherwise; TestRenumberAfterUpdate_FieldGateJudgesSiblingSources pins it. The data-entry handle is not field-gated; that remainder and the missing per-source update ACL on renumber are BUG-T0MFVJG.
status: addressed
---
