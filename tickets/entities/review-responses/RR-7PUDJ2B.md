---
id: RR-7PUDJ2B
type: review-response
title: next_attempt still served without a note
finding: flowview.Pending.NextAttempt and FlowPending.next_attempt are still serialized while the UI no longer shows them; a later change might show them again.
severity: nit
resolution: Both fields carry a comment that no worker acts on the time yet (TKT-FWQZGAM), so a client does not show it.
status: addressed
---
