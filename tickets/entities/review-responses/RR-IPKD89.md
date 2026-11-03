---
id: RR-IPKD89
type: review-response
title: Audit record and the entity-guard refusal could be clearer
finding: The session audit record has no generation, although the notice has one. AuditRecord has no field for it, so pairing a record with its notice needs the cause seq, which a file-tier crash can make wrong. A guard that names entity fails with the compiler's "unknown variable"; replace that with an actionable message ("transition guards cannot read the entity yet; use a trigger that posts a message") and match it in the AC 2 row.
severity: nit
resolution: 'Plan updated: AuditRecord gains Generation; a guard naming entity gets a refusal telling the operator to use a trigger (AC 2).'
status: addressed
---
