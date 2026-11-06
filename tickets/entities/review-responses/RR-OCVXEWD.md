---
id: RR-OCVXEWD
type: review-response
title: MCP relation writes skip relation meta grants
finding: Relation meta grants were enforced only in dataentry. The field-gated entity manager that MCP on rela-server writes through did not check relation meta on CreateRelation or UpdateRelation.
severity: significant
resolution: FieldWriteGate gains CheckRelationMetaWrite. affordances.WriteGate answers it from RelationVerdicts. The manager asks it on create and on update of a field-gated handle against the source row and records a refusal as a denied write. Pinned by TestRelationMeta_FieldGateJudgesCallerMeta and TestWriteGate_RelationMetaFollowsTheAllowlist.
status: addressed
---
