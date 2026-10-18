---
id: RR-8Y12CY
type: review-response
title: Carry the self-asserted marker on ctx like audit.WithTriggeredBy, not as a Principal field
finding: 'Principal is the trust-boundary type: its package doc gates growth, and every rebuild site must copy each field by hand (previous finding). A ctx value set by localPrincipal (audit.WithAssertedBy) and read by the audit writer and realtime.authorOf leaves Equal, ShareKey, JSON and the principal package unchanged, and cannot be dropped by a rebuild. Reconsider decision 4''s mechanism; the marker itself is right.'
severity: significant
resolution: 'Adopted: audit.WithAssertedBy on ctx, set by localPrincipal and read by the graph audit writer. Principal is unchanged.'
status: addressed
---
