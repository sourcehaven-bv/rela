---
id: RR-BZNL0S
type: review-response
title: '"Same serializer redaction path" for relations is false — relations have no field-redaction today'
finding: 'The design claims relation-version snapshots run through ''the same serializer redaction as a live read.'' That path does not exist for relations. Entity history reuses serializer.forWire → stripHiddenProperties/Inaccessible (history_handler.go ~L208). Relation.Inaccessible (entity.go:215) is NEVER populated (zero writers), and the live relation GET emits properties RAW: `rel["meta"] = edge.Properties` with no redaction (api_v1.go ~L1266). RelationVerdicts gates relation TYPE create/read, not per-field visible:. So relation properties have no field redaction anywhere in the live system, and relation history would inherit and amplify that. Fix: EITHER (a) scope this ticket to state explicitly that relation properties have no field-level redaction today — relation history exposes exactly what a live relation GET exposes — and add a test pinning that equivalence (only defensible if C4 dual-endpoint gating holds); OR (b) build a relation field-redaction path first. Do NOT ship the false ''same serializer path'' claim (cranky C3).'
severity: critical
resolution: 'Design revised: dropped the false ''same serializer path'' claim. Ticket now states relations have NO field redaction today; relation history exposes exactly what a live relation GET exposes, pinned by a test, safe because dual-endpoint gating (RR-SDDYZO) covers relation visibility. Field-redaction path is an explicit out-of-scope follow-up.'
status: addressed
---
