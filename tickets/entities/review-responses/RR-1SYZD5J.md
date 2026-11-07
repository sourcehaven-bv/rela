---
id: RR-1SYZD5J
type: review-response
title: Client ceilings and the reserved wildcard
finding: readQuery applies the type-level ceiling first, so excluding reserved names from the wildcard denies runs to every attenuated client.
severity: significant
resolution: 'Plan changed: for _flow-run the type-level ceiling check is skipped; each branch''s subject read query applies the ceiling.'
status: addressed
---
