---
id: RR-I3F8C1A
type: review-response
title: buildFlowEdits fails open when the run store is nil
finding: buildFlowEdits returned no ceiling when flows were declared but the run store was nil, which would silently drop every outside_edits rule.
severity: minor
resolution: The run store is built before buildFlowEdits; buildFlowEdits returns nil only when no flows are declared, and flowedits.New rejects a nil store.
status: addressed
---
