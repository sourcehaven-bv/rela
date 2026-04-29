---
id: RR-THTVI
type: review-response
title: Multiple ScriptErrors are discarded
finding: 'v1 deliberately shows only the first error, but the data for N-1 more is right there in the rejection list. Trivial leverage: append ''(and N-1 more)'' to the toast or expose a ''Next error'' button in the dialog. Out of scope for v1 per ticket plan, but track as a follow-up.'
severity: nit
resolution: Explicit v1 design decision per planning checklist (approved by user before implementation). Multi-error stacking would require UI work in ScriptErrorPanel which is out of scope. Worth revisiting if N>1 list-action failures become a common operator complaint.
reason: Explicit v1 design decision per planning checklist (approved by user before implementation). Multi-error stacking would require UI work in ScriptErrorPanel which is out of scope. Worth revisiting if N>1 list-action failures become a common operator complaint.
status: deferred
---
