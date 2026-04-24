---
id: RR-NQIOW
type: review-response
title: MCP tool description could mention short/sequential explicitly
finding: The id parameter description 'only valid when the type's id_type is manual; auto-generated otherwise' is accurate but doesn't enumerate short and sequential by name. Reword to call them out.
severity: nit
reason: The error message already lists the concrete id_type value when a caller gets it wrong, so the feedback loop is short. Adding 'short and sequential' to the tool description risks drift when new id_types are added and doesn't help the common case (caller omits id, generation works). Lean tool descriptions preferred.
status: wont-fix
---
