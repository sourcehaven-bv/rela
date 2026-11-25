---
id: RR-OIZNBLY
type: review-response
title: Replaying header patches after commit reorders or loses them
finding: Deferring move/fork header patches to after commit can reorder, is lost on a crash and has no commit list when the update joins a foreign transaction.
severity: significant
resolution: Header patches stay inside the write transaction, written as system:flow bound before the transaction (as create starts do).
status: addressed
---
