---
id: RR-JKH5HB
type: review-response
title: Edges written backwards before this fix are not repaired
finding: Section creates on outgoing relations wrote new --rel--> peer. Where the relation's from/to types accept both ends the backwards edge was stored.
severity: significant
resolution: Data repair is deployment-specific and needs the operator's schema. Reported to the user as an open decision; a one-off audit per deployment is the follow-up.
reason: Data repair is deployment-specific and needs the operator's schema. Reported to the user as an open decision; a one-off audit per deployment is the follow-up.
status: deferred
---
